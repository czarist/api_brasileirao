package fmf

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const (
	userAgent = "api-brasileirao/0.1 (+https://github.com/czarist/api_brasileirao)"
	origin    = "https://www.fmf.com.br"
)

var digits = regexp.MustCompile(`^[0-9]+$`)

// Client fetches the fixed FMF competition pages.
type Client struct {
	baseURL *url.URL
	http    *http.Client
	limiter *rate.Limiter
	maxBody int64
	retries int
}

// ClientOptions configures timeouts and the request interval.
// BaseURL is only for tests. An empty value uses https://www.fmf.com.br.
type ClientOptions struct {
	BaseURL     string
	Timeout     time.Duration
	MinInterval time.Duration
	MaxBody     int64
	Retries     int
}

// NewClient builds a client that refuses any host other than www.fmf.com.br,
// except a loopback host used by tests.
func NewClient(opts ClientOptions) (*Client, error) {
	raw := opts.BaseURL
	if raw == "" {
		raw = origin
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
		return nil, fmt.Errorf("fmf base url: %w", err)
	}
	if err := allowHost(parsed); err != nil {
		return nil, err
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.MinInterval <= 0 {
		opts.MinInterval = time.Second
	}
	if opts.MaxBody <= 0 {
		opts.MaxBody = 8 << 20
	}
	host := parsed.Host
	transport := http.DefaultTransport.(*http.Transport).Clone()
	httpClient := &http.Client{
		Timeout:   opts.Timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if !strings.EqualFold(req.URL.Host, host) {
				return errors.New("redirect left the source host")
			}
			return nil
		},
	}
	return &Client{
		baseURL: parsed,
		http:    httpClient,
		limiter: rate.NewLimiter(rate.Every(opts.MinInterval), 1),
		maxBody: opts.MaxBody,
		retries: opts.Retries,
	}, nil
}

func allowHost(u *url.URL) error {
	host := u.Hostname()
	if strings.EqualFold(host, "www.fmf.com.br") {
		if u.Scheme != "https" {
			return errors.New("fmf origin must be https")
		}
		return nil
	}
	if host == "127.0.0.1" || host == "localhost" || host == "::1" {
		return nil
	}
	return errors.New("refusing a source host that is not www.fmf.com.br")
}

// GetDivision loads the current page for a fixed division id (1, 2, or 3).
func (c *Client) GetDivision(ctx context.Context, division string) ([]byte, error) {
	path, err := divisionPath(division)
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodGet, path, "")
}

// PostSeason loads one catalog season. seasonID is the option value from the
// division page, not a client-supplied URL.
func (c *Client) PostSeason(ctx context.Context, division, seasonID string) ([]byte, error) {
	path, err := divisionPath(division)
	if err != nil {
		return nil, err
	}
	if !digits.MatchString(seasonID) {
		return nil, errors.New("refusing a season id that is not digits")
	}
	return c.do(ctx, http.MethodPost, path, seasonID)
}

func divisionPath(division string) (string, error) {
	switch division {
	case "1", "2", "3":
		return "/Competicoes/ProxJogos.aspx?d=" + division, nil
	default:
		return "", fmt.Errorf("unknown FMF division %s", division)
	}
}

func (c *Client) do(ctx context.Context, method, path, seasonID string) ([]byte, error) {
	if err := validatePath(path); err != nil {
		return nil, err
	}
	endpoint, err := c.baseURL.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("build source url: %w", err)
	}
	if !strings.EqualFold(endpoint.Host, c.baseURL.Host) {
		return nil, errors.New("refusing to request a different host")
	}
	if endpoint.Path != "/Competicoes/ProxJogos.aspx" {
		return nil, errors.New("refusing a source path outside the competition catalog")
	}

	var last error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			wait := backoff(attempt, retryAfter(last))
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, err
		}
		body, status, header, err := c.once(ctx, method, endpoint, seasonID)
		if err == nil {
			return body, nil
		}
		last = err
		if !retryable(status, err) {
			return nil, err
		}
		if header != "" {
			last = &retryErr{err: err, after: header}
		}
	}
	return nil, last
}

func (c *Client) once(ctx context.Context, method string, endpoint *url.URL, seasonID string) ([]byte, int, string, error) {
	var bodyReader io.Reader
	if method == http.MethodPost {
		form := url.Values{}
		form.Set("request", seasonID)
		bodyReader = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), bodyReader)
	if err != nil {
		return nil, 0, "", err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, "", fmt.Errorf("source request: %w", err)
	}
	defer resp.Body.Close()
	retry := resp.Header.Get("Retry-After")
	body, err := readLimited(resp.Body, c.maxBody)
	if err != nil {
		return nil, resp.StatusCode, retry, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, retry, fmt.Errorf("source returned HTTP %d", resp.StatusCode)
	}
	got := strings.ToLower(resp.Header.Get("Content-Type"))
	if !strings.Contains(got, "text/html") && !strings.Contains(got, "application/xhtml") {
		return nil, resp.StatusCode, retry, fmt.Errorf("unexpected content type %q", got)
	}
	if len(body) == 0 {
		return nil, resp.StatusCode, retry, errors.New("source returned an empty body")
	}
	lower := strings.ToLower(string(body[:min(len(body), 512)]))
	if strings.Contains(lower, "cf-browser-verification") || strings.Contains(lower, "captcha") {
		return nil, resp.StatusCode, retry, errors.New("source returned a block page")
	}
	return body, resp.StatusCode, retry, nil
}

func readLimited(r io.Reader, max int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, fmt.Errorf("read source body: %w", err)
	}
	if int64(len(body)) > max {
		return nil, fmt.Errorf("source body exceeds %d bytes", max)
	}
	return body, nil
}

func validatePath(path string) error {
	if !strings.HasPrefix(path, "/") || strings.Contains(path, "://") || strings.Contains(path, "..") {
		return errors.New("refusing a source path that is not a relative URL")
	}
	return nil
}

type retryErr struct {
	err   error
	after string
}

func (e *retryErr) Error() string { return e.err.Error() }
func (e *retryErr) Unwrap() error { return e.err }

func retryAfter(err error) time.Duration {
	var marked *retryErr
	if !errors.As(err, &marked) || marked.after == "" {
		return 0
	}
	if seconds, convErr := strconv.Atoi(strings.TrimSpace(marked.after)); convErr == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, convErr := http.ParseTime(marked.after); convErr == nil {
		wait := time.Until(when)
		if wait < 0 {
			return 0
		}
		return wait
	}
	return 0
}

func backoff(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 && retryAfter <= 30*time.Second {
		return retryAfter
	}
	base := 400 * time.Millisecond
	for i := 1; i < attempt; i++ {
		base *= 2
		if base > 8*time.Second {
			base = 8 * time.Second
			break
		}
	}
	jitter := time.Duration(rand.IntN(250)) * time.Millisecond
	return base + jitter
}

func retryable(status int, err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if status == 0 {
		return true
	}
	switch status {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

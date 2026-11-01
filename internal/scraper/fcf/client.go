package fcf

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

const userAgent = "api-brasileirao/0.1 (+https://github.com/czarist/api_brasileirao)"

var digits = regexp.MustCompile(`^[0-9]+$`)

// Client fetches the fixed FCF catalog paths. It refuses any other host,
// except a loopback host used by tests.
type Client struct {
	baseURL *url.URL
	http    *http.Client
	limiter *rate.Limiter
	maxBody int64
	retries int
}

// ClientOptions configures timeouts and the request interval.
// BaseURL is only for tests. An empty value uses the federation host.
type ClientOptions struct {
	BaseURL     string
	Timeout     time.Duration
	MinInterval time.Duration
	MaxBody     int64
	Retries     int
}

// NewClient builds a client locked to fcf.com.br.
func NewClient(opts ClientOptions) (*Client, error) {
	raw := opts.BaseURL
	if raw == "" {
		raw = "https://" + host
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
		return nil, fmt.Errorf("fcf base url: %w", err)
	}
	if err := allowHost(parsed); err != nil {
		return nil, err
	}
	parsed.Path = ""
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
	locked := parsed.Host
	transport := http.DefaultTransport.(*http.Transport).Clone()
	httpClient := &http.Client{
		Timeout:   opts.Timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if !strings.EqualFold(req.URL.Host, locked) {
				return errors.New("redirect left the source host")
			}
			if err := allowPath(req.URL); err != nil {
				return err
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
	name := u.Hostname()
	if strings.EqualFold(name, host) {
		if u.Scheme != "https" {
			return errors.New("fcf origin must be https")
		}
		return nil
	}
	if name == "127.0.0.1" || name == "localhost" || name == "::1" {
		return nil
	}
	return fmt.Errorf("refusing a source host that is not %s", host)
}

// GetCatalog loads the professional index. Year links are in the HTML.
func (c *Client) GetCatalog(ctx context.Context) ([]byte, error) {
	return c.do(ctx, "/competicoes/profissionais/")
}

// GetYear loads one professional year page. The division select is in the HTML.
func (c *Client) GetYear(ctx context.Context, year string) ([]byte, error) {
	if !fourDigit(year) {
		return nil, errors.New("refusing a season year that is not four digits")
	}
	return c.do(ctx, "/competicoes/profissionais/"+year+"/")
}

// GetSeason loads one adult division page. Matches and the tables are in the HTML.
func (c *Client) GetSeason(ctx context.Context, year, id string) ([]byte, error) {
	if !fourDigit(year) || !digits.MatchString(id) {
		return nil, errors.New("refusing a season path that is not a four-digit year and a numeric id")
	}
	return c.do(ctx, "/competicoes/profissionais/"+year+"/"+id+"/")
}

func fourDigit(year string) bool {
	return len(year) == 4 && digits.MatchString(year)
}

func (c *Client) do(ctx context.Context, path string) ([]byte, error) {
	endpoint, err := c.baseURL.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("build source url: %w", err)
	}
	if !strings.EqualFold(endpoint.Host, c.baseURL.Host) {
		return nil, errors.New("refusing to request a different host")
	}
	if err := allowPath(endpoint); err != nil {
		return nil, err
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
		body, status, header, err := c.once(ctx, endpoint)
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

func allowPath(u *url.URL) error {
	if strings.Contains(u.Path, "..") || strings.Contains(u.EscapedPath(), "..") {
		return errors.New("refusing a source path that is not a relative URL")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return errors.New("refusing a competition query")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "competicoes" || parts[1] != "profissionais" {
		return errors.New("refusing a source path outside the professional catalog")
	}
	switch len(parts) {
	case 2:
		return nil
	case 3:
		if !fourDigit(parts[2]) {
			return errors.New("refusing a season year that is not four digits")
		}
		return nil
	case 4:
		if !fourDigit(parts[2]) || !digits.MatchString(parts[3]) {
			return errors.New("refusing a season path")
		}
		return nil
	default:
		return errors.New("refusing a source path outside the professional catalog")
	}
}

func (c *Client) once(ctx context.Context, endpoint *url.URL) ([]byte, int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, 0, "", err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html")
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
	if len(body) == 0 {
		return nil, resp.StatusCode, retry, errors.New("source returned an empty body")
	}
	lower := strings.ToLower(string(body[:min(len(body), 512)]))
	if strings.Contains(lower, "cf-browser-verification") || strings.Contains(lower, "captcha") {
		return nil, resp.StatusCode, retry, errors.New("source returned a block page")
	}
	got := strings.ToLower(resp.Header.Get("Content-Type"))
	if got != "" && !strings.Contains(got, "text/html") && !strings.Contains(got, "application/xhtml") && !strings.Contains(got, "text/plain") {
		return nil, resp.StatusCode, retry, fmt.Errorf("unexpected content type %q", got)
	}
	if !strings.Contains(string(body), "<") {
		return nil, resp.StatusCode, retry, errors.New("source body is not HTML")
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

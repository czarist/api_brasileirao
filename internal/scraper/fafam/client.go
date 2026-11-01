package fafam

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

var (
	digits = regexp.MustCompile(`^[0-9]+$`)
	serieA = regexp.MustCompile(`^campeonato-amazonense-serie-a-(\d{4})$`)
	serieB = regexp.MustCompile(`^(?:amazonense-serie-b|campeonato-amazonense-serie-b)-(\d{4})$`)
	plain  = regexp.MustCompile(`^(?:campeonato-amazonense|amazonense)-(\d{4})$`)
)

// Client fetches the fixed Amazonas catalog paths. It refuses any other host,
// except a loopback host used by tests. It does not request súmula or borderô files.
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

// NewClient builds a client locked to fafamazonas.com.br.
func NewClient(opts ClientOptions) (*Client, error) {
	raw := opts.BaseURL
	if raw == "" {
		raw = "https://" + host
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
		return nil, fmt.Errorf("fafam base url: %w", err)
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
			return errors.New("fafam origin must be https")
		}
		return nil
	}
	if name == "127.0.0.1" || name == "localhost" || name == "::1" {
		return nil
	}
	return fmt.Errorf("refusing a source host that is not %s", host)
}

// GetCatalog loads one championship index page. Page 1 is the first screen.
func (c *Client) GetCatalog(ctx context.Context, page int) ([]byte, error) {
	if page < 1 || page > 20 {
		return nil, errors.New("refusing a catalog page")
	}
	if page == 1 {
		return c.do(ctx, "/site/campeonatos/")
	}
	return c.do(ctx, "/site/campeonatos/"+strconv.Itoa(page)+"/")
}

// GetRound loads one round of an adult season. The round id is the last path segment.
func (c *Client) GetRound(ctx context.Context, slug, round string) ([]byte, error) {
	if _, _, ok := classify(slug); !ok || !digits.MatchString(round) {
		return nil, errors.New("refusing a season path that is not an adult Amazonas division")
	}
	return c.do(ctx, "/site/campeonato/"+slug+"/"+round+"/")
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
	if len(parts) < 2 || parts[0] != "site" {
		return errors.New("refusing a source path outside the championship catalog")
	}
	switch parts[1] {
	case "campeonatos":
		if len(parts) == 2 {
			return nil
		}
		if len(parts) == 3 && digits.MatchString(parts[2]) {
			return nil
		}
	case "campeonato":
		if len(parts) == 4 {
			if _, _, ok := classify(parts[2]); ok && digits.MatchString(parts[3]) {
				return nil
			}
		}
	}
	return errors.New("refusing a source path outside the championship catalog")
}

// classify maps a catalog slug to an adult division. Women's, youth, and cup
// slugs are refused. prefer is 0 for the slug to keep when a year is listed twice.
func classify(slug string) (division, int, bool) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if strings.Contains(slug, "femin") || strings.Contains(slug, "sub") || strings.Contains(slug, "copa") {
		return division{}, 0, false
	}
	if found := serieA.FindStringSubmatch(slug); found != nil {
		year, err := strconv.Atoi(found[1])
		if err != nil || year < 1990 || year > 2100 {
			return division{}, 0, false
		}
		return divisions[0], year, true
	}
	if found := serieB.FindStringSubmatch(slug); found != nil {
		year, err := strconv.Atoi(found[1])
		if err != nil || year < 1990 || year > 2100 {
			return division{}, 0, false
		}
		return divisions[1], year, true
	}
	if found := plain.FindStringSubmatch(slug); found != nil {
		year, err := strconv.Atoi(found[1])
		if err != nil || year < 1990 || year > 2100 {
			return division{}, 0, false
		}
		return divisions[2], year, true
	}
	return division{}, 0, false
}

func preferSlug(slug string) int {
	if strings.HasPrefix(slug, "amazonense-serie-b-") || strings.HasPrefix(slug, "campeonato-amazonense-") && !strings.Contains(slug, "serie-b") {
		return 0
	}
	return 1
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

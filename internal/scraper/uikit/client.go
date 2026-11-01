package uikit

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

// Client fetches one federation's fixed catalog paths.
type Client struct {
	site    Site
	baseURL *url.URL
	http    *http.Client
	limiter *rate.Limiter
	maxBody int64
	retries int
}

// ClientOptions configures timeouts and the request interval.
// BaseURL is only for tests. An empty value uses the site host.
type ClientOptions struct {
	BaseURL     string
	Timeout     time.Duration
	MinInterval time.Duration
	MaxBody     int64
	Retries     int
}

// NewClient builds a client that refuses any host other than the site host,
// except a loopback host used by tests.
func NewClient(site Site, opts ClientOptions) (*Client, error) {
	if site.Host == "" || site.Source == "" || site.IndexPath == "" {
		return nil, errors.New("uikit site is missing its host")
	}
	raw := opts.BaseURL
	if raw == "" {
		raw = "https://" + site.Host
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
		return nil, fmt.Errorf("uikit base url: %w", err)
	}
	if err := allowHost(site, parsed); err != nil {
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
			if err := allowPath(site, req.URL); err != nil {
				return err
			}
			return nil
		},
	}
	return &Client{
		site:    site,
		baseURL: parsed,
		http:    httpClient,
		limiter: rate.NewLimiter(rate.Every(opts.MinInterval), 1),
		maxBody: opts.MaxBody,
		retries: opts.Retries,
	}, nil
}

func allowHost(site Site, u *url.URL) error {
	host := u.Hostname()
	if strings.EqualFold(host, site.Host) {
		if u.Scheme != "https" {
			return errors.New("uikit origin must be https")
		}
		return nil
	}
	if host == "127.0.0.1" || host == "localhost" || host == "::1" {
		return nil
	}
	return fmt.Errorf("refusing a source host that is not %s", site.Host)
}

// GetIndex loads the current catalog page.
func (c *Client) GetIndex(ctx context.Context) ([]byte, error) {
	return c.do(ctx, http.MethodGet, c.site.IndexPath, nil)
}

// PostIndex loads one catalog year. year is an option value from the page.
func (c *Client) PostIndex(ctx context.Context, year string) ([]byte, error) {
	if !digits.MatchString(year) || len(year) != 4 {
		return nil, errors.New("refusing a catalog year that is not four digits")
	}
	return c.do(ctx, http.MethodPost, c.site.IndexPath, url.Values{"exercicio": {year}})
}

// GetSeason loads the phase list for one catalog id.
func (c *Client) GetSeason(ctx context.Context, id string) ([]byte, error) {
	if !digits.MatchString(id) {
		return nil, errors.New("refusing a competition id that is not digits")
	}
	return c.do(ctx, http.MethodGet, c.seasonPath(id), nil)
}

// PostMatches loads every round of one phase.
func (c *Client) PostMatches(ctx context.Context, query FeedQuery, phaseID string) ([]byte, error) {
	form, err := feedForm(query, phaseID)
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPost, c.feedPath("jogos_ajax.php"), form)
}

// PostStandings loads the classification HTML for one phase.
func (c *Client) PostStandings(ctx context.Context, query FeedQuery, phaseID string) ([]byte, error) {
	form, err := feedForm(query, phaseID)
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPost, c.feedPath("classificacao_ajax.php"), form)
}

func (c *Client) seasonPath(id string) string {
	if c.site.Host == "www.fmfmt.com.br" {
		return "/pt/competicoes/tabela.php?ID=" + id
	}
	return "/competicoes/tabela/" + id
}

func (c *Client) feedPath(script string) string {
	if c.site.Host == "www.fmfmt.com.br" {
		return "/pt/competicoes/" + script
	}
	return "/competicoes/" + script
}

func feedForm(query FeedQuery, phaseID string) (url.Values, error) {
	for _, value := range []string{query.CompetitionID, query.CategoryID, query.Championship, query.Year, phaseID} {
		if !digits.MatchString(value) {
			return nil, errors.New("refusing a feed id that is not digits")
		}
	}
	if len(query.Year) != 4 {
		return nil, errors.New("refusing a feed year that is not four digits")
	}
	return url.Values{
		"idcampeonato": {query.CompetitionID},
		"categoria":    {query.CategoryID},
		"campeonato":   {query.Championship},
		"exercicio":    {query.Year},
		"rodada":       {"1"},
		"fase":         {phaseID},
	}, nil
}

func (c *Client) do(ctx context.Context, method, path string, form url.Values) ([]byte, error) {
	endpoint, err := c.baseURL.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("build source url: %w", err)
	}
	if !strings.EqualFold(endpoint.Host, c.baseURL.Host) {
		return nil, errors.New("refusing to request a different host")
	}
	if err := allowPath(c.site, endpoint); err != nil {
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
		body, status, header, err := c.once(ctx, method, endpoint, form)
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

func allowPath(site Site, u *url.URL) error {
	if strings.Contains(u.Path, "..") || strings.Contains(u.EscapedPath(), "..") {
		return errors.New("refusing a source path that is not a relative URL")
	}
	index := strings.TrimRight(site.IndexPath, "/")
	switch strings.TrimRight(u.Path, "/") {
	case index:
		if u.RawQuery != "" {
			return errors.New("refusing a catalog query")
		}
		return nil
	}
	if site.Host == "www.fmfmt.com.br" {
		switch u.Path {
		case "/pt/competicoes/tabela.php":
			if !digits.MatchString(u.Query().Get("ID")) || len(u.Query()) != 1 {
				return errors.New("refusing a competition query that is not ID=digits")
			}
			return nil
		case "/pt/competicoes/jogos_ajax.php", "/pt/competicoes/classificacao_ajax.php":
			if u.RawQuery != "" {
				return errors.New("refusing a query on a phase feed")
			}
			return nil
		}
	}
	if site.Host != "www.fmfmt.com.br" {
		if strings.HasPrefix(u.Path, "/competicoes/tabela/") {
			id := strings.TrimPrefix(u.Path, "/competicoes/tabela/")
			if !digits.MatchString(id) || u.RawQuery != "" {
				return errors.New("refusing a competition path that is not digits")
			}
			return nil
		}
		switch u.Path {
		case "/competicoes/jogos_ajax.php", "/competicoes/classificacao_ajax.php":
			if u.RawQuery != "" {
				return errors.New("refusing a query on a phase feed")
			}
			return nil
		}
	}
	return errors.New("refusing a source path outside the competition catalog")
}

func (c *Client) once(ctx context.Context, method string, endpoint *url.URL, form url.Values) ([]byte, int, string, error) {
	var bodyReader io.Reader
	if method == http.MethodPost {
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
	if len(body) == 0 {
		return nil, resp.StatusCode, retry, errors.New("source returned an empty body")
	}
	lower := strings.ToLower(string(body[:min(len(body), 512)]))
	if strings.Contains(lower, "cf-browser-verification") || strings.Contains(lower, "captcha") {
		return nil, resp.StatusCode, retry, errors.New("source returned a block page")
	}
	got := strings.ToLower(resp.Header.Get("Content-Type"))
	if !strings.Contains(got, "text/html") && !strings.Contains(got, "application/xhtml") && !strings.Contains(got, "text/plain") && got != "" {
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

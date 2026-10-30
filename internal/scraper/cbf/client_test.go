package cbf

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientRejectsUnsafePathsAndWrongType(t *testing.T) {
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>ok</html>"))
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(ClientOptions{BaseURL: server.URL, MinInterval: time.Millisecond, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, path := range []string{"https://evil.test/x", "/../secret", "relative"} {
		if _, err := client.GetJSON(ctx, path); err == nil {
			t.Fatalf("path %s was accepted", path)
		}
	}
	if _, err := client.GetJSON(ctx, "/payload"); err == nil {
		t.Fatal("html was accepted as json")
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("wrong content type was retried: hits=%d", hits)
	}
}

func TestClientRetries429AndHonorsRetryAfter(t *testing.T) {
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(ClientOptions{BaseURL: server.URL, MinInterval: time.Millisecond, Timeout: 2 * time.Second, Retries: 1})
	if err != nil {
		t.Fatal(err)
	}
	body, err := client.GetJSON(context.Background(), "/payload")
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"ok":true}` || atomic.LoadInt32(&hits) != 2 {
		t.Fatalf("body %s hits %d", body, hits)
	}
}

func TestClientDoesNotFollowOffHostRedirectsOrOversizedBodies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/away":
			http.Redirect(w, r, "https://example.com/nope", http.StatusFound)
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"value":"too-long"}`))
		}
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(ClientOptions{BaseURL: server.URL, MinInterval: time.Millisecond, Timeout: time.Second, MaxBody: 8, Retries: 0})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetJSON(context.Background(), "/away"); err == nil {
		t.Fatal("off-host redirect was accepted")
	}
	if _, err := client.GetJSON(context.Background(), "/big"); err == nil {
		t.Fatal("oversized body was accepted")
	}
}

func TestClientRejectsABlockPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html>please complete the captcha</html>`))
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(ClientOptions{BaseURL: server.URL, MinInterval: time.Millisecond, Timeout: time.Second, Retries: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetHTML(context.Background(), "/page"); err == nil {
		t.Fatal("captcha page was accepted")
	}
}

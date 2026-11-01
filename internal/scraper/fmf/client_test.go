package fmf

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientPostsOnlyTheFixedDivisionPath(t *testing.T) {
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.URL.RequestURI() != "/Competicoes/ProxJogos.aspx?d=1" {
			t.Errorf("uri = %s", r.URL.RequestURI())
		}
		if r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			if string(body) != "request=1899" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
				t.Errorf("body = %q type = %s", body, r.Header.Get("Content-Type"))
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html>ok</html>"))
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(ClientOptions{BaseURL: server.URL, MinInterval: time.Millisecond, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := client.GetDivision(ctx, "9"); err == nil {
		t.Fatal("unknown division was accepted")
	}
	if _, err := client.PostSeason(ctx, "1", "1899;drop"); err == nil {
		t.Fatal("non-digit season id was accepted")
	}
	if _, err := client.GetDivision(ctx, "1"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.PostSeason(ctx, "1", "1899"); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&hits) != 2 {
		t.Fatalf("hits = %d", hits)
	}
}

func TestClientRefusesAnotherHostAndABlockPage(t *testing.T) {
	if _, err := NewClient(ClientOptions{BaseURL: "https://example.com"}); err == nil {
		t.Fatal("foreign host was accepted")
	}
	if _, err := NewClient(ClientOptions{BaseURL: "http://www.fmf.com.br"}); err == nil {
		t.Fatal("http origin was accepted")
	}
	if _, err := NewClient(ClientOptions{}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/away" {
			http.Redirect(w, r, "https://example.com/nope", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>captcha</html>"))
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(ClientOptions{BaseURL: server.URL, MinInterval: time.Millisecond, Timeout: time.Second, Retries: 0})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetDivision(context.Background(), "1"); err == nil {
		t.Fatal("block page was accepted")
	}
}

func TestClientDoesNotFollowAnOffHostRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://example.com/nope", http.StatusFound)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(ClientOptions{BaseURL: server.URL, MinInterval: time.Millisecond, Timeout: time.Second, Retries: 0})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetDivision(context.Background(), "2"); err == nil {
		t.Fatal("off-host redirect was accepted")
	}
}

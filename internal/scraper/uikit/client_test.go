package uikit

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientStaysOnTheFixedPaths(t *testing.T) {
	site, _, ok := Lookup("maranhense-serie-a")
	if !ok {
		t.Fatal("missing site")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/competicoes/tabela/1655" && r.URL.Path != "/competicoes/jogos_ajax.php" {
			t.Errorf("path %s", r.URL.RequestURI())
		}
		if r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			if string(body) == "" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
				t.Errorf("body %q", body)
			}
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>ok</html>"))
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(site, ClientOptions{BaseURL: server.URL, MinInterval: time.Millisecond, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := client.GetSeason(ctx, "../etc"); err == nil {
		t.Fatal("non-digit id was accepted")
	}
	if _, err := client.PostIndex(ctx, "26"); err == nil {
		t.Fatal("short year was accepted")
	}
	if _, err := client.GetSeason(ctx, "1655"); err != nil {
		t.Fatal(err)
	}
	query := FeedQuery{CompetitionID: "1655", CategoryID: "32", Championship: "256", Year: "2026"}
	if _, err := client.PostMatches(ctx, query, "3821"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.PostMatches(ctx, query, "3821;drop"); err == nil {
		t.Fatal("non-digit phase was accepted")
	}
}

func TestClientRefusesAnotherHostAndABlockPage(t *testing.T) {
	site, _, _ := Lookup("mato-grossense-primeira-divisao")
	if _, err := NewClient(site, ClientOptions{BaseURL: "https://example.com"}); err == nil {
		t.Fatal("foreign host was accepted")
	}
	if _, err := NewClient(site, ClientOptions{BaseURL: "http://www.fmfmt.com.br"}); err == nil {
		t.Fatal("http origin was accepted")
	}
	if _, err := NewClient(site, ClientOptions{}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>captcha</html>"))
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(site, ClientOptions{BaseURL: server.URL, MinInterval: time.Millisecond, Timeout: time.Second, Retries: 0})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetIndex(context.Background()); err == nil || !strings.Contains(err.Error(), "block") {
		t.Fatal(err)
	}
}

package fpfpr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientRefusesAnotherHostAndAMatchPage(t *testing.T) {
	if _, err := NewClient(ClientOptions{BaseURL: "https://example.com"}); err == nil {
		t.Fatal("foreign host was accepted")
	}
	if _, err := NewClient(ClientOptions{BaseURL: "http://federacaopr.com.br"}); err == nil {
		t.Fatal("http origin was accepted")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("<html></html>")); err != nil {
			t.Errorf("write: %v", err)
		}
	}))
	defer srv.Close()
	client, err := NewClient(ClientOptions{BaseURL: srv.URL, MinInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetSeason(context.Background(), "2026", "33"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.do(context.Background(), "/campeonato/profissional/2026/33/?jogo=1"); err == nil {
		t.Fatal("query was accepted")
	}
	if _, err := client.do(context.Background(), "/jogo/1043/"); err == nil {
		t.Fatal("match page was accepted")
	}
	if _, err := client.do(context.Background(), "/campeonato/../secret"); err == nil {
		t.Fatal("parent path was accepted")
	}
}

package fcf

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientRefusesAnotherHostAndAQuery(t *testing.T) {
	if _, err := NewClient(ClientOptions{BaseURL: "https://example.com"}); err == nil {
		t.Fatal("foreign host was accepted")
	}
	if _, err := NewClient(ClientOptions{BaseURL: "http://fcf.com.br"}); err == nil {
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
	if _, err := client.GetSeason(context.Background(), "2026", "686"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.do(context.Background(), "/competicoes/profissionais/2026/686/?fcf_jogo=1"); err == nil {
		t.Fatal("query was accepted")
	}
	if _, err := client.do(context.Background(), "/competicoes/../secret"); err == nil {
		t.Fatal("parent path was accepted")
	}
}

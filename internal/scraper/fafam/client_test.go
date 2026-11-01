package fafam

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientRefusesAnotherHostAndASumula(t *testing.T) {
	if _, err := NewClient(ClientOptions{BaseURL: "https://example.com"}); err == nil {
		t.Fatal("foreign host was accepted")
	}
	if _, err := NewClient(ClientOptions{BaseURL: "http://fafamazonas.com.br"}); err == nil {
		t.Fatal("http origin was accepted")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html></html>"))
	}))
	defer srv.Close()
	client, err := NewClient(ClientOptions{BaseURL: srv.URL, MinInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetRound(context.Background(), "campeonato-amazonense-serie-a-2026", "3"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.do(context.Background(), "/site/arquivos/imagens/sumula/jogo.pdf"); err == nil {
		t.Fatal("sumula was accepted")
	}
	if _, err := client.GetRound(context.Background(), "amazonense-feminino-2026", "3"); err == nil {
		t.Fatal("women's season was accepted")
	}
	if _, err := client.do(context.Background(), "/site/campeonato/campeonato-amazonense-serie-a-2026/3/?jogo=1"); err == nil {
		t.Fatal("query was accepted")
	}
}

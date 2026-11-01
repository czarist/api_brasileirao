package fsfse

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
	if _, err := NewClient(ClientOptions{BaseURL: "http://fsf-se.com.br"}); err == nil {
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
	if _, err := client.GetSeason(context.Background(), "sergipano-a1-2026"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.do(context.Background(), "/campeonatos/sergipano-a1-2026/?jogo=1"); err == nil {
		t.Fatal("query was accepted")
	}
	if _, err := client.do(context.Background(), "/jogo/50762/arbitragem/"); err == nil {
		t.Fatal("match page was accepted")
	}
	if _, err := client.do(context.Background(), "/campeonatos/copa-lotese-2026/"); err == nil {
		t.Fatal("cup page was accepted")
	}
	if _, err := client.GetSeason(context.Background(), "sergipano-sub-20-a1-2026"); err == nil {
		t.Fatal("youth season was accepted")
	}
}

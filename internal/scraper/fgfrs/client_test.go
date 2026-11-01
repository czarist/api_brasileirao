package fgfrs

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
	if _, err := NewClient(ClientOptions{BaseURL: "http://www.fgf.com.br"}); err == nil {
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
	ctx := context.Background()
	if _, err := client.GetDivision(ctx, "23"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetYear(ctx, "24", "2026"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetPhase(ctx, "25", "2017", "4338"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetDivision(ctx, "26"); err == nil {
		t.Fatal("copa division was accepted")
	}
	if _, err := client.do(ctx, "/competicoes/profissional/23/2026/3746?jogo=1"); err == nil {
		t.Fatal("query was accepted")
	}
	if _, err := client.do(ctx, "/jogo/gremioxinternacional-gauchao-01-03-2026"); err == nil {
		t.Fatal("match page was accepted")
	}
	if _, err := client.do(ctx, "/competicoes/../secret"); err == nil {
		t.Fatal("parent path was accepted")
	}
}

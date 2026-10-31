package cbf

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchUsesTheFixedSerieAPaths(t *testing.T) {
	catalog := readFixture(t, "testdata/catalog_sample.json")
	matches := readFixture(t, "testdata/matches_sample.json")
	standings := readFixture(t, "testdata/standings_sample.html")
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		switch r.URL.Path {
		case "/api/cbf/campeonatos/anos":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(catalog)
		case "/api/cbf/jogos/tabela-detalhada/campeonato/1260611":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(matches)
		case "/api/cbf/campeonatos/1260611/imt":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]"))
		case "/futebol-brasileiro/tabelas/campeonato-brasileiro/serie-a/2026":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(standings)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(ClientOptions{BaseURL: server.URL, MinInterval: time.Millisecond, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	snap, err := NewSource(client).Fetch(context.Background(), "brasileirao-serie-a", 2026, time.Unix(100, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if snap.SeasonExternalID != "1260611" || len(snap.Matches) != 4 || len(snap.Standings) != 3 {
		t.Fatalf("snapshot matches=%d standings=%d", len(snap.Matches), len(snap.Standings))
	}
	want := []string{
		"/api/cbf/campeonatos/anos",
		"/api/cbf/jogos/tabela-detalhada/campeonato/1260611",
		"/api/cbf/campeonatos/1260611/imt",
		"/futebol-brasileiro/tabelas/campeonato-brasileiro/serie-a/2026",
	}
	if len(paths) != len(want) {
		t.Fatalf("paths = %v", paths)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("paths = %v", paths)
		}
	}
}

func TestFetchLoadsThePointsPhaseWhenTheYearPageHasNoTable(t *testing.T) {
	catalog := []byte(`{"campeonatos":[{"id":"42","categorias":[{"id":"3","anos":"2026","id_campeonato":"1260613"}]}]}`)
	matches := readFixture(t, "testdata/matches_sample.json")
	standings := readFixture(t, "testdata/standings_sample.html")
	yearPage := []byte(`<html>{\"fase_id\":\"2117\",\"fase_tipo\":\"eliminacao\"},{\"fase_id\":\"2013\",\"fase_tipo\":\"pontuacao\"}</html>`)
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/api/cbf/campeonatos/anos":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(catalog)
		case "/api/cbf/jogos/tabela-detalhada/campeonato/1260613":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(matches)
		case "/api/cbf/campeonatos/1260613/imt":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]"))
		case "/futebol-brasileiro/tabelas/campeonato-brasileiro/serie-c/2026":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write(yearPage)
		case "/futebol-brasileiro/tabelas/campeonato-brasileiro/serie-c/2026/2013":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write(standings)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(ClientOptions{BaseURL: server.URL, MinInterval: time.Millisecond, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	source := NewSource(client)
	plans, err := source.Plan(context.Background(), []string{"brasileirao-serie-c", "brasileirao-serie-d"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0].Year != 2026 || plans[0].ExternalID != "1260613" || plans[0].Competition.Code != "brasileirao-serie-c" {
		t.Fatalf("plans = %+v", plans)
	}
	snap, err := source.Fetch(context.Background(), "brasileirao-serie-c", 2026, time.Unix(100, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if snap.CompetitionCode != "brasileirao-serie-c" || len(snap.Standings) != 3 {
		t.Fatalf("snapshot code=%s standings=%d", snap.CompetitionCode, len(snap.Standings))
	}
	if paths[0] != "/api/cbf/campeonatos/anos" || paths[len(paths)-1] != "/futebol-brasileiro/tabelas/campeonato-brasileiro/serie-c/2026/2013" {
		t.Fatalf("paths = %v", paths)
	}
	catalogHits := 0
	for _, path := range paths {
		if path == "/api/cbf/campeonatos/anos" {
			catalogHits++
		}
	}
	if catalogHits != 1 {
		t.Fatalf("catalog requests = %d paths=%v", catalogHits, paths)
	}
}

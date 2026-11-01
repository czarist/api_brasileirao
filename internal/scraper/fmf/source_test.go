package fmf

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchPostsTheCatalogSeasonAndRejectsTheWrongYear(t *testing.T) {
	page := readFixture(t, "testdata/modulo_i_sample.html")
	var gets, posts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RequestURI() != "/Competicoes/ProxJogos.aspx?d=1" {
			t.Errorf("uri = %s", r.URL.RequestURI())
		}
		w.Header().Set("Content-Type", "text/html")
		if r.Method == http.MethodGet {
			gets++
			_, _ = w.Write(page)
			return
		}
		posts++
		_ = r.ParseForm()
		if r.Form.Get("request") == "1779" {
			_, _ = w.Write(page)
			return
		}
		if r.Form.Get("request") != "1899" {
			t.Errorf("request = %s", r.Form.Get("request"))
		}
		_, _ = w.Write(page)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(ClientOptions{BaseURL: server.URL, MinInterval: time.Millisecond, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	source := NewSource(client)
	plans, err := source.Plan(context.Background(), []string{"mineiro-modulo-i"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 2 || plans[0].Year != 2025 || plans[1].Year != 2026 || plans[1].ExternalID != "1899" {
		t.Fatalf("plans = %+v", plans)
	}
	snap, err := source.Fetch(context.Background(), "mineiro-modulo-i", 2026, time.Unix(100, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if snap.Source != "fmf" || snap.CompetitionCode != "mineiro-modulo-i" || snap.SeasonExternalID != "1899" || len(snap.Matches) != 2 {
		t.Fatalf("snapshot = %+v matches=%d", snap.SeasonExternalID, len(snap.Matches))
	}
	if _, err := source.Fetch(context.Background(), "mineiro-modulo-i", 2025, time.Unix(100, 0).UTC()); err == nil {
		t.Fatal("a page for 2026 was stored as 2025")
	}
	if gets != 1 || posts != 2 {
		t.Fatalf("gets=%d posts=%d", gets, posts)
	}
}

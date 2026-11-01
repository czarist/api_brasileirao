package portal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
)

func TestPlanAndFetchReadTheCatalogYear(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch {
		case r.URL.Path == "/pt/competicoes/" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(indexPage("2026", "10", "Candangão BRB Série A / 2026")))
		case r.URL.Path == "/pt/competicoes/" && r.Method == http.MethodPost:
			if err := r.ParseForm(); err != nil || r.Form.Get("exercicio") != "2025" {
				http.Error(w, "bad year", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(indexPage("2025", "11", "Candangão BRB Série A / 2025")))
		case r.URL.Path == "/pt/competicoes/jogos.php":
			id := r.URL.Query().Get("q")
			year := "2026"
			if id == "11" {
				year = "2025"
			}
			_, _ = w.Write([]byte(`<h3 class="mb-0 fs-title-h3">Candangão BRB Série A / ` + year + `</h3><script>url: 'jogos_ajax_ver3.php'; url: 'classificacao_ajax_ver3.php';</script><a class="nav-link fase" data-id="` + id + `" data-fase="3">Primeira Fase</a>`))
		case r.URL.Path == "/pt/competicoes/jogos_ajax_ver3.php":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"html":"<div class=\"title-rodada\">1ª Rodada</div><div class=\"pr-jogos\"><span class=\"cronolog-sm\">SAB, 10/01 - 15:00 - Jk</span><img alt=\"Gama\" src=\"escudo_clubes/10.png\"><div class=\"pe-2\">GAM</div><span class=\"text-end\">2</span><span class=\"text-start\">1</span><img alt=\"Brasiliense\" src=\"escudo_clubes/20.png\"><div class=\"ps-2\">BRA</div><a href=\"jogo.php?id=1&jogo=50017\">sobre</a></div>","rodada":1}`))
		case r.URL.Path == "/pt/competicoes/classificacao_ajax_ver3.php":
			_, _ = w.Write([]byte(`<table><tr><th colspan="3">GRUPO A</th></tr><tr><td>1º</td><td><img src="escudo_clubes/10.gif"></td><td><span>Gama</span><span>GAM</span></td><td>3</td><td>1</td><td>1</td><td>0</td><td>0</td><td>2</td><td>1</td><td>1</td><td>0</td><td>1</td><td>100</td></tr><tr><td>2º</td><td><img src="escudo_clubes/20.gif"></td><td><span>Brasiliense</span><span>BRA</span></td><td>0</td><td>1</td><td>0</td><td>0</td><td>1</td><td>1</td><td>2</td><td>-1</td><td>0</td><td>2</td><td>0</td></tr></table>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	site, _, _ := Lookup("candangao-serie-a")
	client, err := NewClient(site, ClientOptions{BaseURL: server.URL, MinInterval: time.Millisecond, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	source := NewSource(client)
	plans, err := source.Plan(context.Background(), []string{"candangao-serie-a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 2 || plans[0].Year != 2025 || plans[1].Year != 2026 {
		t.Fatalf("plans=%+v", plans)
	}
	snap, err := source.Fetch(context.Background(), "candangao-serie-a", 2026, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if snap.Source != "ffdf" || snap.SeasonExternalID != "10" || snap.SeasonName != "Candangão BRB Série A / 2026" || len(snap.Matches) != 1 || snap.Matches[0].Kickoff.Format(time.RFC3339) != "2026-01-10T18:00:00Z" {
		t.Fatalf("%+v", snap)
	}
	if snap.Teams[0].Name != "Gama" || snap.Standings[0].Points != 3 || snap.Standings[0].GroupName != "GRUPO A" || snap.Standings[1].YellowCards != 2 {
		t.Fatalf("teams=%+v standings=%+v", snap.Teams, snap.Standings)
	}
}

func TestMergeStandingsKeepsTheFirstTableWhenALaterOneOverlaps(t *testing.T) {
	first := []domain.StandingInput{{ExternalTeamID: "1", Position: 1}, {ExternalTeamID: "2", Position: 2}}
	overlap := []domain.StandingInput{{ExternalTeamID: "1", Position: 1}, {ExternalTeamID: "3", Position: 1}}
	next := []domain.StandingInput{{ExternalTeamID: "9", GroupName: "GRUPO B", Position: 1}}
	got, err := mergeStandings([][]domain.StandingInput{first, overlap, next})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].ExternalTeamID != "1" || got[2].ExternalTeamID != "9" || got[2].GroupOrder != 1 {
		t.Fatalf("%+v", got)
	}
}

func TestClientRefusesAnotherHost(t *testing.T) {
	site, _, _ := Lookup("candangao-serie-a")
	if _, err := NewClient(site, ClientOptions{BaseURL: "https://example.com"}); err == nil {
		t.Fatal("accepted example.com")
	}
	if _, err := NewClient(site, ClientOptions{}); err != nil {
		t.Fatal(err)
	}
}

func indexPage(selected, id, title string) string {
	other := "2025"
	if selected == "2025" {
		other = "2026"
	}
	return `<select><option value="` + other + `">` + other + `</option><option value="` + selected + `" selected>` + selected + `</option></select><a href="jogos.php?q=` + id + `">` + title + `</a>` +
		strings.Repeat(" ", 0)
}

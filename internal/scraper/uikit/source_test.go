package uikit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPlanAndFetchReadTheCatalogYear(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch {
		case r.URL.Path == "/competicoes/" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`<select><option value="2026" selected>2026</option></select><a href="/competicoes/tabela/1655" title="Campeonato Maranhense Série A - 2026">a</a>`))
		case r.URL.Path == "/competicoes/tabela/1655":
			_, _ = w.Write([]byte(`<a href="/competicoes/tabela/1655/3821">Primeira Fase</a><script>{"idcampeonato": 1655, "categoria": 32, "campeonato": 256, "exercicio": 2026}</script>`))
		case r.URL.Path == "/competicoes/jogos_ajax.php":
			_, _ = w.Write([]byte(`<h5 class="uk-card-title">1ª Rodada</h5><p>SAB, 10/01/2026 - 16h00 - Castelão</p><span uk-tooltip="Maranhão"><img src="https://conteudo.cbf.com.br/clubes/20043/escudo.jpg"></span><h4>2 X 0</h4><span uk-tooltip="Iape"><img src="https://conteudo.cbf.com.br/clubes/21894/escudo.jpg"></span><a href="/competicoes/jogo/1655/51796">veja</a>`))
		case r.URL.Path == "/competicoes/classificacao_ajax.php":
			_, _ = w.Write([]byte(`<table><thead><tr><th colspan="3">Classificação</th><th>Pts</th><th>J</th><th>V</th><th>E</th><th>D</th><th>GP</th><th>GC</th><th>SG</th><th>CA</th><th>CV</th><th>%</th></tr></thead><tr><td>1º</td><td><img src="https://conteudo.cbf.com.br/clubes/20043/escudo.jpg"></td><td>Maranhão</td><td>3</td><td>1</td><td>1</td><td>0</td><td>0</td><td>2</td><td>0</td><td>2</td><td>1</td><td>0</td><td>100</td></tr><tr><td>2º</td><td><img src="https://conteudo.cbf.com.br/clubes/21894/escudo.jpg"></td><td>Iape</td><td>0</td><td>1</td><td>0</td><td>0</td><td>1</td><td>0</td><td>2</td><td>-2</td><td>4</td><td>1</td><td>0</td></tr></table>`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	site, _, _ := Lookup("maranhense-serie-a")
	client, err := NewClient(site, ClientOptions{BaseURL: server.URL, MinInterval: time.Millisecond, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	source := NewSource(client)
	plans, err := source.Plan(context.Background(), []string{"maranhense-serie-a"})
	if err != nil || len(plans) != 1 || plans[0].Year != 2026 {
		t.Fatalf("%v %+v", err, plans)
	}
	snap, err := source.Fetch(context.Background(), "maranhense-serie-a", 2026, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if snap.Source != "fmf-ma" || snap.CompetitionName != "Campeonato Maranhense Série A" || snap.SeasonName != "Campeonato Maranhense Série A - 2026" || len(snap.Matches) != 1 {
		t.Fatalf("%+v", snap)
	}
	if snap.Matches[0].Kickoff == nil || snap.Matches[0].Kickoff.Format(time.RFC3339) != "2026-01-10T19:00:00Z" {
		t.Fatalf("kickoff %+v", snap.Matches[0].Kickoff)
	}
	if snap.Standings[0].YellowCards != 1 || snap.Standings[0].RedCards != 0 || snap.Standings[1].YellowCards != 4 || snap.Teams[0].Name != "Maranhão" {
		t.Fatalf("standings=%+v teams=%+v", snap.Standings, snap.Teams)
	}
}

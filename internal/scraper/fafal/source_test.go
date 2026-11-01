package fafal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPlanAndFetchReadEachCatalogYear(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.RequestURI() {
		case "/novo/competicoes/":
			_, _ = w.Write([]byte(`<a href="/novo/competicoes/2026">2026</a><a href="/novo/competicoes/2025">2025</a>
var ano = "2026";
<a href="/novo/tabela?ID=1633"><img src="/novo/imagens/competicoes/alagoano-2026.png"></a>
<a href="/novo/tabela?ID=9"><img src="/novo/imagens/competicoes/alagoano-feminino-2026.png"></a>`))
		case "/novo/competicoes/2025":
			_, _ = w.Write([]byte(`var ano = "2025";
<a href="/novo/tabela?ID=1500"><img src="/novo/imagens/competicoes/alagoano-2025.png"></a>`))
		case "/novo/tabela?ID=1633":
			_, _ = w.Write([]byte(`<meta property="og:title" content="Alagoano Série A / 2026" />
<a href="?ID=1633&fase=3743">Primeira Fase</a>
<a href="?ID=1633&fase=3745">Final</a>`))
		case "/novo/tabela?ID=1633&fase=3743":
			_, _ = w.Write([]byte(phasePage("50416", "1 X 0", "17", "99")))
		case "/novo/tabela?ID=1633&fase=3745":
			_, _ = w.Write([]byte(`<div id="bloco-rodada-1"><p>SAB, 12/04/2026 16h00 Rei Pelé</p>
<span title="Asa"><img src="escudo_clubes/155993103617.gif"></span><h3>2 X 1</h3>
<span title="CSA"><img src="escudo_clubes/155993097246.gif"></span>
<a href="/novo/jogo?id=1633&jogo=60000">veja</a></div>
<table><thead><tr><th colspan="3">Classificação</th><th>Pts</th><th>J</th><th>V</th><th>E</th><th>D</th><th>GP</th><th>GC</th><th>SG</th><th>CA</th><th>CV</th><th>%</th></tr></thead>
<tr><td>1</td><td><img src="escudo_clubes/155993103617.gif"></td><td>Asa</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td></tr>
<tr><td>2</td><td><img src="escudo_clubes/155993097246.gif"></td><td>CSA</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td></tr></table>`))
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
	plans, err := source.Plan(context.Background(), []string{"alagoano-serie-a"})
	if err != nil || len(plans) != 2 || plans[0].Year != 2025 || plans[1].Year != 2026 {
		t.Fatalf("%v %+v", err, plans)
	}
	snap, err := source.Fetch(context.Background(), "alagoano-serie-a", 2026, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if snap.Source != "faf-al" || snap.CompetitionName != "Alagoano Série A" || snap.SeasonName != "Alagoano Série A / 2026" || snap.SeasonExternalID != "1633" {
		t.Fatalf("%+v", snap)
	}
	if len(snap.Matches) != 2 || snap.Matches[0].Kickoff == nil || snap.Matches[0].Kickoff.Format(time.RFC3339) != "2026-01-10T19:00:00Z" {
		t.Fatalf("matches %+v", snap.Matches)
	}
	if len(snap.Standings) != 2 || snap.Standings[0].GoalDifference != 99 || snap.Standings[0].YellowCards != 15 || snap.Standings[0].RedCards != 0 {
		t.Fatalf("standings %+v", snap.Standings)
	}
	if snap.Teams[0].Name != "Asa" {
		t.Fatalf("teams %+v", snap.Teams)
	}
}

func TestClientRefusesAnotherHost(t *testing.T) {
	if _, err := NewClient(ClientOptions{BaseURL: "https://example.com"}); err == nil {
		t.Fatal("foreign host was accepted")
	}
	if _, err := NewClient(ClientOptions{BaseURL: "http://www.futeboldealagoas.net"}); err == nil {
		t.Fatal("plain http origin was accepted")
	}
}

func phasePage(jogo, score, points, goalDifference string) string {
	return `<div id="bloco-rodada-1"><p>SAB, 10/01/2026 16h00 Alfredo Leahy</p>
<span class="uk-text-bold">ASA</span><span title="Asa"><img src="escudo_clubes/155993103617.gif"></span>
<h3>` + score + `</h3>
<span title="CSA"><img src="escudo_clubes/155993097246.gif"></span><span class="uk-text-bold">CSA</span>
<a href="/novo/jogo?id=1633&jogo=` + jogo + `">veja</a></div>
<table><thead><tr><th colspan="3">Classificação</th><th>Pts</th><th>J</th><th>V</th><th>E</th><th>D</th><th>GP</th><th>GC</th><th>SG</th><th>CA</th><th>CV</th><th>%</th></tr></thead>
<tr><td colspan="14">GRUPO A</td></tr>
<tr><td>1º</td><td><img src="escudo_clubes/155993103617.gif"></td><td>Asa</td><td>` + points + `</td><td>7</td><td>5</td><td>2</td><td>0</td><td>13</td><td>3</td><td>` + goalDifference + `</td><td>15</td><td>0</td><td>80</td></tr>
<tr><td>2º</td><td><img src="escudo_clubes/155993097246.gif"></td><td>CSA</td><td>10</td><td>7</td><td>3</td><td>1</td><td>3</td><td>8</td><td>7</td><td>1</td><td>4</td><td>1</td><td>40</td></tr>
</table>`
}

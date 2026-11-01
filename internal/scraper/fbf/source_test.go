package fbf

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPlanAndFetchReadTheAdultDivisions(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		if r.URL.RawQuery != "" {
			t.Errorf("query %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/competicoes/1":
			_, _ = w.Write([]byte(`<select>
<option selected value="https://www.fbf.org.br/competicoes/1/2025">2025</option>
<option selected value="https://www.fbf.org.br/competicoes/1/2027">2027</option>
<option selected value="https://www.fbf.org.br/competicoes/3/2025">2025</option>
</select>`))
		case "/competicoes/1/2025":
			_, _ = w.Write([]byte(`<meta property="og:url" content="https://www.fbf.org.br/competicoes/1/2025">
<ul class="fase-links">
<li><a href="https://www.fbf.org.br/competicoes/1/2025/405">Fase 1 - Classificação</a></li>
<li><a href="https://www.fbf.org.br/competicoes/1/2025/409">Fase 3 - Final</a></li>
<li><a href="https://www.fbf.org.br/competicoes/3/2025/9">Feminino</a></li>
</ul>`))
		case "/competicoes/1/2025/405":
			_, _ = w.Write([]byte(samplePhase(true)))
		case "/competicoes/1/2025/409":
			_, _ = w.Write([]byte(finalPhase()))
		case "/competicoes/1/2027":
			_, _ = w.Write([]byte(`<meta property="og:url" content="https://www.fbf.org.br/competicoes/1/2027">
<ul class="fase-links"><li><a href="/competicoes/1/2027/465">Fase 1 - Classificação</a></li></ul>`))
		case "/competicoes/1/2027/465":
			_, _ = w.Write([]byte(undefinedCard()))
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
	plans, err := source.Plan(context.Background(), []string{"baiano-serie-a"})
	if err != nil || len(plans) != 2 || plans[0].Year != 2025 || plans[1].Year != 2027 {
		t.Fatalf("%v %+v", err, plans)
	}
	snap, err := source.Fetch(context.Background(), "baiano-serie-a", 2025, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if snap.Source != "fbf" || snap.CompetitionName != "Baianão Série A" || snap.SeasonName != "Baianão Série A 2025" || snap.SeasonExternalID != "1-2025" {
		t.Fatalf("%+v", snap)
	}
	if len(snap.Matches) != 2 || len(snap.Standings) != 2 || snap.Standings[0].Points != 21 || snap.Standings[0].GoalDifference != 99 {
		t.Fatalf("matches %d standings %+v", len(snap.Matches), snap.Standings)
	}
	if snap.Teams[0].Name != "Vitória" || snap.Teams[0].ShortName != "ECV" {
		t.Fatalf("teams %+v", snap.Teams)
	}
	for _, path := range paths {
		if strings.Contains(path, "/competicoes/3") || strings.Contains(path, "/competicoes/2") {
			t.Fatalf("requested %s", path)
		}
	}
	undefined, err := source.Fetch(context.Background(), "baiano-serie-a", 2027, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if undefined.Matches[0].HomeScore != nil || undefined.Matches[0].Kickoff != nil {
		t.Fatalf("%+v", undefined.Matches[0])
	}
}

func TestClientRefusesAnotherHost(t *testing.T) {
	if _, err := NewClient(ClientOptions{BaseURL: "https://example.com"}); err == nil {
		t.Fatal("foreign host was accepted")
	}
	if _, err := NewClient(ClientOptions{BaseURL: "http://www.fbf.org.br"}); err == nil {
		t.Fatal("plain http origin was accepted")
	}
	client, err := NewClient(ClientOptions{BaseURL: "https://www.fbf.org.br", MinInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetDivision(context.Background(), "3"); err == nil {
		t.Fatal("feminino division was accepted")
	}
	if _, err := client.GetPhase(context.Background(), "1", "2025", ".."); err == nil {
		t.Fatal("a phase id with .. was accepted")
	}
	if _, err := client.GetSeason(context.Background(), "1", "25"); err == nil {
		t.Fatal("a short year was accepted")
	}
}

func finalPhase() string {
	return `
<div class="section-title-table"><h4 class="title text-black">Grupo 4</h4></div>
<table class="table"><thead><tr>
<th>Posição</th><th>P</th><th>J</th><th>V</th><th>E</th><th>D</th><th>GP</th><th>GC</th><th>SG</th><th>%</th>
</tr></thead><tbody>
<tr><td>1º <img src="/images/clubes/2024041300102017_logobahia (1).png"><span class="team-meta__place">Bahia</span></td>
<td>1</td><td>1</td><td>0</td><td>1</td><td>0</td><td>1</td><td>1</td><td>0</td><td>0</td></tr>
<tr><td>2º <img src="/images/clubes/202401180423newecv.png"><span class="team-meta__place">Vitória</span></td>
<td>1</td><td>1</td><td>0</td><td>1</td><td>0</td><td>1</td><td>1</td><td>0</td><td>0</td></tr>
</tbody></table>
<div class="carousel-item"><div class="carousel-header"><h4 class="title text-black">Rodada 13</h4></div>
<div class="d-flex flex-column">
<div class="data-hora-card-jogo"><span>Arena Fonte Nova <br/>16/03/2025 18:00</span></div>
<div class="d-flex flex-confronto-card">
<div><span class="time-confronto">ECB</span><img src="/images/clubes/2024041300102017_logobahia (1).png"></div>
<div class="hora-confronto-card"><span>2</span><span></span><span>0</span></div>
<div><img src="/images/clubes/202401180423newecv.png"><span class="time-confronto">ECV</span></div>
</div>
<a href="https://www.fbf.org.br/jogo/detalhes/11017">DETALHES DE JOGO</a>
</div></div>`
}

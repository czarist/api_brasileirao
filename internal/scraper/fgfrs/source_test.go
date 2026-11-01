package fgfrs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
)

func TestFetchReadsEachPhaseAndSkipsAnEmptyOne(t *testing.T) {
	const division = `<select><option value="http://fgf.com.br/competicoes/profissional/23/2026">2026</option></select>`
	const year = `<div>
<a class="menu_comp" href="http://fgf.com.br/competicoes/profissional/23/2026/1">Classificatória</a>
<a class="menu_comp" href="http://fgf.com.br/competicoes/profissional/23/2026/2">Copa FGF</a>
<a class="menu_comp" href="http://fgf.com.br/competicoes/profissional/23/2026/3">Final</a>
</div>`
	const group = `<div>
<h4>RODADA 1</h4>
<div class="carousel-conteudo">
  <div class="conteudo-descricao"><div>Sáb, 10/01 16:00 - Alfredo Jaconi</div></div>
  <div class="mandante"><img src="/clubes/juventude.jpg" title="Esporte Clube Juventude"></div>
  <div class="contra"><div>1 X 1</div><a href="/jogo/juventudexypiranga-gauchao-10-01-2026">Sobre o Jogo</a></div>
  <div class="visitante"><img src="/clubes/ypiranga.png" title="Ypiranga"></div>
</div>
<div class="titulo-grupo"><h4>GRUPO A</h4></div>
<div class="tabela"><table><tr>
  <td class="posicao-time"><b>1º</b><img src="/clubes/juventude.jpg" title="Juventude"></td>
  <td class="posicao-time2">JUV</td>
  <td title="Pontos Ganhos">3</td><td title="Jogos">1</td><td title="Vitórias">0</td><td title="Empates">1</td><td title="Derrotas">0</td>
  <td title="Gols Pró">1</td><td title="Gols Contra">1</td><td title="Saldo de Gols">0</td>
  <td title="Cartões Amarelos">2</td><td title="Cartões Vermelhos">0</td><td title="Aproveitamento">33</td>
</tr><tr>
  <td class="posicao-time"><b>2º</b><img src="/clubes/ypiranga.png" title="Ypiranga"></td>
  <td class="posicao-time2">YPI</td>
  <td title="Pontos Ganhos">1</td><td title="Jogos">1</td><td title="Vitórias">0</td><td title="Empates">1</td><td title="Derrotas">0</td>
  <td title="Gols Pró">1</td><td title="Gols Contra">1</td><td title="Saldo de Gols">0</td>
  <td title="Cartões Amarelos">1</td><td title="Cartões Vermelhos">0</td><td title="Aproveitamento">33</td>
</tr></table></div>
</div>`
	const empty = `<html><body>sem jogos</body></html>`
	const final = `<div class="bordas2">
<div class="conteudo-descricao2"><div>Dom, 01/03 18:00 - Arena do Grêmio</div><div>1 alteração</div></div>
<div class="mandante"><img src="/clubes/juventude.jpg" title="Juventude"></div>
<div class="contra2">0 X 0</div>
<div class="visitante"><img src="/clubes/ypiranga.png" title="Ypiranga"></div>
<a href="/jogo/juventudexypiranga-gauchao-01-03-2026">Sobre o jogo</a>
</div>
<div class="titulo-grupo"><h4>FINAL</h4></div>
<div class="tabela"><table><tr>
  <td class="posicao-time"><b>1º</b><img src="/clubes/juventude.jpg" title="Juventude"></td>
  <td class="posicao-time2">JUV</td>
  <td title="Pontos Ganhos">4</td><td title="Jogos">2</td><td title="Vitórias">1</td><td title="Empates">1</td><td title="Derrotas">0</td>
  <td title="Gols Pró">1</td><td title="Gols Contra">1</td><td title="Saldo de Gols">0</td>
  <td title="Cartões Amarelos">2</td><td title="Cartões Vermelhos">0</td><td title="Aproveitamento">67</td>
</tr></table></div>`
	var phases []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/competicoes/profissional/23":
			_, _ = w.Write([]byte(division))
		case "/competicoes/profissional/23/2026":
			_, _ = w.Write([]byte(year))
		case "/competicoes/profissional/23/2026/1":
			phases = append(phases, "1")
			_, _ = w.Write([]byte(group))
		case "/competicoes/profissional/23/2026/3":
			phases = append(phases, "3")
			_, _ = w.Write([]byte(final))
		case "/competicoes/profissional/23/2026/9":
			_, _ = w.Write([]byte(empty))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	client, err := NewClient(ClientOptions{BaseURL: srv.URL, MinInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	source := NewSource(client)
	plans, err := source.Plan(context.Background(), []string{"gauchao"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0].Code != "gauchao" || plans[0].Year != 2026 {
		t.Fatalf("plans = %+v", plans)
	}
	snap, err := source.Fetch(context.Background(), "gauchao", 2026, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if snap.Source != domain.SourceFGFRS || snap.SeasonExternalID != "23-2026" || snap.CompetitionName != "Gauchão" || len(snap.Matches) != 2 || len(snap.Standings) != 2 {
		t.Fatalf("snap matches %d standings %d id %s", len(snap.Matches), len(snap.Standings), snap.SeasonExternalID)
	}
	if snap.Matches[1].Status != domain.StatusFinished || *snap.Matches[1].HomeScore != 0 || snap.Matches[1].Phase != "Final" {
		t.Fatalf("final = %+v", snap.Matches[1])
	}
	if snap.Standings[0].ExternalTeamID != "juventude" || snap.Standings[0].Played != 2 || snap.Standings[0].GroupName != "FINAL" || snap.Standings[0].Points != 4 {
		t.Fatalf("leader = %+v", snap.Standings[0])
	}
	var youth domain.TeamInput
	for _, team := range snap.Teams {
		if team.ExternalID == "juventude" {
			youth = team
		}
	}
	if youth.Name != "Juventude" || youth.ShortName != "JUV" || youth.State != "RS" {
		t.Fatalf("team = %+v", youth)
	}
	if len(phases) != 2 {
		t.Fatalf("phases = %v", phases)
	}
	if err := snap.Validate(Limits()); err == nil {
		t.Fatal("two matches passed the eight-match limit")
	}
}

func TestPlanSkipsAYearPageWithNoMatches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/competicoes/profissional/23":
			_, _ = w.Write([]byte(`<option value="/competicoes/profissional/23/2017">2017</option><option value="/competicoes/profissional/23/2019">2019</option>`))
		case "/competicoes/profissional/23/2017":
			_, _ = w.Write([]byte(`<h3>Gauchão <span>2017</span></h3>`))
		case "/competicoes/profissional/23/2019":
			_, _ = w.Write([]byte(`<a class="menu_comp" href="/competicoes/profissional/23/2019/1">Classificatória</a>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	client, err := NewClient(ClientOptions{BaseURL: srv.URL, MinInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	plans, err := NewSource(client).Plan(context.Background(), []string{"gauchao"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0].Year != 2019 {
		t.Fatalf("plans = %+v", plans)
	}
}

func TestPlanSkipsAYearWhosePhasesBelongToAnotherYear(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/competicoes/profissional/25":
			_, _ = w.Write([]byte(`<option value="/competicoes/profissional/25/2019">2019</option><option value="/competicoes/profissional/25/2020">2020</option>`))
		case "/competicoes/profissional/25/2019":
			_, _ = w.Write([]byte(`<a class="menu_comp" href="/competicoes/profissional/25/2019/138">Classificatória</a>`))
		case "/competicoes/profissional/25/2020":
			_, _ = w.Write([]byte(`<a class="menu_comp" href="http://fgf.com.br/competicoes/profissional/25/2019/138">Classificatória</a><a class="menu_comp" href="/competicoes/profissional/25/2019/242">Quartas</a>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	client, err := NewClient(ClientOptions{BaseURL: srv.URL, MinInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	source := NewSource(client)
	plans, err := source.Plan(context.Background(), []string{"gauchao-serie-b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0].Year != 2019 {
		t.Fatalf("plans = %+v", plans)
	}
	_, err = source.Fetch(context.Background(), "gauchao-serie-b", 2020, time.Now())
	if err == nil || !strings.Contains(err.Error(), "another year") {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchUsesTheYearPageWhenItHasNoPhaseLink(t *testing.T) {
	const division = `<option value="/competicoes/profissional/24/2025">2025</option>`
	const year = `<h4>RODADA 3</h4>
<div class="carousel-conteudo">
  <div class="conteudo-descricao"><div>10/05 20:00 - Estádio</div></div>
  <div class="mandante"><img src="/clubes/a.png" title="A"></div>
  <div class="contra">2 X 0</div>
  <div class="visitante"><img src="/clubes/b.png" title="B"></div>
  <a href="/jogo/axb">Sobre o jogo</a>
</div>
<div class="titulo-grupo"><h4>GRUPO</h4></div>
<div class="tabela"><table><tr>
  <td class="posicao-time"><b>1º</b><img src="/clubes/a.png" title="A"></td><td class="posicao-time2">A</td>
  <td title="Pontos Ganhos">3</td><td title="Jogos">1</td><td title="Vitórias">1</td><td title="Empates">0</td><td title="Derrotas">0</td>
  <td title="Gols Pró">2</td><td title="Gols Contra">0</td><td title="Saldo de Gols">2</td>
  <td title="Cartões Amarelos">0</td><td title="Cartões Vermelhos">0</td><td title="Aproveitamento">100</td>
</tr><tr>
  <td class="posicao-time"><b>2º</b><img src="/clubes/b.png" title="B"></td><td class="posicao-time2">B</td>
  <td title="Pontos Ganhos">0</td><td title="Jogos">1</td><td title="Vitórias">0</td><td title="Empates">0</td><td title="Derrotas">1</td>
  <td title="Gols Pró">0</td><td title="Gols Contra">2</td><td title="Saldo de Gols">-2</td>
  <td title="Cartões Amarelos">0</td><td title="Cartões Vermelhos">0</td><td title="Aproveitamento">0</td>
</tr></table></div>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/competicoes/profissional/24/2025" {
			_, _ = w.Write([]byte(year))
			return
		}
		_, _ = w.Write([]byte(division))
	}))
	defer srv.Close()
	client, err := NewClient(ClientOptions{BaseURL: srv.URL, MinInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	snap, err := NewSource(client).Fetch(context.Background(), "gauchao-serie-a2", 2025, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if snap.SeasonExternalID != "24-2025" || snap.Matches[0].Phase != "Gauchão Série A2" || snap.Matches[0].RoundNumber != 3 || snap.Standings[1].GoalDifference != -2 {
		t.Fatalf("snap = %+v %+v", snap.Matches[0], snap.Standings[1])
	}
}

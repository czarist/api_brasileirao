package fafam

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
)

func TestFetchFollowsRoundLinksAndSkipsFeminino(t *testing.T) {
	const page1 = `<div>
<a class="link-corpo" href="campeonato/campeonato-amazonense-serie-a-2026/3/">Série A</a>
<a class="link-corpo" href="campeonato/amazonense-feminino-2026/1/">Feminino</a>
<div id="paginacao"><a href="campeonatos/2/">2</a></div>
</div>`
	const page2 = `<div>
<a class="link-corpo" href="campeonato/amazonense-serie-b-2022/9/">Série B</a>
</div>`
	const round3 = `<div>
<a href="campeonato/campeonato-amazonense-serie-a-2026/4/#jogos" class="seta-dir"></a>
<h1 class="titulo">1ª Fase</h1>
<h1>GRUPO A</h1>
<table class="tabela-campeonato-interna"><tr>
  <td class="coluna-time"><b>1°</b><img src="arquivos/imagens/campeonato/amazonas.jpeg"><div class="time">Amazonas F. C</div></td>
  <td class="coluna-dados">3</td><td class="coluna-dados">1</td><td class="coluna-dados">1</td><td class="coluna-dados">0</td>
  <td class="coluna-dados">0</td><td class="coluna-dados">2</td><td class="coluna-dados">1</td><td class="coluna-dados">1</td>
</tr><tr>
  <td class="coluna-time"><b>2°</b><img src="arquivos/imagens/campeonato/nacional.jpg"><div class="time">Nacional</div></td>
  <td class="coluna-dados">0</td><td class="coluna-dados">1</td><td class="coluna-dados">0</td><td class="coluna-dados">0</td>
  <td class="coluna-dados">1</td><td class="coluna-dados">1</td><td class="coluna-dados">2</td><td class="coluna-dados">-1</td>
</tr></table>
<div class="rodada">3ª RODADA</div>
<ul class="rodada-jogos"><li>
  <div class="data">SEX 23/01/2026 19:00 - Arena da Amazônia</div>
  <div class="time1"><img src="arquivos/imagens/campeonato/amazonas.jpeg"></div>
  <div class="resultado">2</div><div class="x"></div><div class="resultado">1</div>
  <div class="time2"><img src="arquivos/imagens/campeonato/nacional.jpg"></div>
</li></ul>
</div>`
	const round4 = `<div>
<h1 class="titulo">1ª Fase</h1>
<h1>GRUPO A</h1>
<table class="tabela-campeonato-interna"><tr>
  <td class="coluna-time"><b>1°</b><img src="arquivos/imagens/campeonato/amazonas.jpeg"><div class="time">Amazonas F. C</div></td>
  <td class="coluna-dados">6</td><td class="coluna-dados">2</td><td class="coluna-dados">2</td><td class="coluna-dados">0</td>
  <td class="coluna-dados">0</td><td class="coluna-dados">4</td><td class="coluna-dados">1</td><td class="coluna-dados">3</td>
</tr><tr>
  <td class="coluna-time"><b>2°</b><img src="arquivos/imagens/campeonato/nacional.jpg"><div class="time">Nacional</div></td>
  <td class="coluna-dados">0</td><td class="coluna-dados">2</td><td class="coluna-dados">0</td><td class="coluna-dados">0</td>
  <td class="coluna-dados">2</td><td class="coluna-dados">1</td><td class="coluna-dados">4</td><td class="coluna-dados">-3</td>
</tr></table>
<div class="rodada">3ª RODADA</div>
<ul class="rodada-jogos"><li>
  <div class="data">SEX 23/01/2026 19:00 - Arena da Amazônia</div>
  <div class="time1"><img src="arquivos/imagens/campeonato/amazonas.jpeg"></div>
  <div class="resultado">2</div><div class="x"></div><div class="resultado">1</div>
  <div class="time2"><img src="arquivos/imagens/campeonato/nacional.jpg"></div>
</li></ul>
</div>`
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/site/campeonatos/2/":
			_, _ = w.Write([]byte(page2))
		case "/site/campeonato/campeonato-amazonense-serie-a-2026/4/":
			_, _ = w.Write([]byte(round4))
		case "/site/campeonato/campeonato-amazonense-serie-a-2026/3/":
			_, _ = w.Write([]byte(round3))
		default:
			_, _ = w.Write([]byte(page1))
		}
	}))
	defer srv.Close()
	client, err := NewClient(ClientOptions{BaseURL: srv.URL, MinInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	source := NewSource(client)
	plans, err := source.Plan(context.Background(), Codes())
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 2 || plans[0].Code != "amazonense-serie-a" || plans[1].Code != "amazonense-serie-b" || plans[1].Year != 2022 {
		t.Fatalf("plans = %+v", plans)
	}
	snap, err := source.Fetch(context.Background(), "amazonense-serie-a", 2026, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if snap.Source != domain.SourceFAFAM || snap.SeasonExternalID != "campeonato-amazonense-serie-a-2026" || len(snap.Matches) != 1 || snap.Matches[0].RoundNumber != 3 || *snap.Matches[0].HomeScore != 2 || snap.Standings[0].Played != 2 || snap.Standings[1].GoalDifference != -3 {
		t.Fatalf("snap = %+v %+v", snap.Matches, snap.Standings)
	}
	if snap.Teams[0].State != "AM" {
		t.Fatalf("team = %+v", snap.Teams[0])
	}
	for _, path := range paths {
		if path != "/site/campeonatos/" && path != "/site/campeonatos/2/" && path != "/site/campeonato/campeonato-amazonense-serie-a-2026/3/" && path != "/site/campeonato/campeonato-amazonense-serie-a-2026/4/" {
			t.Fatalf("path = %s", path)
		}
	}
}

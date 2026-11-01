package fsfse

import (
	"strings"
	"testing"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
)

func TestParseCatalogKeepsAdultCardsOnly(t *testing.T) {
	body := `<div>
<a class="camp-card" href="https://fsf-se.com.br/campeonatos/sergipano-a1-2026/"><h3>Campeonato Sergipano A1 2026</h3></a>
<a class="camp-card" href="/campeonatos/sergipano-serie-a2-2020/"><h3>Sergipano Série A2 2020</h3></a>
<a class="camp-card" href="/campeonatos/sergipano-serie-a3-2025/"><h3>Sergipano Série A3 2025</h3></a>
<a class="camp-card" href="/campeonatos/copa-lotese-2026/"><h3>Copa Lotese 2026</h3></a>
<a class="camp-card" href="/campeonatos/sergipano-sub-20-a1-2026/"><h3>Sub-20</h3></a>
<a class="camp-card" href="/campeonatos/campeonato-feminino-2025/"><h3>Feminino</h3></a>
<a href="/campeonatos/sergipano-serie-a1-2019/">not a card</a>
</div>`
	refs, err := ParseCatalog(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 3 || refs[0].Code != "sergipano-a1" || refs[0].Year != 2026 || refs[0].Slug != "sergipano-a1-2026" || refs[1].Code != "sergipano-a2" || refs[2].Code != "sergipano-a3" {
		t.Fatalf("refs = %+v", refs)
	}
}

func TestParseSeasonReadsTheTableAndSkipsAnotherYear(t *testing.T) {
	body := `<div>
<h3 class="fase-title">Fase Classificatória</h3>
<div class="classificacao-full" data-fase-id="3729">
<table class="table-class"><tr>
  <td class="table-class__pos">1</td>
  <td><div class="table-class__clube"><img src="https://fsf-se.com.br/wp-content/uploads/escudos/20034.jpg" alt="Sergipe"><span>Sergipe</span></div></td>
  <td>9</td><td>6</td><td>2</td><td>1</td><td>13</td><td>6</td><td class="table-class__saldo">+7</td><td class="table-class__pts">20</td>
</tr><tr>
  <td class="table-class__pos">2</td>
  <td><div class="table-class__clube"><img src="/escudos/20057.jpg" alt="Itabaiana"><span>Itabaiana</span></div></td>
  <td>9</td><td>5</td><td>2</td><td>2</td><td>10</td><td>6</td><td>+4</td><td>17</td>
</tr></table>
</div>
<h3 class="fase-title">Final</h3>
<div class="classificacao-full" data-fase-id="3902">
<table class="table-class"><tr>
  <td class="table-class__pos">1</td>
  <td><div class="table-class__clube"><img src="/escudos/20034.jpg" alt="Sergipe"><span>Sergipe</span></div></td>
  <td>2</td><td>1</td><td>1</td><td>0</td><td>2</td><td>1</td><td>+1</td><td>4</td>
</tr></table>
</div>
<div class="jogos-group" data-group="f3729r1">
  <h3 class="jogos-group__title">RODADA 1</h3>
  <div class="jogos-group__date">sábado - 10 de janeiro de 2026</div>
  <article class="match-panel">
    <div class="match-panel__team"><span class="match-panel__name">Desportiva</span><img src="/escudos/62251.jpg"></div>
    <span class="match-panel__placar">0 - 3</span>
    <div class="match-panel__team"><img src="/escudos/64368.jpg"><span class="match-panel__name">Confiança</span></div>
    <div class="match-panel__meta"><span>16:00</span><span>📍 Batistão</span></div>
    <a href="https://fsf-se.com.br/jogo/50762/sumula/">Súmula</a>
  </article>
  <div class="jogos-group__date">sábado - 10 de janeiro de 2019</div>
  <article class="match-panel">
    <div class="match-panel__team"><span class="match-panel__name">Velho</span><img src="/escudos/1.jpg"></div>
    <span class="match-panel__placar">1 - 0</span>
    <div class="match-panel__team"><img src="/escudos/2.jpg"><span class="match-panel__name">Outro</span></div>
    <a href="/jogo/1/">jogo</a>
  </article>
</div>
<div class="jogos-group" data-group="f3902">
  <h3 class="jogos-group__title">FINAL</h3>
  <div class="jogos-group__date">domingo - 8 de março de 2026</div>
  <article class="match-panel">
    <div class="match-panel__team"><span class="match-panel__name">Sergipe</span><img src="/escudos/20034.jpg"></div>
    <span class="match-panel__placar">x</span>
    <div class="match-panel__team"><img src="/escudos/20057.jpg"><span class="match-panel__name">Itabaiana</span></div>
    <div class="match-panel__meta"><span>18:00</span><span>Batistão</span></div>
    <a href="/jogo/51773/arbitragem/">Arbitragem</a>
  </article>
</div>
</div>`
	loc, err := time.LoadLocation("America/Maceio")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseSeason(body, "a1", 2026, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.matches) != 2 {
		t.Fatalf("matches = %+v", parsed.matches)
	}
	match := parsed.matches[0]
	if match.ExternalID != "a1-2026-50762" || match.RoundNumber != 1 || match.Phase != "Fase Classificatória" || match.Status != domain.StatusFinished || match.Venue != "Batistão" || match.State != "SE" || *match.HomeScore != 0 || *match.AwayScore != 3 {
		t.Fatalf("match = %+v", match)
	}
	if match.Kickoff == nil || match.Kickoff.Format(time.RFC3339) != "2026-01-10T19:00:00Z" || match.HomeExternalID != "62251" || match.AwayExternalID != "64368" {
		t.Fatalf("kickoff = %v clubs %s %s", match.Kickoff, match.HomeExternalID, match.AwayExternalID)
	}
	final := parsed.matches[1]
	if final.ExternalID != "a1-2026-51773" || final.RoundNumber != 1 || final.Phase != "Final" || final.Status != domain.StatusScheduled || final.HomeScore != nil {
		t.Fatalf("final = %+v", final)
	}
	if parsed.clubs["20034"].Name != "Sergipe" {
		t.Fatalf("club = %+v", parsed.clubs["20034"])
	}
	rows := mergeStandings(parsed.tables)
	if len(rows) != 2 || rows[0].ExternalTeamID != "20034" || rows[0].GroupName != "Fase Classificatória" || rows[0].Points != 20 || rows[0].Played != 9 || rows[0].GoalDifference != 7 || rows[0].YellowCards != 0 || rows[0].WinPercentage != 0 {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestParseSeasonRejectsAClubListedTwice(t *testing.T) {
	body := `<div class="jogos-group" data-group="f1r1">
<h3 class="fase-title">Fase</h3>
<div class="classificacao-full" data-fase-id="1"></div>
<article class="match-panel">
  <div class="match-panel__team"><span class="match-panel__name">A</span><img src="/escudos/1.jpg"></div>
  <span class="match-panel__placar">0 - 0</span>
  <div class="match-panel__team"><span class="match-panel__name">A</span><img src="/escudos/1.jpg"></div>
  <a href="/jogo/9/">jogo</a>
</article>
</div>`
	loc, err := time.LoadLocation("America/Maceio")
	if err != nil {
		t.Fatal(err)
	}
	_, err = ParseSeason(body, "a1", 2026, loc)
	if err == nil || !strings.Contains(err.Error(), "twice") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseSeasonSplitsGroupBoxesAndSkipsACardWithoutAnID(t *testing.T) {
	body := `<div>
<h3 class="fase-title">Fase Classificatória</h3>
<div class="classificacao-full" data-fase-id="9">
  <div class="grupo-box"><h4 class="grupo-box__titulo">Grupo A</h4>
    <table><tr>
      <td class="table-class__pos">1</td>
      <td><div class="table-class__clube"><img src="/escudos/1.jpg"><span>A</span></div></td>
      <td>4</td><td>3</td><td>0</td><td>1</td><td>6</td><td>2</td><td>+4</td><td>9</td>
    </tr></table>
  </div>
  <div class="grupo-box"><h4 class="grupo-box__titulo">Grupo B</h4>
    <table><tr>
      <td class="table-class__pos">1</td>
      <td><div class="table-class__clube"><img src="/escudos/2.jpg"><span>B</span></div></td>
      <td>4</td><td>2</td><td>1</td><td>1</td><td>5</td><td>3</td><td>+2</td><td>7</td>
    </tr></table>
  </div>
</div>
<h3 class="fase-title">Eliminatórias</h3>
<div class="classificacao-full" data-fase-id="10">
  <div class="grupo-box"><h4 class="grupo-box__titulo">Grupo A</h4>
    <table><tr>
      <td class="table-class__pos">1</td>
      <td><div class="table-class__clube"><img src="/escudos/3.jpg"><span>C</span></div></td>
      <td>2</td><td>1</td><td>0</td><td>1</td><td>2</td><td>2</td><td>0</td><td>3</td>
    </tr></table>
  </div>
</div>
<div class="jogos-group" data-group="f9r1">
  <h3 class="jogos-group__title">RODADA 1</h3>
  <div class="jogos-group__date">sábado - 10 de janeiro de 2026</div>
  <article class="match-panel">
    <div class="match-panel__team"><span class="match-panel__name">A</span><img src="/escudos/1.jpg"></div>
    <span class="match-panel__vs">VS</span>
    <div class="match-panel__team"><img src="/escudos/2.jpg"><span class="match-panel__name">B</span></div>
  </article>
  <article class="match-panel">
    <div class="match-panel__team"><span class="match-panel__name">A</span><img src="/escudos/1.jpg"></div>
    <span class="match-panel__placar">1 - 0</span>
    <div class="match-panel__team"><img src="/escudos/2.jpg"><span class="match-panel__name">B</span></div>
    <a href="/jogo/42/">jogo</a>
  </article>
</div>
</div>`
	loc, err := time.LoadLocation("America/Maceio")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseSeason(body, "a2", 2026, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.matches) != 1 || parsed.matches[0].ExternalID != "a2-2026-42" {
		t.Fatalf("matches = %+v", parsed.matches)
	}
	rows := mergeStandings(parsed.tables)
	if len(rows) != 3 || rows[0].GroupName != "Fase Classificatória - Grupo A" || rows[1].GroupName != "Grupo B" || rows[2].GroupName != "Eliminatórias - Grupo A" {
		t.Fatalf("rows = %+v", rows)
	}
}

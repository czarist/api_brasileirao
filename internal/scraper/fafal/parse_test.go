package fafal

import (
	"strings"
	"testing"
	"time"
)

func TestParseCatalogKeepsOnlyTheAdultSerieACard(t *testing.T) {
	page := `<a href="/novo/competicoes/2026">2026</a>
var ano = "2026";
<a href="/novo/tabela?ID=1"><img src="/novo/imagens/competicoes/alagoano-feminino-2026.png"></a>
<a href="/novo/tabela?ID=2"><img src="/novo/imagens/competicoes/alagoano-sub-15-2026.png"></a>
<a href="/novo/tabela?ID=1633"><div style="height:40px"></div><img src="/novo/imagens/competicoes/alagoano-2026.png"></a>
<a href="/novo/tabela?ID=1633"><img src="/novo/imagens/competicoes/alagoano-2026.png"></a>
<a href="/novo/tabela?ID=4"><img src="/novo/imagens/competicoes/alagoano-serie-b-2026-sub-23.png"></a>`
	links, err := ParseCatalog(page, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].ID != "1633" || links[0].Code != "alagoano-serie-a" || links[0].Year != 2026 {
		t.Fatalf("%+v", links)
	}
	years, err := ParseYears(page)
	if err != nil || len(years) != 1 || years[0] != 2026 {
		t.Fatalf("%v %v", err, years)
	}
}

func TestParseCatalogReadsTextCardsAndSkipsYouth(t *testing.T) {
	page := `var ano = "2020";
<a href="/novo/tabela?ID=345"><h3>Copa Alagoas</h3></a>
<a href="/novo/tabela?ID=383"><h3>Alagoano Série A</h3></a>
<a href="/novo/tabela?ID=8112"><h3>E-Alagoano Aloo Telecom 2020</h3></a>
<a href="/novo/tabela?ID=1412">Alagoano Série A / 2025</a>`
	links, err := ParseCatalog(page, 2020)
	if err != nil || len(links) != 1 || links[0].ID != "383" || links[0].Code != codeA {
		t.Fatalf("%v %+v", err, links)
	}
	newer := `var ano = "2024";
<a href="/novo/tabela?ID=1186"><h3>Alagoano Série A 2024</h3></a>
<a href="/novo/tabela?ID=1343"><h3>Alagoano Série B 2024</h3></a>
<a href="/novo/tabela?ID=1222"><h3>Alagoano Sub-17 2024</h3></a>`
	links, err = ParseCatalog(newer, 2024)
	if err != nil || len(links) != 2 || links[0].Code != codeA || links[1].Code != codeB || links[1].ID != "1343" {
		t.Fatalf("%v %+v", err, links)
	}
	absent := `var ano = "2021";
<a href="/novo/tabela?ID=571"><h3>Copa Alagoas</h3></a>
<a href="/novo/tabela?ID=699"><h3>Alagoano Sub-23</h3></a>`
	links, err = ParseCatalog(absent, 2021)
	if err != nil || len(links) != 0 {
		t.Fatalf("%v %+v", err, links)
	}
}

func TestParseMatchesKeepsTheShootoutBesideTheScore(t *testing.T) {
	page := `<div id="bloco-rodada-1">
<h3>Seletiva da Copa do Brasil - Volta</h3>
<p>DOM, 16/08/2020 16h00 Rei Pelé</p>
<span title="Asa"><img src="escudo_clubes/10.gif"></span>
<h3>0 X 0</h3><p>(2 X 4)</p>
<span title="Murici"><img src="escudo_clubes/20.gif"></span>
<a href="/novo/jogo?id=383&jogo=7951">veja</a>
</div>`
	loc, err := maceio()
	if err != nil {
		t.Fatal(err)
	}
	matches, err := ParseMatches(page, "Seletiva da Copa do Brasil", 2020, loc)
	if err != nil || len(matches) != 1 {
		t.Fatalf("%v %+v", err, matches)
	}
	got := matches[0].Match
	if got.Phase != "Seletiva da Copa do Brasil - Volta" || got.HomeScore == nil || *got.HomeScore != 0 || got.HomePenalties == nil || *got.HomePenalties != 2 || got.AwayPenalties == nil || *got.AwayPenalties != 4 {
		t.Fatalf("%+v", got)
	}
	if got.Kickoff == nil || got.Kickoff.Format(time.RFC3339) != "2020-08-16T19:00:00Z" || got.Venue != "Rei Pelé" {
		t.Fatalf("%+v", got)
	}
}

func TestParseCatalogRejectsAnImageFromAnotherYear(t *testing.T) {
	page := `var ano = "2026";
<a href="/novo/tabela?ID=1633"><img src="/novo/imagens/competicoes/alagoano-2025.png"></a>`
	if _, err := ParseCatalog(page, 2026); err == nil {
		t.Fatal("image year was accepted")
	}
}

func TestParseMatchesReadsTheMaceioClock(t *testing.T) {
	page := `<meta property="og:title" content="Alagoano Série A / 2026" />
<a href="?ID=1633&amp;fase=3743">Primeira Fase</a>
<a href="?ID=999&fase=1">Outro</a>
<div id="bloco-rodada-1">
<h5>1ª Rodada</h5>
<p class="font-12">
<i class="fa-solid fa-calendar"></i> <span>SAB, 10/01/2026</span>
<i class="fa-solid fa-clock"></i> 16h00 <i class="fa-solid fa-map-marker"></i> Alfredo Leahy
</p>
<span class="uk-text-bold">SCP</span><span title="Penedense"><img src="../../assets/uploads/escudo_clubes/155993276982.gif"></span>
<h3>1 X 0</h3>
<span title="Cruzeiro"><img src="../../assets/uploads/escudo_clubes/1569950670100.gif"></span><span class="uk-text-bold">CRU</span>
<a href="/novo/jogo?id=1633&jogo=50416">veja como foi</a>
</div>`
	title, err := SeasonTitle(page, 2026, codeA)
	if err != nil || title != "Alagoano Série A / 2026" {
		t.Fatalf("%v %q", err, title)
	}
	phases, err := ParsePhases(page, "1633")
	if err != nil || len(phases) != 1 || phases[0].ID != "3743" || phases[0].Name != "Primeira Fase" {
		t.Fatalf("%v %+v", err, phases)
	}
	loc, err := maceio()
	if err != nil {
		t.Fatal(err)
	}
	matches, err := ParseMatches(page, "Primeira Fase", 2026, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("%+v", matches)
	}
	got := matches[0]
	if got.Match.ExternalID != "50416" || got.HomeName != "Penedense" || got.AwayName != "Cruzeiro" || got.HomeShort != "SCP" || got.AwayShort != "CRU" {
		t.Fatalf("%+v", got)
	}
	if got.Match.HomeExternalID != "155993276982" || got.Match.AwayExternalID != "1569950670100" {
		t.Fatalf("ids %s %s", got.Match.HomeExternalID, got.Match.AwayExternalID)
	}
	if got.Match.Kickoff == nil || got.Match.Kickoff.Format(time.RFC3339) != "2026-01-10T19:00:00Z" || got.Match.Venue != "Alfredo Leahy" {
		t.Fatalf("kickoff=%v venue=%q", got.Match.Kickoff, got.Match.Venue)
	}
	if got.Match.HomeScore == nil || *got.Match.HomeScore != 1 || got.Match.AwayScore == nil || *got.Match.AwayScore != 0 || got.Match.Status != "finished" {
		t.Fatalf("%+v", got.Match)
	}
}

func TestParseMatchesTreatsDefinirAsNoKickoffAndSkipsAnUnnamedSide(t *testing.T) {
	page := `<div id="bloco-rodada-2">
<p>A DEFINIR</p>
<span class="uk-text-bold">ASA</span><span title="Asa"><img src="escudo_clubes/10.gif"></span>
<h3>- X -</h3>
<span title="CSA"><img src="escudo_clubes/20.gif"></span><span class="uk-text-bold">CSA</span>
<a href="/novo/jogo?id=1&jogo=9">veja</a>
<p>SAB, 11/01/2026 15h00 Estádio</p>
<h3>0 X 0</h3>
<a href="/novo/jogo?id=1&jogo=8">veja</a>
</div>`
	loc, err := maceio()
	if err != nil {
		t.Fatal(err)
	}
	matches, err := ParseMatches(page, "Primeira Fase", 2026, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].Match.ExternalID != "9" || matches[0].Match.Kickoff != nil || matches[0].Match.Status != "scheduled" {
		t.Fatalf("%+v", matches)
	}
}

func TestParseMatchesRejectsTheSameClubOnBothSides(t *testing.T) {
	page := `<div id="bloco-rodada-1">
<p>10/01/2026 16h00 Castelão</p>
<span title="Asa"><img src="escudo_clubes/10.gif"></span>
<h3>1 X 0</h3>
<span title="Asa"><img src="escudo_clubes/10.gif"></span>
<a href="/novo/jogo?id=1&jogo=7">veja</a>
</div>`
	loc, err := maceio()
	if err != nil {
		t.Fatal(err)
	}
	_, err = ParseMatches(page, "Final", 2026, loc)
	if err == nil || !strings.Contains(err.Error(), "same club") {
		t.Fatal(err)
	}
}

func TestParseStandingsKeepsPublishedGoalDifferenceAndCardOrder(t *testing.T) {
	page := `<table><tr><td>Artilheiro</td><td>9</td></tr></table>
<table>
<thead><tr>
<th colspan="3">Classificação</th>
<th><div uk-tooltip="title: Pontos">Pts</div></th>
<th>J</th><th>V</th><th>E</th><th>D</th><th>GP</th><th>GC</th><th>SG</th><th>CA</th><th>CV</th><th>%</th>
</tr></thead>
<tr><td colspan="14">GRUPO A</td></tr>
<tr>
<td>1º</td>
<td><img src="../../assets/uploads/escudo_clubes/155993103617.gif"></td>
<td>Asa</td>
<td>17</td><td>7</td><td>5</td><td>2</td><td>0</td><td>13</td><td>3</td><td>99</td><td>15</td><td>0</td><td>80%</td>
</tr>
</table>`
	rows, clubs, err := ParseStandings(page)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Position != 1 || rows[0].Points != 17 || rows[0].GoalDifference != 99 || rows[0].YellowCards != 15 || rows[0].RedCards != 0 {
		t.Fatalf("%+v", rows)
	}
	if rows[0].GroupName != "GRUPO A" || rows[0].ExternalTeamID != "155993103617" || clubs["155993103617"].Name != "Asa" {
		t.Fatalf("%+v %+v", rows[0], clubs)
	}
}

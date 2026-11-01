package portal

import (
	"strings"
	"testing"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
)

func TestClassifyKeepsAdultDivisionsAndSkipsTheRest(t *testing.T) {
	df, _, ok := Lookup("candangao-serie-a")
	if !ok {
		t.Fatal("missing df")
	}
	checks := []struct {
		title string
		code  string
	}{
		{"Candangão BRB Série A / 2026", "candangao-serie-a"},
		{"Candangão Série B / 2025", "candangao-serie-b"},
		{"Candango SUB 11 / 2026", ""},
		{"Candango Feminino / 2026", ""},
	}
	for _, check := range checks {
		got, ok := df.Classify(check.title)
		if check.code == "" {
			if ok {
				t.Fatalf("%s classified as %s", check.title, got.Code)
			}
			continue
		}
		if !ok || got.Code != check.code {
			t.Fatalf("%s -> %v %s", check.title, ok, got.Code)
		}
	}
	goias, _, _ := Lookup("goiano-primeira-divisao")
	for _, check := range []struct{ title, code string }{
		{"Campeonato Goiano de Profissionais da 1ª Divisão / 2026", "goiano-primeira-divisao"},
		{"Campeonato Goiano de Profissionais da Divisão de Acesso (2ª Divisão) / 2026", "goiano-divisao-de-acesso"},
		{"Campeonato Goiano de Profissionais da 3ª Divisão / 2026", "goiano-terceira-divisao"},
		{"Campeonato Goiano Sub 17 da 1ª Divisão / 2026", ""},
		{"Copa Goiás Sub 11 da 1ª Divisão / 2026", ""},
	} {
		got, ok := goias.Classify(check.title)
		if check.code == "" && ok {
			t.Fatalf("%s classified as %s", check.title, got.Code)
		}
		if check.code != "" && (!ok || got.Code != check.code) {
			t.Fatalf("%s -> %v %s", check.title, ok, got.Code)
		}
	}
	pe, _, _ := Lookup("pernambucano-a1")
	for _, check := range []struct{ title, code string }{
		{"Pernambucano A1 / 2019", "pernambucano-a1"},
		{"Pernambucano A2 / 2019", "pernambucano-a2"},
		{"Pernambucano A3 / 2024", "pernambucano-a3"},
		{"Copa Pernambuco / 2019", ""},
		{"Brasileiro Série B / 2019", ""},
		{"Pernambucano Feminino / 2026", ""},
	} {
		got, ok := pe.Classify(check.title)
		if check.code == "" && ok {
			t.Fatalf("%s classified as %s", check.title, got.Code)
		}
		if check.code != "" && (!ok || got.Code != check.code) {
			t.Fatalf("%s -> %v %s", check.title, ok, got.Code)
		}
	}
}

func TestParseCatalogReadsTheCardAndTheHeading(t *testing.T) {
	site, _, _ := Lookup("candangao-serie-a")
	page := `<select><option value="2026" selected>2026</option><option value="2025">2025</option></select>
<a class="stretched-link" href="../competicoes/jogos.php?q=1626">Candangão BRB Série A / 2026</a>
<a href="jogos.php?q=1667">Candango SUB 11 / 2026</a>
<h3 class="head_background_title m-0">Pernambucano A1 / 2026</h3>
<a href="jogos.php?q=1651"><img src="x.jpg"></a>`
	selected, links, years, err := ParseCatalog(site, page)
	if err != nil {
		t.Fatal(err)
	}
	if selected != 2026 || len(years) != 2 || len(links) != 1 || links[0].ID != "1626" || links[0].Code != "candangao-serie-a" {
		t.Fatalf("selected=%d years=%v links=%+v", selected, years, links)
	}
	pe, _, _ := Lookup("pernambucano-a1")
	_, links, _, err = ParseCatalog(pe, page)
	if err != nil || len(links) != 1 || links[0].ID != "1651" || links[0].Title != "Pernambucano A1 / 2026" {
		t.Fatalf("pe links=%+v err=%v", links, err)
	}
}

func TestParseMatchAndStandingsKeepPublishedFigures(t *testing.T) {
	loc, err := saoPaulo()
	if err != nil {
		t.Fatal(err)
	}
	feed := `<div class="title-rodada">1ª Rodada</div>
<div class="pr-jogos">
<span class="cronolog-sm">SAB, 10/01 - 15:00 - Jk</span>
<img alt="Paranoá Esporte Clube" data-bs-title="Paranoá" src="../../assets/uploads/escudo_clubes/17150202178.png">
<div class="pe-2">PEC</div>
<span class="text-end">0</span><span class="versus">X</span><span class="text-start">1</span>
<img data-bs-title="ARUC" src="../../assets/uploads/escudo_clubes/170327796416.jpg">
<div class="ps-2">ARU</div>
<a href="../competicoes/jogo.php?id=1626&jogo=50017">sobre o jogo</a>
</div>`
	matches, err := ParseMatches(feed, "Primeira Fase", 2026, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("matches=%d", len(matches))
	}
	got := matches[0]
	if got.Match.ExternalID != "50017" || got.Match.Venue != "Jk" || got.Match.Status != domain.StatusFinished || got.HomeShort != "PEC" || got.AwayName != "ARUC" {
		t.Fatalf("%+v", got)
	}
	if got.Match.Kickoff == nil || got.Match.Kickoff.Format(time.RFC3339) != "2026-01-10T18:00:00Z" || *got.Match.HomeScore != 0 || *got.Match.AwayScore != 1 {
		t.Fatalf("kickoff/score %+v", got.Match)
	}
	table := `<table><thead><tr><th colspan="3">GRUPO A</th></tr></thead><tbody>
<tr>
<td>1º</td><td><img src="../../assets/uploads/escudo_clubes/170441905472.gif"></td>
<td><span class="d-none d-sm-block">Gama</span><span class="d-block d-sm-none">GAM</span></td>
<td>23</td><td>9</td><td>7</td><td>2</td><td>0</td><td>15</td><td>4</td><td>99</td><td>4</td><td>28</td><td>85</td>
</tr>
<tr>
<td>2º</td><td><img src="../../assets/uploads/escudo_clubes/17150202178.png"></td>
<td><span>Paranoá</span><span>PEC</span></td>
<td>-2*</td><td>9</td><td>0</td><td>0</td><td>9</td><td>1</td><td>10</td><td>-9</td><td>0</td><td>1</td><td>0</td>
</tr>
</tbody></table>`
	rows, clubs, err := ParseStandings(table)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].GroupName != "GRUPO A" || rows[0].Points != 23 || rows[0].GoalDifference != 99 || rows[0].RedCards != 4 || rows[0].YellowCards != 28 || rows[0].WinPercentage != 85 {
		t.Fatalf("%+v", rows)
	}
	if rows[1].Points != -2 || clubs["17150202178"].Name != "Paranoá" || clubs["170441905472"].Short != "GAM" {
		t.Fatalf("row=%+v clubs=%+v", rows[1], clubs)
	}
}

func TestParseMatchKeepsAShortNameAndAnHourClock(t *testing.T) {
	loc, err := saoPaulo()
	if err != nil {
		t.Fatal(err)
	}
	feed := `<div class="title-rodada">1ª Rodada</div>
<div class="pr-jogos">
<span class="cronolog-sm">SAB, 10/01 - 16h00 - Antonio Accioly</span>
<img alt="Atlético Goianiense Saf" src="../../assets/uploads/escudo_clubes/171905265129.jpg">
<div class="pe-2">ACG</div>
<span class="text-end">2</span><span class="text-start">0</span>
<img src="https://conteudo.cbf.com.br/clubes/20330/escudo.jpg">
<div class="ps-2">AFC</div>
<a href="jogo.php?id=1643&jogo=51021">sobre</a>
</div>`
	matches, err := ParseMatches(feed, "1ª Fase / Turno Único", 2026, loc)
	if err != nil {
		t.Fatal(err)
	}
	got := matches[0]
	plain, err := ParseMatchFeed([]byte(`{"html":""}`))
	if err != nil || plain != "" {
		t.Fatalf("empty feed %q %v", plain, err)
	}
	shield := `<div class="title-rodada">1ª Rodada</div><div class="pr-jogos"><img alt="Rio Verde" src="../../assets/uploads/sem_escudo.gif"><div class="pe-2">AEC</div><span class="text-end">1</span><span class="text-start">1</span><img alt="Jataiense" src="escudo_clubes/99.gif"><div class="ps-2">AEJ</div><a href="jogo.php?jogo=3573">sobre</a></div>`
	shielded, err := ParseMatches(shield, "1ª Fase", 2019, loc)
	if err != nil {
		t.Fatal(err)
	}
	if shielded[0].Match.HomeExternalID != "name:rio-verde" || shielded[0].HomeName != "Rio Verde" {
		t.Fatalf("shield %+v", shielded[0])
	}
	bare := `<div class="title-rodada">1ª Rodada</div><div class="pr-jogos"><img alt="Anápolis" src="escudo_clubes/7.jpg"><div class="pe-2">AFC</div><span class="text-end">2</span><span class="text-start">0</span><img src="sem_escudo.gif"><div class="ps-2">AEC</div><a href="jogo.php?jogo=3577">sobre</a></div>`
	other, err := ParseMatches(bare, "1ª Fase", 2019, loc)
	if err != nil {
		t.Fatal(err)
	}
	kept := Fixture{
		HomeName: "Goiás", HomeShort: "GEC", AwayName: "Vila Nova", AwayShort: "VNN",
		Match: domain.MatchInput{ExternalID: "1", HomeExternalID: "10", AwayExternalID: "20"},
	}
	linked := LinkClubs([]Fixture{shielded[0], other[0], kept})
	if linked[1].Match.AwayExternalID != "name:rio-verde" || linked[1].AwayName != "Rio Verde" {
		t.Fatalf("linked %+v", linked[1])
	}
	if linked[2].Match.HomeExternalID != "10" || linked[2].Match.AwayExternalID != "20" {
		t.Fatalf("badges changed %+v", linked[2].Match)
	}
	if got.AwayName != "AFC" || got.Match.AwayExternalID != "20330" || got.Match.Venue != "Antonio Accioly" {
		t.Fatalf("%+v", got)
	}
	if got.Match.Kickoff == nil || got.Match.Kickoff.Format(time.RFC3339) != "2026-01-10T19:00:00Z" {
		t.Fatalf("kickoff %+v", got.Match.Kickoff)
	}
	table := `<table><tr><th colspan="14"><span class="title-grupo">GRUPO A</span></th></tr>
<tr><th colspan="3">Classificação</th></tr>
<tr><td>1º</td><td><img src="https://conteudo.cbf.com.br/clubes/20330/escudo.jpg" alt=""></td><td><span>Náutico</span></td><td>18</td><td>7</td><td>6</td><td>0</td><td>1</td><td>10</td><td>2</td><td>8</td><td>1</td><td>4</td><td>85</td></tr></table>`
	rows, clubs, err := ParseStandings(table)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].GroupName != "GRUPO A" || rows[0].ExternalTeamID != "20330" || clubs["20330"].Name != "Náutico" || rows[0].Points != 18 {
		t.Fatalf("rows=%+v clubs=%+v", rows, clubs)
	}
	matchFeed, tableFeed, err := FeedScripts(`url: 'jogos_ajax_ver2.php'; url: 'classificacao_ajax_ver2.php';`)
	if err != nil || matchFeed != "jogos_ajax_ver2.php" || tableFeed != "classificacao_ajax_ver2.php" {
		t.Fatalf("feeds %s %s %v", matchFeed, tableFeed, err)
	}
}

func TestLinkClubsKeepsTwoClubsThatShareAShortInOneMatch(t *testing.T) {
	collision := Fixture{
		HomeName: "Rio Verde", HomeShort: "IEC",
		AwayName: "IEC", AwayShort: "IEC",
		Match: domain.MatchInput{ExternalID: "8196", HomeExternalID: "name:rio-verde", AwayExternalID: "20"},
	}
	named := Fixture{
		HomeName: "Rio Verde", HomeShort: "AEC", AwayName: "Jataiense", AwayShort: "AEJ",
		Match: domain.MatchInput{ExternalID: "3573", HomeExternalID: "name:rio-verde", AwayExternalID: "99"},
	}
	bare := Fixture{
		HomeName: "Anápolis", HomeShort: "AFC", AwayName: "AEC", AwayShort: "AEC",
		Match: domain.MatchInput{ExternalID: "3577", HomeExternalID: "7", AwayExternalID: "name:aec"},
	}
	linked := LinkClubs([]Fixture{collision, named, bare})
	if linked[0].Match.HomeExternalID != "name:rio-verde" || linked[0].Match.AwayExternalID != "20" {
		t.Fatalf("collision %+v", linked[0].Match)
	}
	if linked[2].Match.AwayExternalID != "name:rio-verde" || linked[2].AwayName != "Rio Verde" {
		t.Fatalf("cross-match link %+v", linked[2])
	}
}

func TestParseSkipsAMatchWithNoPublishedClub(t *testing.T) {
	loc, err := saoPaulo()
	if err != nil {
		t.Fatal(err)
	}
	feed := `<div class="title-rodada">1ª Rodada</div>
<div class="pr-jogos"><img alt="A" src="escudo_clubes/1.png"><div class="pe-2">A</div><span class="text-end">1</span><span class="text-start">0</span><img alt="B" src="escudo_clubes/2.png"><div class="ps-2">B</div><a href="jogo.php?jogo=1">x</a></div>
<div class="pr-jogos"><img alt="A" src="escudo_clubes/1.png"><div class="pe-2">A</div><span class="text-end">1</span><span class="text-start">2</span><img src="sem_escudo.gif"><div class="ps-2"></div><a href="jogo.php?jogo=3597">x</a></div>`
	got, err := ParseMatches(feed, "Fase", 2019, loc)
	if err != nil || len(got) != 1 || got[0].Match.ExternalID != "1" {
		t.Fatalf("%v %+v", err, got)
	}
}

func TestParseRejectsTheSameClubTwice(t *testing.T) {
	loc, err := saoPaulo()
	if err != nil {
		t.Fatal(err)
	}
	feed := `<div class="title-rodada">1ª Rodada</div><div class="pr-jogos">
<span class="cronolog-sm">SAB, 10/01 - 15:00 - Jk</span>
<img alt="Gama" src="escudo_clubes/10.png"><div class="pe-2">GAM</div>
<span class="text-end">1</span><span class="text-start">0</span>
<img alt="Gama" src="escudo_clubes/10.png"><div class="ps-2">GAM</div>
<a href="jogo.php?id=1&jogo=9">sobre</a></div>`
	if _, err := ParseMatches(feed, "Primeira Fase", 2026, loc); err == nil || !strings.Contains(err.Error(), "same club") {
		t.Fatal(err)
	}
}

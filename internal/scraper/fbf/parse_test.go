package fbf

import (
	"strings"
	"testing"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
)

func TestParseYearsIgnoresSelectedAndSkipsAnotherDivision(t *testing.T) {
	page := `<select>
<option selected value="https://www.fbf.org.br/competicoes/1/2011">2011</option>
<option selected value="https://www.fbf.org.br/competicoes/1/2025">2025</option>
<option selected value="https://www.fbf.org.br/competicoes/1/2025">2025</option>
<option selected value="https://www.fbf.org.br/competicoes/3/2025">2025</option>
<option value="https://www.fbf.org.br/competicoes/2/2026">2026</option>
</select>`
	years, err := ParseYears(page, "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(years) != 2 || years[0] != 2011 || years[1] != 2025 {
		t.Fatalf("years = %v", years)
	}
	if _, err := ParseYears(`<select><option value="https://www.fbf.org.br/competicoes/1/2025">2024</option></select>`, "1"); err == nil {
		t.Fatal("a mismatched option label was accepted")
	}
	if _, err := ParseYears(`<p>no select</p>`, "1"); err == nil {
		t.Fatal("a page without a season select was accepted")
	}
}

func TestParsePageKeepsThePublishedTableAndDoesNotReadAComment(t *testing.T) {
	loc, err := bahia()
	if err != nil {
		t.Fatal(err)
	}
	page, err := ParsePage(samplePhase(true), "Fase 1 - Classificação", 2025, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Matches) != 1 || page.Matches[0].ExternalID != "10949" {
		t.Fatalf("matches %+v", page.Matches)
	}
	match := page.Matches[0]
	if match.Status != domain.StatusFinished || match.HomeScore == nil || *match.HomeScore != 0 || *match.AwayScore != 0 {
		t.Fatalf("score %+v", match)
	}
	if match.HomePenalties != nil || match.AwayPenalties != nil {
		t.Fatalf("placeholder penalties were stored: %+v", match)
	}
	if match.Kickoff == nil || match.Kickoff.Format(time.RFC3339) != "2025-01-11T19:00:00Z" || match.Venue != "Manoel Barradas" || match.RoundNumber != 1 {
		t.Fatalf("clock %+v", match)
	}
	if len(page.Tables) != 1 || len(page.Tables[0]) != 2 {
		t.Fatalf("tables %+v", page.Tables)
	}
	row := page.Tables[0][0]
	if row.GroupName != "Grupo 1" || row.Points != 21 || row.GoalDifference != 99 || row.WinPercentage != 78 || row.YellowCards != 0 || row.RedCards != 0 {
		t.Fatalf("row %+v", row)
	}
	bahia := page.Tables[0][1]
	if bahia.WinPercentage != 67 || bahia.GoalDifference != -2 || page.Clubs[bahia.ExternalTeamID].Name != "Bahia" {
		t.Fatalf("bahia %+v %+v", bahia, page.Clubs)
	}
	if page.Clubs[row.ExternalTeamID].Short != "ECV" {
		t.Fatalf("clubs %+v", page.Clubs)
	}
}

func TestParsePageKeepsAnUndefinedFixtureUnset(t *testing.T) {
	loc, err := bahia()
	if err != nil {
		t.Fatal(err)
	}
	page, err := ParsePage(undefinedCard(), "Fase 1 - Classificação", 2027, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Matches) != 1 {
		t.Fatal(page.Matches)
	}
	match := page.Matches[0]
	if match.Status != domain.StatusScheduled || match.HomeScore != nil || match.AwayScore != nil || match.Kickoff != nil || match.Venue != "" || match.RoundNumber != 1 {
		t.Fatalf("%+v", match)
	}
	if page.Tables[0][0].Points != 0 || page.Tables[0][0].Played != 0 || page.Tables[0][0].WinPercentage != 0 {
		t.Fatalf("%+v", page.Tables)
	}
}

func TestParsePageStoresAShootoutAndKeepsAZeroOnOneSide(t *testing.T) {
	loc, err := bahia()
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ReplaceAll(samplePhase(false), "<span>0</span><span>(0)</span>", "<span>1</span><span>(4)</span>")
	body = strings.Replace(body, "<span>0</span>", "<span>1</span><span>(0)</span>", 1)
	page, err := ParsePage(body, "Fase 2 - Semifinal", 2025, loc)
	if err != nil {
		t.Fatal(err)
	}
	match := page.Matches[0]
	if match.HomeScore == nil || *match.HomeScore != 1 || *match.AwayScore != 1 {
		t.Fatalf("score %+v", match)
	}
	if match.HomePenalties == nil || *match.HomePenalties != 4 || match.AwayPenalties == nil || *match.AwayPenalties != 0 {
		t.Fatalf("penalties %+v", match)
	}
}

func TestParsePageSeparatesClubsThatShareThePlaceholderBadge(t *testing.T) {
	loc, err := bahia()
	if err != nil {
		t.Fatal(err)
	}
	page, err := ParsePage(placeholderPhase(), "Fase 1 - Classificação", 2012, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Tables) != 1 || len(page.Tables[0]) != 2 {
		t.Fatalf("tables %+v", page.Tables)
	}
	if page.Tables[0][0].ExternalTeamID != "name:ipitanga" || page.Tables[0][1].ExternalTeamID != "name:cruzeiro" {
		t.Fatalf("ids %+v %+v", page.Tables[0][0].ExternalTeamID, page.Tables[0][1].ExternalTeamID)
	}
	match := page.Matches[0]
	if match.HomeExternalID != "name:ipitanga" || match.AwayExternalID != "name:cruzeiro" {
		t.Fatalf("match clubs %s %s", match.HomeExternalID, match.AwayExternalID)
	}
	if page.Clubs["name:ipitanga"].Short != "IPI" || page.Clubs["name:cruzeiro"].Name != "Cruzeiro" {
		t.Fatalf("clubs %+v", page.Clubs)
	}
}

func TestLinkPlaceholderClubsUsesANameFromAnotherPhase(t *testing.T) {
	clubs := map[string]Club{
		"name:cruzeiro":          {Name: "Cruzeiro"},
		"name:cruzeiro-de-feira": {Name: "Cruzeiro de Feira"},
		"name:cru":               {Short: "CRU"},
		"name:gua":               {Short: "GUA"},
		"name:guanambi":          {Name: "Guanambí"},
	}
	matches := []domain.MatchInput{{
		ExternalID:     "1",
		HomeExternalID: "name:gua",
		AwayExternalID: "/images/clubes/jequie.png",
	}}
	linked, err := linkPlaceholderClubs(matches, clubs)
	if err != nil {
		t.Fatal(err)
	}
	if linked[0].HomeExternalID != "name:guanambi" || clubs["name:guanambi"].Short != "GUA" {
		t.Fatalf("%+v %+v", linked[0], clubs["name:guanambi"])
	}
	ambiguous := []domain.MatchInput{{
		ExternalID:     "2",
		HomeExternalID: "name:cru",
		AwayExternalID: "/images/clubes/jequie.png",
	}}
	if _, err := linkPlaceholderClubs(ambiguous, clubs); err == nil {
		t.Fatal("a short that matches two clubs was accepted")
	}
}

func TestParsePageReadsABareRoundNumber(t *testing.T) {
	loc, err := bahia()
	if err != nil {
		t.Fatal(err)
	}
	body := strings.Replace(samplePhase(false), "Rodada 1", "11", 1)
	page, err := ParsePage(body, "Fase 2 - Semifinais", 2025, loc)
	if err != nil {
		t.Fatal(err)
	}
	if page.Matches[0].RoundNumber != 11 {
		t.Fatalf("round %d", page.Matches[0].RoundNumber)
	}
}

func TestParsePageRejectsTheSameBadge(t *testing.T) {
	loc, err := bahia()
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ReplaceAll(samplePhase(false), "/images/clubes/barcelona-ilheus1.png", "/images/clubes/202401180423newecv.png")
	if _, err := ParsePage(body, "Fase 1 - Classificação", 2025, loc); err == nil {
		t.Fatal("the same badge on both sides was accepted")
	}
}

func TestParsePhasesKeepsPageOrderAndSkipsARepeatedID(t *testing.T) {
	page := `<ul class="fase-links">
<li><a href="https://www.fbf.org.br/competicoes/1/2025/405">Fase 1 - Classificação</a></li>
<li><a href="https://www.fbf.org.br/competicoes/1/2024/1">Fase antiga</a></li>
<li><a href="/competicoes/1/2025/409">Fase 3 - Final</a></li>
<li><a href="/competicoes/1/2025/405">Fase 1 - Classificação</a></li>
</ul>`
	phases, err := ParsePhases(page, "1", 2025)
	if err != nil {
		t.Fatal(err)
	}
	if len(phases) != 2 || phases[0].ID != "405" || phases[1].Name != "Fase 3 - Final" {
		t.Fatalf("%+v", phases)
	}
}

func samplePhase(withComment bool) string {
	comment := ""
	if withComment {
		comment = `<!-- <div class="d-flex flex-column"><a href="https://www.fbf.org.br/jogo/detalhes/99999">oculto</a></div> -->`
	}
	return comment + `
<div class="section-title-table"><h4 class="title text-black">Grupo 1</h4></div>
<table class="table"><thead><tr>
<th>Posição</th><th>P</th><th>J</th><th>V</th><th>E</th><th>D</th><th>GP</th><th>GC</th><th>SG</th><th>%</th>
</tr></thead><tbody>
<tr><td>1º <img src="/images/clubes/202401180423newecv.png"><span class="team-meta__place">Vitória</span></td>
<td>21</td><td>9</td><td>6</td><td>3</td><td>0</td><td>18</td><td>4</td><td>99</td><td>77.78</td></tr>
<tr><td>2º <img src="/images/clubes/2024041300102017_logobahia (1).png"><span class="team-meta__place">Bahia</span></td>
<td>4</td><td>2</td><td>1</td><td>1</td><td>0</td><td>3</td><td>1</td><td>-2</td><td>66.67</td></tr>
</tbody></table>
<div class="carousel-item"><div class="carousel-header"><h4 class="title text-black">Rodada 1</h4></div>
<div class="d-flex flex-column">
<div class="data-hora-card-jogo"><span>Manoel Barradas <br/>11/01/2025 16:00</span></div>
<div class="d-flex flex-confronto-card">
<div><span class="time-confronto">ECV</span><img src="/images/clubes/202401180423newecv.png"></div>
<div class="hora-confronto-card"><span>0</span><span>(0)</span><span><?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">
<svg xmlns="http://www.w3.org/2000/svg"><path d="M0 0"/></svg></span><span>0</span></div>
<div><img src="/images/clubes/barcelona-ilheus1.png"><span class="time-confronto">BAR</span></div>
</div>
<a href="https://www.fbf.org.br/jogo/detalhes/10949">DETALHES DE JOGO</a>
</div></div>`
}

func placeholderPhase() string {
	return `
<table class="table"><thead><tr>
<th>Posição</th><th>P</th><th>J</th><th>V</th><th>E</th><th>D</th><th>GP</th><th>GC</th><th>SG</th><th>%</th>
</tr></thead><tbody>
<tr><td>4º <img src="/images/clubes/default.png"><span class="team-meta__place">Ipitanga</span></td>
<td>10</td><td>8</td><td>3</td><td>1</td><td>4</td><td>8</td><td>9</td><td>-1</td><td>41.67</td></tr>
<tr><td>5º <img src="/images/clubes/default.png"><span class="team-meta__place">Cruzeiro</span></td>
<td>8</td><td>8</td><td>2</td><td>2</td><td>4</td><td>7</td><td>10</td><td>-3</td><td>33.33</td></tr>
</tbody></table>
<div class="carousel-item"><h4 class="title text-black">1 Rodada</h4>
<div class="d-flex flex-column">
<div class="data-hora-card-jogo"><span>Junqueira Ayres <br/>28/04/2012 16:00</span></div>
<div class="d-flex flex-confronto-card">
<div><span class="time-confronto">IPI</span><img src="/images/clubes/default.png"></div>
<div class="hora-confronto-card"><span>1</span><span>(0)</span><span>0</span></div>
<div><img src="/images/clubes/default.png"><span class="time-confronto">CRU</span></div>
</div>
<a href="https://www.fbf.org.br/jogo/detalhes/1688">DETALHES DE JOGO</a>
</div></div>`
}

func undefinedCard() string {
	return `
<div class="section-title-table"><h4 class="title text-black">Grupo 1</h4></div>
<table class="table"><thead><tr>
<th>Posição</th><th>P</th><th>J</th><th>V</th><th>E</th><th>D</th><th>GP</th><th>GC</th><th>SG</th><th>%</th>
</tr></thead><tbody>
<tr><td>1º <img src="/images/clubes/1647366502641.jpg"><span class="team-meta__place">Fluminense</span></td>
<td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0.00</td></tr>
<tr><td>2º <img src="/images/clubes/202511271250logo juazeirense (1)_page-0001.jpg"><span class="team-meta__place">Juazeirense</span></td>
<td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0.00</td></tr>
</tbody></table>
<div class="carousel-item"><div class="carousel-header"><h4 class="title text-black">1 Rodada</h4></div>
<div class="d-flex flex-column">
<div class="data-hora-card-jogo"><span>A definir <br/>DATA A DEFINIR</span></div>
<div class="d-flex flex-confronto-card">
<div><span class="time-confronto">FLU</span><img src="/images/clubes/1647366502641.jpg"></div>
<div class="hora-confronto-card"><span> </span><span></span><span> </span></div>
<div><img src="/images/clubes/202511271250logo juazeirense (1)_page-0001.jpg"><span class="time-confronto">JUA</span></div>
</div>
<a href="https://www.fbf.org.br/jogo/detalhes/12640">DETALHES DE JOGO</a>
</div></div>`
}

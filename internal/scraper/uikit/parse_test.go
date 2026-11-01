package uikit

import (
	"strings"
	"testing"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
)

func TestClassifyKeepsAdultDivisionsAndSkipsTheRest(t *testing.T) {
	ma, _, ok := Lookup("maranhense-serie-a")
	if !ok || ma.Host != "www.futebolmaranhense.com.br" {
		t.Fatalf("site %+v", ma)
	}
	cases := []struct {
		title string
		code  string
	}{
		{"Campeonato Maranhense Série A - 2026", "maranhense-serie-a"},
		{"Campeonato Maranhense Série B - 2020", "maranhense-serie-b"},
		{"Campeonato Maranhense Feminino - 2026", ""},
		{"Campeonato Maranhense Sub 17 - 2026", ""},
		{"Campeonato Treinamento Árbitros Súmul - 2026", ""},
	}
	for _, tc := range cases {
		got, ok := ma.Classify(tc.title)
		if tc.code == "" {
			if ok {
				t.Fatalf("%s classified as %s", tc.title, got.Code)
			}
			continue
		}
		if !ok || got.Code != tc.code {
			t.Fatalf("%s -> %+v %v", tc.title, got, ok)
		}
	}
	mt, _, _ := Lookup("mato-grossense-primeira-divisao")
	first, ok := mt.Classify("Mato-Grossense 1ª Divisão 2020 - 2020")
	second, secondOK := mt.Classify(" Mato-grossense II Divisão - 2026")
	_, cupOK := mt.Classify("Copa FMF - 2026")
	if !ok || first.Code != "mato-grossense-primeira-divisao" || !secondOK || second.Code != "mato-grossense-segunda-divisao" || cupOK {
		t.Fatalf("first=%+v %v second=%+v %v cup=%v", first, ok, second, secondOK, cupOK)
	}
}

func TestParseCatalogReadsBothCardShapes(t *testing.T) {
	ma, _, _ := Lookup("maranhense-serie-a")
	page := `<select><option value="2025">2025</option><option value="2026" selected>2026</option></select>
<a href="/competicoes/tabela/1751" title="Campeonato Maranhense Série B - 2026">b</a>
<a href="/competicoes/tabela/1655" title="Campeonato Maranhense Série A - 2026">a</a>
<a href="/competicoes/tabela/1827" title="Campeonato Maranhense Feminino - 2026">f</a>`
	selected, links, years, err := ParseCatalog(ma, page)
	if err != nil || selected != 2026 || len(years) != 2 || len(links) != 2 {
		t.Fatalf("selected=%d years=%v links=%+v err=%v", selected, years, links, err)
	}
	mt, _, _ := Lookup("mato-grossense-primeira-divisao")
	mtPage := `<select><option value="2026" selected>2026</option></select>
<a href="../competicoes/tabela.php?ID=1659" title=" Mato-grossense 1 Divisão - 2026">1</a>
<a href="../competicoes/tabela.php?ID=1817" title="Copa FMF - 2026">c</a>`
	_, mtLinks, _, err := ParseCatalog(mt, mtPage)
	if err != nil || len(mtLinks) != 1 || mtLinks[0].ID != "1659" || mtLinks[0].Code != "mato-grossense-primeira-divisao" {
		t.Fatalf("%v %+v", err, mtLinks)
	}
}

func TestParseMatchesKeepsThePublishedClockAndSkipsAnUnnamedClub(t *testing.T) {
	cuiaba, err := loadLocation("America/Cuiaba")
	if err != nil {
		t.Fatal(err)
	}
	fortaleza, err := loadLocation("America/Fortaleza")
	if err != nil {
		t.Fatal(err)
	}
	feed := `<h5 class="uk-card-title">1ª Rodada</h5>
<p class="uk-text-center">SAB, 10/01/2026 - 15h30 - Valdir Doilho Wons</p>
<span class="uk-text-bold">NMU</span><span uk-tooltip="NOVA MUTUM"><img src="https://conteudo.cbf.com.br/clubes/57118/escudo.jpg"></span>
<h4 class="uk-text-bold">1 X 1</h4>
<span uk-tooltip="SPORT SINOP"><img src="https://conteudo.cbf.com.br/clubes/59100/escudo.jpg"></span><span class="uk-text-bold">SIN</span>
<a href="../competicoes/jogo.php?id=1659&jogo=51079">veja</a>
<p>A Definir</p><span class="uk-text-bold">AAA</span><img src="sem_escudo.gif"><h4>X</h4><img src="sem_escudo.gif">
<a href="/competicoes/jogo/1655/1">veja</a>`
	got, err := ParseMatches(feed, "Primeira Fase", 2026, cuiaba)
	if err != nil || len(got) != 1 {
		t.Fatalf("%v %+v", err, got)
	}
	match := got[0].Match
	if match.ExternalID != "51079" || match.HomeExternalID != "57118" || match.AwayExternalID != "59100" || got[0].HomeName != "NOVA MUTUM" || match.Venue != "Valdir Doilho Wons" {
		t.Fatalf("%+v %+v", got[0], match)
	}
	if match.Kickoff == nil || match.Kickoff.Format(time.RFC3339) != "2026-01-10T19:30:00Z" || match.Status != domain.StatusFinished || *match.HomeScore != 1 {
		t.Fatalf("%+v", match)
	}
	zero := `<h5 class="uk-card-title">Final</h5>
<p>DOM, 22/02/2026 - 16h00 - Castelão</p>
<span uk-tooltip="Maranhão"><img src="https://conteudo.cbf.com.br/clubes/20043/escudo.jpg"></span>
<h4>0 X 0</h4>
<span uk-tooltip="Iape"><img src="https://conteudo.cbf.com.br/clubes/21894/escudo.jpg"></span>
<a href="/competicoes/jogo/1655/51796">veja</a>`
	drawn, err := ParseMatches(zero, "Final", 2026, fortaleza)
	if err != nil || drawn[0].Match.Kickoff.Format(time.RFC3339) != "2026-02-22T19:00:00Z" || *drawn[0].Match.HomeScore != 0 {
		t.Fatalf("%v %+v", err, drawn)
	}
	same := strings.Replace(zero, "21894", "20043", 1)
	if _, err := ParseMatches(same, "Final", 2026, fortaleza); err == nil || !strings.Contains(err.Error(), "same club") {
		t.Fatal(err)
	}
}

func TestParseStandingsReadsYellowBeforeRed(t *testing.T) {
	page := `<p>Classificação indisponível no momento.</p>`
	empty, clubs, err := ParseStandings(page)
	if err != nil || len(empty) != 0 || len(clubs) != 0 {
		t.Fatalf("empty %v %+v %v", err, empty, clubs)
	}
	table := `<table><thead><tr>
<th colspan="3">Classificação</th><th>Pts</th><th>J</th><th>V</th><th>E</th><th>D</th><th>GP</th><th>GC</th><th>SG</th><th>CA</th><th>CV</th><th>%</th>
</tr></thead><tbody>
<tr><td colspan="14">A</td></tr>
<tr><td>1º</td><td><img src="https://conteudo.cbf.com.br/clubes/21725/escudo.jpg"></td><td>Luverdense</td><td>17</td><td>9</td><td>5</td><td>2</td><td>2</td><td>7</td><td>3</td><td>99</td><td>21</td><td>0</td><td>62</td></tr>
</tbody></table>`
	rows, clubs, err := ParseStandings(table)
	if err != nil || len(rows) != 1 {
		t.Fatalf("%v %+v", err, rows)
	}
	row := rows[0]
	if row.ExternalTeamID != "21725" || row.GroupName != "A" || row.Points != 17 || row.GoalDifference != 99 || row.YellowCards != 21 || row.RedCards != 0 || clubs["21725"].Name != "Luverdense" {
		t.Fatalf("%+v %+v", row, clubs)
	}
}

func TestParsePhasesAndFeedStayOnTheRequestedCompetition(t *testing.T) {
	page := `<a href="/competicoes/tabela/1655/3821">Primeira Fase</a>
<a href="/competicoes/tabela/1655/3846">Final</a>
<a href="/competicoes/tabela/9999/1">Outra</a>
<a href="?ID=1659&amp;fase=3826">Primeira Fase</a>
<script>var data = {"idcampeonato": 1655, "categoria": 32, "campeonato": 256, "exercicio": 2026, "fase": 3846};</script>`
	phases, err := ParsePhases(page, "1655")
	if err != nil || len(phases) != 2 || phases[0].ID != "3821" || phases[1].Name != "Final" {
		t.Fatalf("%v %+v", err, phases)
	}
	query, err := ParseFeedQuery(page, "1655", 2026)
	if err != nil || query.CategoryID != "32" || query.Championship != "256" || query.Year != "2026" {
		t.Fatalf("%v %+v", err, query)
	}
	if _, err := ParseFeedQuery(page, "1659", 2026); err == nil {
		t.Fatal("feed for another competition was accepted")
	}
}

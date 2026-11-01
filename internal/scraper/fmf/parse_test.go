package fmf

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
)

func TestParseSampleKeepsTheClassificationAndThePublishedFigures(t *testing.T) {
	body := readFixture(t, "testdata/modulo_i_sample.html")
	seasons, err := ParseCatalog(body, "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(seasons) != 2 || seasons[0].Year != 2025 || seasons[0].ExternalID != "1779" || seasons[1].ExternalID != "1899" {
		t.Fatalf("catalog = %+v", seasons)
	}
	page, err := ParsePage(body, "1")
	if err != nil {
		t.Fatal(err)
	}
	if page.Season.Year != 2026 || page.Season.ExternalID != "1899" || page.Season.Name != "MÓDULO I - 2026" {
		t.Fatalf("season = %+v", page.Season)
	}
	if len(page.Matches) != 2 || len(page.Standings) != 4 || len(page.Teams) != 4 {
		t.Fatalf("matches=%d standings=%d teams=%d", len(page.Matches), len(page.Standings), len(page.Teams))
	}
	scored := page.Matches[0]
	if scored.ExternalID != "44225" || scored.RoundNumber != 8 || scored.Phase != "CLASSIFICATÓRIA" || scored.Status != domain.StatusFinished {
		t.Fatalf("scored = %+v", scored)
	}
	if scored.HomeScore == nil || *scored.HomeScore != 0 || scored.AwayScore == nil || *scored.AwayScore != 2 {
		t.Fatalf("score = %v-%v", scored.HomeScore, scored.AwayScore)
	}
	if scored.Kickoff == nil || scored.Kickoff.Format(time.RFC3339) != "2026-02-14T22:00:00Z" {
		t.Fatalf("kickoff = %v", scored.Kickoff)
	}
	if scored.Venue != `Governador Magalhães Pinto "Mineirão"` || scored.City != "BELO HORIZONTE" {
		t.Fatalf("place = %q / %q", scored.Venue, scored.City)
	}
	open := page.Matches[1]
	if open.ExternalID != "44100" || open.Status != domain.StatusScheduled || open.HomeScore != nil || open.AwayScore != nil {
		t.Fatalf("open = %+v", open)
	}
	if open.Kickoff == nil || open.Kickoff.Format(time.RFC3339) != "2026-01-10T19:00:00Z" {
		t.Fatalf("open kickoff = %v", open.Kickoff)
	}
	if open.HomeExternalID != "16052" || open.AwayExternalID != "16053" {
		t.Fatalf("open clubs = %s vs %s", open.HomeExternalID, open.AwayExternalID)
	}
	var america domain.TeamInput
	for _, team := range page.Teams {
		if team.ExternalID == "16052" {
			america = team
		}
	}
	if america.Name != "AMERICA SAF" {
		t.Fatalf("joined name = %q", america.Name)
	}
	var row domain.StandingInput
	for _, item := range page.Standings {
		if item.ExternalTeamID == "16052" {
			row = item
		}
		if item.GroupName == "D" {
			t.Fatal("later phase table was stored")
		}
	}
	if row.GroupName != "A" || row.GroupOrder != 0 || row.Position != 2 || row.GoalDifference != 3 || row.GoalsFor != 10 || row.GoalsAgainst != 6 || row.YellowCards != 0 {
		t.Fatalf("america row = %+v", row)
	}
}

func TestParseRejectsADisagreeingDuplicateAndTheSameClubTwice(t *testing.T) {
	body := readFixture(t, "testdata/modulo_i_sample.html")
	parts := bytes.Split(body, []byte(`data-jogo="44225"`))
	if len(parts) != 3 {
		t.Fatalf("copies = %d", len(parts)-1)
	}
	parts[1] = bytes.Replace(parts[1], []byte("<td>0</td><td>X</td><td>2</td>"), []byte("<td>1</td><td>X</td><td>2</td>"), 1)
	if _, err := ParsePage(bytes.Join(parts, []byte(`data-jogo="44225"`)), "1"); err == nil {
		t.Fatal("disagreeing duplicate was accepted")
	}
	same := bytes.ReplaceAll(body, []byte("POUSO ALEGRE SAF"), []byte("UBERLANDIA S.A.F"))
	same = bytes.ReplaceAll(same, []byte("Foto_Logo_16283"), []byte("Foto_Logo_16380"))
	if _, err := ParsePage(same, "1"); err == nil {
		t.Fatal("same club on both sides was accepted")
	}
}

func TestParseKeepsAPublishedPointsDeduction(t *testing.T) {
	body := []byte(`<html><body>
<select><option selected="selected" value="1906">M&#211;DULO II - 2026</option></select>
<div id="tab_1-1"><a href="#fase_1-1">CLASSIFICATÓRIA</a>
<div id="fase_1-1"><h3 class="box-title_es">RODADA 1</h3><b>Sábado, 10 de janeiro de 2026</b>
<div class='col-md-1'>16:00</div><div align='right'>IPATINGA - SAF</div><img src="https://esumula.fmf.com.br/escudos/Foto_Logo_16186.png"><table width='100%'><tr><td>1</td><td>X</td><td>0</td></tr></table><img src="https://esumula.fmf.com.br/escudos/uberaba.png"><div align='left'>UBERABA</div><div>Jogo 1<br>Estadio<br>IPATINGA</div><a data-jogo="1"></a>
</div></div>
<div id="tab_2-2"><table>
<tr><td>#</td><td></td><td>Equipe</td><td>PG</td><td>J</td><td>V</td><td>E</td><td>D</td><td>GP</td><td>GC</td><td>SG</td><td>%</td></tr>
<tr><td>1</td><td></td><td>IPATINGA - SAF</td><td><b>-2*</b></td><td>10</td><td>1</td><td>1</td><td>8</td><td>6</td><td>33</td><td>-27</td><td>-7</td></tr>
<tr><td>2</td><td></td><td>UBERABA</td><td>1</td><td>2</td><td>0</td><td>1</td><td>1</td><td>1</td><td>2</td><td>-1</td><td>17</td></tr>
<tr><td>3</td><td></td><td>PASSOS</td><td><b>1**</b></td><td>12</td><td>1</td><td>1</td><td>10</td><td>10</td><td>37</td><td>-27</td><td>3</td></tr>
</table></div>
</body></html>`)
	page, err := ParsePage(body, "2")
	if err != nil {
		t.Fatal(err)
	}
	if page.Standings[0].Points != -2 || page.Standings[0].WinPercentage != -7 || page.Standings[0].GoalDifference != -27 {
		t.Fatalf("row = %+v", page.Standings[0])
	}
	if page.Standings[2].Points != 1 || page.Standings[2].ExternalTeamID != "name:passos" {
		t.Fatalf("passos = %+v", page.Standings[2])
	}
	if page.Matches[0].AwayExternalID != "badge:uberaba" {
		t.Fatalf("away id = %s", page.Matches[0].AwayExternalID)
	}
}

func TestParseUsesTheMapCodeWhenThePagePublishesNoMatchId(t *testing.T) {
	body := []byte(`<html><body>
<select><option selected="selected" value="279">M&#211;DULO I - 2013</option></select>
<div id="tab_1-1"><a href="#fase_1-1">Primeira Fase</a>
<div id="fase_1-1"><h3 class="box-title_es">RODADA 11</h3><b>Sábado, 5 de maio de 2013</b>
<div class='col-md-1'>16:00</div><div align='right'>TUPI</div><img src="https://esumula.fmf.com.br/escudos/Tupi.png"><table width='100%'><tr><td>0</td><td>X</td><td>2</td></tr></table><img src="https://esumula.fmf.com.br/escudos/Foto_Logo_7250.png"><div align='left'>CRUZEIRO</div><div>Jogo <br>Radialista Mário Helênio<br>JUIZ DE FORA</div><img onclick="window.open('../GoogleMaps.aspx?cod=848A9B5D8E5A0B1B770B262573503E66','WindowGoogle','width=680');">
</div></div>
<div id="tab_2-2"><table>
<tr><td>#</td><td></td><td>Equipe</td><td>PG</td><td>J</td><td>V</td><td>E</td><td>D</td><td>GP</td><td>GC</td><td>SG</td><td>%</td></tr>
<tr><td>1</td><td></td><td>CRUZEIRO</td><td>3</td><td>1</td><td>1</td><td>0</td><td>0</td><td>2</td><td>0</td><td>2</td><td>100</td></tr>
<tr><td>2</td><td></td><td>TUPI</td><td>0</td><td>1</td><td>0</td><td>0</td><td>1</td><td>0</td><td>2</td><td>-2</td><td>0</td></tr>
</table></div></body></html>`)
	page, err := ParsePage(body, "1")
	if err != nil {
		t.Fatal(err)
	}
	match := page.Matches[0]
	if match.ExternalID != "848a9b5d8e5a0b1b770b262573503e66" || match.MatchNumber != 0 || match.Phase != "Primeira Fase" || match.HomeExternalID != "badge:Tupi" {
		t.Fatalf("match = %+v", match)
	}
	if match.Kickoff == nil || match.Kickoff.Format(time.RFC3339) != "2013-05-05T19:00:00Z" {
		t.Fatalf("kickoff = %v", match.Kickoff)
	}
}

func TestParseCatalogDecodesTheSegundaDivisaoLabel(t *testing.T) {
	body := []byte(`<option selected="selected" value="1972">SEGUNDA DIVIS&#195;O - 2026</option>`)
	seasons, err := ParseCatalog(body, "3")
	if err != nil {
		t.Fatal(err)
	}
	if len(seasons) != 1 || seasons[0].Year != 2026 || seasons[0].Name != "SEGUNDA DIVISÃO - 2026" {
		t.Fatalf("seasons = %+v", seasons)
	}
	if _, err := ParseCatalog([]byte(`<html></html>`), "1"); err == nil {
		t.Fatal("empty catalog was accepted")
	}
}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

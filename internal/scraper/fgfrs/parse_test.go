package fgfrs

import (
	"strings"
	"testing"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
)

func TestParseYearsKeepsThisDivisionOnly(t *testing.T) {
	body := `<select>
<option value="http://fgf.com.br/competicoes/profissional/23/2026">2026</option>
<option value="http://fgf.com.br/competicoes/profissional/23/2017">2017</option>
<option value="http://fgf.com.br/competicoes/profissional/26/2026">2026</option>
<option value="http://fgf.com.br/competicoes/profissional/23/2026">2026</option>
</select>`
	years, err := ParseYears(body, "23")
	if err != nil {
		t.Fatal(err)
	}
	if len(years) != 2 || years[0] != 2026 || years[1] != 2017 {
		t.Fatalf("years = %v", years)
	}
}

func TestParsePhasesSkipsCopaAndKeepsTheAdultButtons(t *testing.T) {
	body := `<div>
<a class="uk-button menu_comp" href="http://fgf.com.br/competicoes/profissional/23/2026/3746">Classificatória</a>
<a class="menu_comp" href="http://fgf.com.br/competicoes/profissional/23/2026/3857">Troféu Farroupilha</a>
<a class="menu_comp" href="http://fgf.com.br/competicoes/profissional/23/2026/3879">Final</a>
<a class="menu_comp" href="http://fgf.com.br/competicoes/profissional/26/2026/1">Copa FGF</a>
<a class="menu_comp" href="http://fgf.com.br/competicoes/profissional/23/2026/9">Recopa Gaúcha</a>
</div>`
	refs, err := ParsePhases(body, "23", "2026")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 3 || refs[0].ID != "3746" || refs[0].Name != "Classificatória" || refs[2].Name != "Final" {
		t.Fatalf("phases = %+v", refs)
	}
}

func TestParsePhaseReadsTheGroupCardAndTheTable(t *testing.T) {
	body := `<div>
<h4>RODADA 1</h4>
<div class="carousel-conteudo">
  <div class="conteudo-descricao"><div>Sáb, 10/01 16:00  - Alfredo Jaconi</div><div>1 alteração</div></div>
  <div class="mandante"><img src="https://www.fgf.com.br/public/uploads/clubes/16172238736064e0c16c05f.jpg" title="Esporte Clube Juventude"></div>
  <div class="contra"><div>1 X 1</div><a href="https://fgf.com.br/jogo/juventudexypiranga-gauchao-10-01-2026">Sobre o Jogo</a></div>
  <div class="visitante"><img src="https://www.fgf.com.br/public/uploads/clubes/32-66c62f443ceac.png" title="Ypiranga Futebol Clube"></div>
</div>
<div class="carousel-conteudo">
  <div class="conteudo-descricao"><div>Sáb, 10/01 - Estádio</div></div>
  <div class="mandante"><img src="https://www.fgf.com.br/public/uploads/clubes/Guarany_Bage_RS.jpg" title="Guarany Futebol Clube (Bagé)"></div>
  <div class="contra"><div>x</div><a href="https://fgf.com.br/jogo/guarany-agendado">Sobre o Jogo</a></div>
  <div class="visitante"><img src="https://www.fgf.com.br/public/uploads/clubes/188-6a1879257d1d4.png" title="Monsoon FC"></div>
</div>
<div class="titulo-grupo"><h4>GRUPO A</h4></div>
<div class="tabela"><table><tbody>
<tr>
  <td class="posicao-time"><b>1º</b><img src="https://www.fgf.com.br/public/uploads/clubes/14-65cfaff758870.png" title="Internacional">Internacional</td>
  <td class="posicao-time2"><b>1º</b>INT</td>
  <td title="Pontos Ganhos">15</td><td title="Jogos">6</td><td title="Vitórias">5</td><td title="Empates">0</td><td title="Derrotas">1</td>
  <td title="Gols Pró">14</td><td title="Gols Contra">5</td><td title="Saldo de Gols">9</td>
  <td title="Cartões Amarelos">17</td><td title="Cartões Vermelhos">0</td><td title="Aproveitamento">83</td>
</tr>
<tr>
  <td class="posicao-time"><b>2º</b><img src="https://www.fgf.com.br/public/uploads/clubes/16172238736064e0c16c05f.jpg" title="Juventude"></td>
  <td class="posicao-time2"><b>2º</b>JUV</td>
  <td title="Pontos Ganhos">12</td><td title="Jogos">6</td><td title="Vitórias">3</td><td title="Empates">3</td><td title="Derrotas">0</td>
  <td title="Gols Pró">8</td><td title="Gols Contra">3</td><td title="Saldo de Gols">5</td>
  <td title="Cartões Amarelos">4</td><td title="Cartões Vermelhos">1</td><td title="Aproveitamento">56.7%</td>
</tr>
</tbody></table></div>
<div class="titulo-grupo"><h4>Classificação Geral</h4></div>
<div class="tabela"><table><tbody></tbody></table></div>
</div>`
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	page, err := ParsePhase(body, "23", 2026, "Classificatória", loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.matches) != 2 || len(page.tables) != 1 || len(page.tables[0]) != 2 {
		t.Fatalf("matches %d tables %d", len(page.matches), len(page.tables))
	}
	match := page.matches[0]
	if match.ExternalID != "23-2026-juventudexypiranga-gauchao-10-01-2026" || match.RoundNumber != 1 || match.Phase != "Classificatória" || match.Status != domain.StatusFinished || match.Venue != "Alfredo Jaconi" || match.State != "RS" {
		t.Fatalf("match = %+v", match)
	}
	if match.Kickoff == nil || match.Kickoff.Format(time.RFC3339) != "2026-01-10T19:00:00Z" || *match.HomeScore != 1 || *match.AwayScore != 1 {
		t.Fatalf("kickoff/score = %v %v %v", match.Kickoff, match.HomeScore, match.AwayScore)
	}
	if match.HomeExternalID != "16172238736064e0c16c05f" || match.AwayExternalID != "32-66c62f443ceac" {
		t.Fatalf("clubs = %s %s", match.HomeExternalID, match.AwayExternalID)
	}
	open := page.matches[1]
	if open.Status != domain.StatusScheduled || open.HomeScore != nil || open.Kickoff != nil || open.HomeExternalID != "guarany_bage_rs" {
		t.Fatalf("open = %+v", open)
	}
	row := page.tables[0][0]
	if row.ExternalTeamID != "14-65cfaff758870" || row.GroupName != "GRUPO A" || row.Position != 1 || row.Points != 15 || row.Played != 6 || row.GoalDifference != 9 || row.YellowCards != 17 || row.RedCards != 0 || row.WinPercentage != 83 {
		t.Fatalf("row = %+v", row)
	}
	if page.clubs["14-65cfaff758870"].Short != "INT" || page.clubs["16172238736064e0c16c05f"].Name != "Juventude" {
		t.Fatalf("clubs = %+v", page.clubs)
	}
	if page.tables[0][1].WinPercentage != 57 {
		t.Fatalf("percent = %d", page.tables[0][1].WinPercentage)
	}
}

func TestParsePhaseReadsAKnockoutWithoutARoundLabel(t *testing.T) {
	body := `<div class="bordas2">
<div class="conteudo-descricao2"><div>Dom, 01/03 18:00  - Arena do Grêmio</div><div>1 alteração</div></div>
<div class="mandante"><img src="https://www.fgf.com.br/public/uploads/clubes/13-65527e3fbb461.png" title="Grêmio"></div>
<div class="contra2">3 X 0</div>
<div class="visitante"><img src="https://www.fgf.com.br/public/uploads/clubes/14-65cfaff758870.png" title="Internacional"></div>
<a href="https://fgf.com.br/jogo/gremioxinternacional-gauchao-01-03-2026">Sobre o jogo</a>
</div>`
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	page, err := ParsePhase(body, "23", 2026, "Final", loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.matches) != 1 {
		t.Fatalf("matches = %d", len(page.matches))
	}
	match := page.matches[0]
	if match.ExternalID != "23-2026-gremioxinternacional-gauchao-01-03-2026" || match.RoundNumber != 1 || match.Phase != "Final" || match.Venue != "Arena do Grêmio" || *match.HomeScore != 3 || *match.AwayScore != 0 {
		t.Fatalf("match = %+v", match)
	}
	if match.Kickoff == nil || match.Kickoff.Format(time.RFC3339) != "2026-03-01T21:00:00Z" {
		t.Fatalf("kickoff = %v", match.Kickoff)
	}
}

func TestParsePhaseKeepsARowWhosePositionCellIsBlank(t *testing.T) {
	body := `<div class="titulo-grupo"><h4>GRUPO A</h4></div>
<div class="tabela"><table>
<tr>
  <td class="posicao-time"><b>1º</b><img src="/clubes/a.png" title="Avenida"></td>
  <td class="posicao-time2">AVE</td>
  <td title="Pontos Ganhos">1</td><td title="Jogos">1</td><td title="Vitórias">0</td><td title="Empates">1</td><td title="Derrotas">0</td>
  <td title="Gols Pró">0</td><td title="Gols Contra">0</td><td title="Saldo de Gols">0</td>
  <td title="Cartões Amarelos">0</td><td title="Cartões Vermelhos">0</td><td title="Aproveitamento">33</td>
</tr>
<tr>
  <td class="posicao-time"><b></b><img src="/clubes/14.png" title="Internacional"></td>
  <td class="posicao-time2">INT</td>
  <td title="Pontos Ganhos">1</td><td title="Jogos">1</td><td title="Vitórias">0</td><td title="Empates">1</td><td title="Derrotas">0</td>
  <td title="Gols Pró">0</td><td title="Gols Contra">0</td><td title="Saldo de Gols">0</td>
  <td title="Cartões Amarelos">0</td><td title="Cartões Vermelhos">0</td><td title="Aproveitamento">33</td>
</tr>
</table></div>`
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	page, err := ParsePhase(body, "23", 2018, "Classificatória", loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.tables) != 1 || len(page.tables[0]) != 2 || page.tables[0][1].Position != 2 || page.tables[0][1].ExternalTeamID != "14" {
		t.Fatalf("rows = %+v", page.tables)
	}
}

func TestParsePhaseSkipsARowWithNoPublishedStats(t *testing.T) {
	body := `<div class="titulo-grupo"><h4>GRUPO A</h4></div>
<div class="tabela"><table>
<tr>
  <td class="posicao-time"><b>15º</b><img src="/clubes/real.jpg" title="Real Sc"></td>
  <td class="posicao-time2">RSC</td>
  <td title="Pontos Ganhos">3</td><td title="Jogos">14</td><td title="Vitórias">0</td><td title="Empates">3</td><td title="Derrotas">11</td>
  <td title="Gols Pró">4</td><td title="Gols Contra">23</td><td title="Saldo de Gols">-19</td>
  <td title="Cartões Amarelos">40</td><td title="Cartões Vermelhos">4</td><td title="Aproveitamento">7</td>
</tr>
<tr>
  <td class="posicao-time"><b>16º</b><img src="/clubes/logo.png" title="Futebol Com Vida *"></td>
  <td class="posicao-time2">FCV *</td>
  <td title="Pontos Ganhos"></td><td title="Jogos"></td><td title="Vitórias"></td><td title="Empates"></td><td title="Derrotas"></td>
  <td title="Gols Pró"></td><td title="Gols Contra"></td><td title="Saldo de Gols"></td>
  <td title="Cartões Amarelos"></td><td title="Cartões Vermelhos"></td><td title="Aproveitamento"></td>
</tr>
</table></div>`
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	page, err := ParsePhase(body, "24", 2025, "Classificatória", loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.tables) != 1 || len(page.tables[0]) != 1 || page.tables[0][0].ExternalTeamID != "real" || page.tables[0][0].GoalDifference != -19 {
		t.Fatalf("rows = %+v", page.tables)
	}
	if _, ok := page.clubs["logo"]; ok {
		t.Fatal("empty row was stored")
	}
}

func TestParsePhaseRejectsAClubListedTwice(t *testing.T) {
	body := `<div class="carousel-conteudo">
<div class="mandante"><img src="/clubes/1.png" title="A"></div>
<div class="contra">0 X 0</div>
<div class="visitante"><img src="/clubes/1.png" title="A"></div>
<a href="/jogo/same">Sobre o jogo</a>
</div>`
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	_, err = ParsePhase(body, "23", 2026, "Classificatória", loc)
	if err == nil || !strings.Contains(err.Error(), "twice") {
		t.Fatalf("err = %v", err)
	}
}

func TestParsePhaseSkipsASlugFromAnotherYearAndKeeps1900(t *testing.T) {
	body := `<div>
<div class="carousel-conteudo">
  <div class="conteudo-descricao"><div>30/03 15:00 - Estádio</div></div>
  <div class="mandante"><img src="/clubes/a.png" title="Guarany"></div>
  <div class="contra">1 X 0</div>
  <div class="visitante"><img src="/clubes/b.png" title="São Paulo"></div>
  <a href="/jogo/guaranyxsc12horas-segundadivisao-30-03-2019">Sobre o jogo</a>
</div>
<div class="carousel-conteudo">
  <div class="conteudo-descricao"><div>01/01 00:00 - Estádio</div></div>
  <div class="mandante"><img src="/clubes/c.png" title="Internacional SM"></div>
  <div class="contra">0 X 0</div>
  <div class="visitante"><img src="/clubes/d.png" title="Monsoon"></div>
  <a href="/jogo/internacionalsmxmonsoonfc-gauchao-01-01-1900">Sobre o jogo</a>
</div>
<div class="carousel-conteudo">
  <div class="mandante"><img src="/clubes/e.png" title="Avenida"></div>
  <div class="contra">x</div>
  <div class="visitante"><img src="/clubes/f.png" title="Guarany FC"></div>
  <a href="/jogo/avenidaxguaranyfc-gauchao-">Sobre o jogo</a>
</div>
</div>`
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	page, err := ParsePhase(body, "23", 2020, "Classificatória", loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.matches) != 2 {
		t.Fatalf("matches = %+v", page.matches)
	}
	if page.matches[0].ExternalID != "23-2020-internacionalsmxmonsoonfc-gauchao-01-01-1900" || page.matches[1].ExternalID != "23-2020-avenidaxguaranyfc-gauchao-" {
		t.Fatalf("ids = %s %s", page.matches[0].ExternalID, page.matches[1].ExternalID)
	}
}

func TestForeignYearOnlyDetectsAPageThatRepeatsAnotherSeason(t *testing.T) {
	body := `<div>
<a class="uk-active uk-button menu_comp" href="http://fgf.com.br/competicoes/profissional/25/2019/138">Classificatória</a>
<a class="menu_comp" href="http://fgf.com.br/competicoes/profissional/25/2019/242">Quartas</a>
<a class="menu_comp" href="http://fgf.com.br/competicoes/profissional/26/2019/1">Copa FGF</a>
</div>`
	foreign, err := foreignYearOnly(body, "25", "2020")
	if err != nil {
		t.Fatal(err)
	}
	if !foreign {
		t.Fatal("2020 page with only 2019 phases was treated as this season")
	}
	same, err := foreignYearOnly(body, "25", "2019")
	if err != nil {
		t.Fatal(err)
	}
	if same {
		t.Fatal("2019 page was treated as another year")
	}
}

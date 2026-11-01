package fafam

import (
	"testing"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
)

func TestParseCatalogKeepsAdultCardsAndThePreferredSlug(t *testing.T) {
	body := `<ul>
<a class="link-corpo" href="campeonato/campeonato-amazonense-serie-a-2026/3/">Campeonato Amazonense Série A 2026</a>
<a class="link-corpo" href="campeonato/amazonense-serie-b-2026/7/">Amazonense Série B 2026</a>
<a class="link-corpo" href="campeonato/amazonense-feminino-2026/3/">Feminino</a>
<a class="link-corpo" href="campeonato_sub20/amazonense-sub-20-2026/8/">Sub-20</a>
<a class="link-corpo" href="campeonato/amazonense-serie-b-2017/7/">Série B</a>
<a class="link-corpo" href="campeonato/campeonato-amazonense-serie-b-2017/3/">Série B outra</a>
<a class="link-corpo" href="campeonato/amazonense-2020/3/">Amazonense 2020</a>
<a class="link-corpo" href="campeonato/campeonato-amazonense-2020/3/">Campeonato Amazonense 2020</a>
</ul>`
	refs, err := ParseCatalog(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 4 {
		t.Fatalf("refs = %+v", refs)
	}
	if refs[0].Code != "amazonense-serie-a" || refs[0].Slug != "campeonato-amazonense-serie-a-2026" || refs[0].Round != "3" {
		t.Fatalf("a = %+v", refs[0])
	}
	var b2017, plain SeasonRef
	for _, ref := range refs {
		if ref.Year == 2017 {
			b2017 = ref
		}
		if ref.Year == 2020 {
			plain = ref
		}
	}
	if b2017.Slug != "amazonense-serie-b-2017" || plain.Slug != "campeonato-amazonense-2020" || plain.Code != "amazonense" {
		t.Fatalf("b = %+v plain = %+v", b2017, plain)
	}
}

func TestParseRoundReadsGoalsPenaltiesAndSkipsAnUnknownBadge(t *testing.T) {
	body := `<div>
<h1 class="titulo">Final</h1>
<div class="container-jogo-final">
  <div class="rodada-titulo"><div class="rodada">Final</div></div>
  <ul class="rodada-jogo-final"><li>
    <div class="data">DOM 08/02/2026 17:00 - Arena da Amazônia</div>
    <div class="time1"><img src="arquivos/imagens/campeonato/amazonas.jpeg"></div>
    <div class="resultado">0-5</div><div class="x"></div><div class="resultado">0-3</div>
    <div class="time2"><img src="arquivos/imagens/campeonato/nacional.jpg"></div>
  </li></ul>
</div>
<h1 class="titulo">1ª Fase</h1>
<h1>GRUPO A</h1>
<table class="tabela-campeonato-interna"><tr>
  <td class="coluna-time"><b>1°</b><img src="arquivos/imagens/campeonato/amazonas.jpeg"><div class="time">Amazonas F. C</div></td>
  <td class="coluna-dados"><b>8</b></td><td class="coluna-dados">4</td><td class="coluna-dados">2</td><td class="coluna-dados">2</td>
  <td class="coluna-dados">0</td><td class="coluna-dados">6</td><td class="coluna-dados">2</td><td class="coluna-dados">4</td>
</tr><tr>
  <td class="coluna-time"><b>2°</b><img src="arquivos/imagens/campeonato/nacional.jpg"><div class="time">Nacional - (Campeão)</div></td>
  <td class="coluna-dados">7</td><td class="coluna-dados">4</td><td class="coluna-dados">2</td><td class="coluna-dados">1</td>
  <td class="coluna-dados">1</td><td class="coluna-dados">6</td><td class="coluna-dados">3</td><td class="coluna-dados">3</td>
</tr></table>
<div class="rodada-titulo"><div class="rodada">3ª RODADA</div></div>
<ul class="rodada-jogos"><li>
  <div class="data">SEX 23/01/2026 19:00 - Arena da Amazônia</div>
  <div class="time1"><img src="arquivos/imagens/campeonato/amazonas.jpeg"></div>
  <div class="resultado">2</div><div class="x"></div><div class="resultado">1</div>
  <div class="time2"><img src="arquivos/imagens/campeonato/manaus.png"></div>
</li><li>
  <div class="data">SÁB 21/03/2019 15:30 - Estádio</div>
  <div class="time1"><img src="arquivos/imagens/campeonato/amazonas.jpeg"></div>
  <div class="resultado">1</div><div class="x"></div><div class="resultado">0</div>
  <div class="time2"><img src="arquivos/imagens/campeonato/nacional.jpg"></div>
</li></ul>
</div>`
	loc, err := time.LoadLocation("America/Manaus")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseRound(body, "a", 2026, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.matches) != 1 || len(parsed.tables) != 1 || len(parsed.tables[0]) != 2 {
		t.Fatalf("matches %d tables %+v", len(parsed.matches), parsed.tables)
	}
	match := parsed.matches[0]
	if match.ExternalID != "a-2026-08022026-amazonasfc-nacional" || match.Phase != "Final" || match.RoundNumber != 1 || match.Venue != "Arena da Amazônia" || match.Status != domain.StatusFinished || *match.HomeScore != 0 || *match.AwayScore != 0 || *match.HomePenalties != 5 || *match.AwayPenalties != 3 {
		t.Fatalf("final = %+v pens %v %v", match, match.HomePenalties, match.AwayPenalties)
	}
	if match.Kickoff == nil || match.Kickoff.Format(time.RFC3339) != "2026-02-08T21:00:00Z" {
		t.Fatalf("kickoff = %v", match.Kickoff)
	}
	if parsed.clubs["nacional"].Name != "Nacional" || parsed.tables[0][0].Points != 8 || parsed.tables[0][0].Played != 4 || parsed.tables[0][0].GoalDifference != 4 || parsed.tables[0][0].YellowCards != 0 || parsed.tables[0][0].GroupName != "GRUPO A" {
		t.Fatalf("table = %+v club = %+v", parsed.tables[0][0], parsed.clubs["nacional"])
	}
}

func TestParseRoundKeepsATableWithoutBadgesAndAGroupBox(t *testing.T) {
	body := `<div>
<h1 class="titulo">CLASSIFICAÇÃO GERAL</h1>
<table class="tabela-campeonato-interna"><tr>
  <td class="coluna-time"><b>1°</b><div class="time">Manaus (Campeão)</div></td>
  <td class="coluna-dados"><b>12</b></td><td class="coluna-dados">7</td><td class="coluna-dados">3</td><td class="coluna-dados">3</td>
  <td class="coluna-dados">1</td><td class="coluna-dados">11</td><td class="coluna-dados">8</td><td class="coluna-dados">3</td>
</tr><tr>
  <td class="coluna-time"><b>2°</b><div class="time">Fast Clube (Vice-Campeão)</div></td>
  <td class="coluna-dados"><b>6</b></td><td class="coluna-dados">7</td><td class="coluna-dados">2</td><td class="coluna-dados">0</td>
  <td class="coluna-dados">5</td><td class="coluna-dados">4</td><td class="coluna-dados">11</td><td class="coluna-dados">-7</td>
</tr><tr>
  <td class="coluna-time"><b>3°</b><div class="time">São Raimundo **</div></td>
  <td class="coluna-dados">4</td><td class="coluna-dados">7</td><td class="coluna-dados">1</td><td class="coluna-dados">1</td>
  <td class="coluna-dados">5</td><td class="coluna-dados">3</td><td class="coluna-dados">8</td><td class="coluna-dados">-5</td>
</tr></table>
<h1 class="titulo">1ª Fase</h1>
<div class="titulo">Grupo A</div>
<table class="tabela-campeonato-interna"><tr>
  <td class="coluna-time"><b>1°</b><div class="clube"><img src="arquivos/imagens/campeonato/20171205231953fast.png"></div><div class="time">Fast</div></td>
  <td class="coluna-dados">3</td><td class="coluna-dados">3</td><td class="coluna-dados">1</td><td class="coluna-dados">0</td>
  <td class="coluna-dados">2</td><td class="coluna-dados">2</td><td class="coluna-dados">3</td><td class="coluna-dados">-1</td>
</tr><tr>
  <td class="coluna-time"><b>2°</b><div class="clube"><img src="arquivos/imagens/campeonato/20171217013100srec.jpg"></div><div class="time">São Raimundo EC</div></td>
  <td class="coluna-dados">3</td><td class="coluna-dados">3</td><td class="coluna-dados">1</td><td class="coluna-dados">0</td>
  <td class="coluna-dados">2</td><td class="coluna-dados">2</td><td class="coluna-dados">3</td><td class="coluna-dados">-1</td>
</tr></table>
<ul class="rodada-jogos-final"><li>
  <div class="data">SÁB 31/03/2018 15:00 - Arena da Amazônia</div>
  <div class="time1"><img src="arquivos/imagens/campeonato/20171205231350fast.png"></div>
  <div class="resultado">1</div><div class="x"></div><div class="resultado">0</div>
  <div class="time2"><img src="arquivos/imagens/campeonato/20171217013100srec.jpg"></div>
</li></ul>
</div>`
	loc, err := time.LoadLocation("America/Manaus")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseRound(body, "am", 2018, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.matches) != 1 || parsed.matches[0].ExternalID != "am-2018-31032018-fastclube-saoraimundoec" || parsed.matches[0].Phase != "1ª Fase" {
		t.Fatalf("match = %+v", parsed.matches)
	}
	if parsed.clubs["fast"] != (club{}) || parsed.clubs["fastclube"].Name != "Fast Clube" || parsed.clubs["saoraimundoec"].Name != "São Raimundo EC" {
		t.Fatalf("clubs = %+v", parsed.clubs)
	}
	if parsed.tables[1][0].GroupName != "Grupo A" || parsed.tables[1][0].ExternalTeamID != "fastclube" {
		t.Fatalf("group = %+v", parsed.tables[1][0])
	}
}

func TestParseRoundReadsParentheticalPenaltiesAndNamedGroups(t *testing.T) {
	body := `<div>
<h1 class="titulo">1ª Fase</h1>
<div class="titulo">CLASSIFICAÇÃO - GRUPO A</div>
<table class="tabela-campeonato-interna"><tr>
  <td class="coluna-time"><b>1°</b><img src="arquivos/imagens/campeonato/amazonas.jpeg"><div class="time">Amazonas</div></td>
  <td class="coluna-dados">3</td><td class="coluna-dados">1</td><td class="coluna-dados">1</td><td class="coluna-dados">0</td>
  <td class="coluna-dados">0</td><td class="coluna-dados">1</td><td class="coluna-dados">0</td><td class="coluna-dados">1</td>
</tr></table>
<div class="titulo">CLASSIFICAÇÃO - GRUPO B</div>
<table class="tabela-campeonato-interna"><tr>
  <td class="coluna-time"><b>1°</b><img src="arquivos/imagens/campeonato/nacional.jpg"><div class="time">Nacional</div></td>
  <td class="coluna-dados">3</td><td class="coluna-dados">1</td><td class="coluna-dados">1</td><td class="coluna-dados">0</td>
  <td class="coluna-dados">0</td><td class="coluna-dados">1</td><td class="coluna-dados">0</td><td class="coluna-dados">1</td>
</tr></table>
<ul class="rodada-jogos"><li>
  <div class="data">SÁB 02/03/2024 16:00 - Arena da Amazônia</div>
  <div class="time1"><img src="arquivos/imagens/campeonato/amazonas.jpeg"></div>
  <div class="resultado">1 (4)</div><div class="x"></div><div class="resultado">1(2)</div>
  <div class="time2"><img src="arquivos/imagens/campeonato/nacional.jpg"></div>
</li><li>
  <div class="data">DOM 03/03/2024 16:00 - Arena da Amazônia</div>
  <div class="time1"><img src="arquivos/imagens/campeonato/amazonas.jpeg"></div>
  <div class="resultado">0 (1) (4)</div><div class="x"></div><div class="resultado">0 (1) (2)</div>
  <div class="time2"><img src="arquivos/imagens/campeonato/nacional.jpg"></div>
</li></ul>
</div>`
	loc, err := time.LoadLocation("America/Manaus")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseRound(body, "a", 2024, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.matches) != 1 || *parsed.matches[0].HomeScore != 1 || *parsed.matches[0].AwayScore != 1 || *parsed.matches[0].HomePenalties != 4 || *parsed.matches[0].AwayPenalties != 2 {
		t.Fatalf("match = %+v", parsed.matches)
	}
	if parsed.tables[0][0].GroupName != "CLASSIFICAÇÃO - GRUPO A" || parsed.tables[1][0].GroupName != "CLASSIFICAÇÃO - GRUPO B" || parsed.tables[0][0].Position != 1 || parsed.tables[1][0].Position != 1 {
		t.Fatalf("groups = %+v %+v", parsed.tables[0][0], parsed.tables[1][0])
	}
}

func TestParseRoundKeepsNacionalAndNacionalBorbaApart(t *testing.T) {
	body := `<div>
<h1 class="titulo">CLASSIFICAÇÃO GERAL</h1>
<table class="tabela-campeonato-interna"><tr>
  <td class="coluna-time"><b>1°</b><div class="clube"><img src="arquivos/imagens/campeonato/20150429195108nacional.png"></div><div class="time">Nacional</div></td>
  <td class="coluna-dados">10</td><td class="coluna-dados">4</td><td class="coluna-dados">3</td><td class="coluna-dados">1</td>
  <td class="coluna-dados">0</td><td class="coluna-dados">6</td><td class="coluna-dados">2</td><td class="coluna-dados">4</td>
</tr><tr>
  <td class="coluna-time"><b>7°</b><div class="clube"><img src="arquivos/imagens/campeonato/20150429195316nacionalborbense.jpg"></div><div class="time">Nacional Borba</div></td>
  <td class="coluna-dados">4</td><td class="coluna-dados">4</td><td class="coluna-dados">1</td><td class="coluna-dados">1</td>
  <td class="coluna-dados">2</td><td class="coluna-dados">3</td><td class="coluna-dados">5</td><td class="coluna-dados">-2</td>
</tr></table>
<ul class="rodada-jogos"><li>
  <div class="data">SÁB 02/05/2015 16:00 - Estádio</div>
  <div class="time1"><img src="arquivos/imagens/campeonato/20150429195108nacional.png"></div>
  <div class="resultado">1</div><div class="x"></div><div class="resultado">0</div>
  <div class="time2"><img src="arquivos/imagens/campeonato/20150429195316nacionalborbense.jpg"></div>
</li></ul>
</div>`
	loc, err := time.LoadLocation("America/Manaus")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseRound(body, "am", 2015, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.matches) != 1 || parsed.matches[0].ExternalID != "am-2015-02052015-nacional-nacionalborba" {
		t.Fatalf("match = %+v", parsed.matches)
	}
	if parsed.clubs["nacional"].Name != "Nacional" || parsed.clubs["nacionalborba"].Name != "Nacional Borba" {
		t.Fatalf("clubs = %+v", parsed.clubs)
	}
}

func TestParseRoundDropsChampionTagsAndARepeatedFinal(t *testing.T) {
	body := `<div>
<h1 class="titulo">CLASSIFICAÇÃO GERAL</h1>
<table class="tabela-campeonato-interna"><tr>
  <td class="coluna-time"><b>1°</b><div class="time">Manaus (C)</div></td>
  <td class="coluna-dados">20</td><td class="coluna-dados">10</td><td class="coluna-dados">6</td><td class="coluna-dados">2</td>
  <td class="coluna-dados">2</td><td class="coluna-dados">12</td><td class="coluna-dados">6</td><td class="coluna-dados">6</td>
</tr><tr>
  <td class="coluna-time"><b>2°</b><div class="time">Fast * (Vice)</div></td>
  <td class="coluna-dados">17</td><td class="coluna-dados">10</td><td class="coluna-dados">5</td><td class="coluna-dados">2</td>
  <td class="coluna-dados">3</td><td class="coluna-dados">10</td><td class="coluna-dados">8</td><td class="coluna-dados">2</td>
</tr></table>
<h1 class="titulo">Final Geral</h1>
<div class="rodada">Final Geral</div>
<ul class="rodada-jogos-final"><li>
  <div class="data">QUA 24/04/2019 20:00 - Ismael Benigno</div>
  <div class="time1"><img src="arquivos/imagens/campeonato/fast.png"></div>
  <div class="resultado">0</div><div class="x"></div><div class="resultado">2</div>
  <div class="time2"><img src="arquivos/imagens/campeonato/manausfc.png"></div>
</li></ul>
<h1 class="titulo">Final</h1>
<div class="rodada">Final</div>
<ul class="rodada-jogo-final"><li>
  <div class="data">QUA 24/04/2019 20:00 - Ismael Benigno</div>
  <div class="time1"><img src="arquivos/imagens/campeonato/fast.png"></div>
  <div class="resultado">0</div><div class="x"></div><div class="resultado">2</div>
  <div class="time2"><img src="arquivos/imagens/campeonato/manausfc.png"></div>
</li></ul>
<h1 class="titulo">1ª Fase</h1>
<table class="tabela-campeonato-interna"><tr>
  <td class="coluna-time"><b>1°</b><div class="clube"><img src="arquivos/imagens/campeonato/manausfc.png"></div><div class="time">Manaus</div></td>
  <td class="coluna-dados">8</td><td class="coluna-dados">4</td><td class="coluna-dados">2</td><td class="coluna-dados">2</td>
  <td class="coluna-dados">0</td><td class="coluna-dados">6</td><td class="coluna-dados">2</td><td class="coluna-dados">4</td>
</tr><tr>
  <td class="coluna-time"><b>2°</b><div class="clube"><img src="arquivos/imagens/campeonato/fast.png"></div><div class="time">Fast</div></td>
  <td class="coluna-dados">6</td><td class="coluna-dados">4</td><td class="coluna-dados">2</td><td class="coluna-dados">0</td>
  <td class="coluna-dados">2</td><td class="coluna-dados">4</td><td class="coluna-dados">3</td><td class="coluna-dados">1</td>
</tr></table>
</div>`
	loc, err := time.LoadLocation("America/Manaus")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseRound(body, "am", 2019, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.matches) != 1 || parsed.matches[0].ExternalID != "am-2019-24042019-fast-manaus" || parsed.matches[0].Phase != "Final Geral" {
		t.Fatalf("match = %+v", parsed.matches)
	}
	if parsed.clubs["fast"].Name != "Fast" || parsed.clubs["manaus"].Name != "Manaus" || parsed.clubs["fastvice"].Name != "" || parsed.clubs["manausc"].Name != "" {
		t.Fatalf("clubs = %+v", parsed.clubs)
	}
}

func TestParseRoundRepairsADoubleEncodedClubName(t *testing.T) {
	body := `<div>
<h1 class="titulo">CLASSIFICAÇÃO GERAL</h1>
<table class="tabela-campeonato-interna"><tr>
  <td class="coluna-time"><b>1°</b><div class="time">Sul América</div></td>
  <td class="coluna-dados">10</td><td class="coluna-dados">14</td><td class="coluna-dados">3</td><td class="coluna-dados">1</td>
  <td class="coluna-dados">10</td><td class="coluna-dados">13</td><td class="coluna-dados">25</td><td class="coluna-dados">-12</td>
</tr></table>
<h1 class="titulo">1ª Fase</h1>
<table class="tabela-campeonato-interna"><tr>
  <td class="coluna-time"><b>7°</b><div class="clube"><img src="arquivos/imagens/campeonato/sulamerica.png"></div><div class="time">Sul AmÃ©rica</div></td>
  <td class="coluna-dados">6</td><td class="coluna-dados">7</td><td class="coluna-dados">2</td><td class="coluna-dados">0</td>
  <td class="coluna-dados">5</td><td class="coluna-dados">8</td><td class="coluna-dados">13</td><td class="coluna-dados">-5</td>
</tr></table>
<ul class="rodada-jogos"><li>
  <div class="data">QUA 20/02/2019 20:00 - Arena da AmazÃ´nia</div>
  <div class="time1"><img src="arquivos/imagens/campeonato/sulamerica.png"></div>
  <div class="resultado">2</div><div class="x"></div><div class="resultado">1</div>
  <div class="time2"><img src="arquivos/imagens/campeonato/fast.png"></div>
</li></ul>
<table class="tabela-campeonato-interna"><tr>
  <td class="coluna-time"><b>2°</b><div class="clube"><img src="arquivos/imagens/campeonato/fast.png"></div><div class="time">Fast</div></td>
  <td class="coluna-dados">6</td><td class="coluna-dados">4</td><td class="coluna-dados">2</td><td class="coluna-dados">0</td>
  <td class="coluna-dados">2</td><td class="coluna-dados">4</td><td class="coluna-dados">3</td><td class="coluna-dados">1</td>
</tr></table>
</div>`
	loc, err := time.LoadLocation("America/Manaus")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseRound(body, "am", 2019, loc)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.clubs["sulamerica"].Name != "Sul América" || parsed.clubs["sulamarica"].Name != "" {
		t.Fatalf("clubs = %+v", parsed.clubs)
	}
	if len(parsed.matches) != 1 || parsed.matches[0].ExternalID != "am-2019-20022019-sulamerica-fast" || parsed.matches[0].Venue != "Arena da Amazônia" {
		t.Fatalf("match = %+v", parsed.matches)
	}
}

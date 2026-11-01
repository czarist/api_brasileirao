package fcf

import (
	"strings"
	"testing"
	"time"
)

const yearPage = `
<select>
<option value="">Selecione um campeonato...</option>
<option value="https://fcf.com.br/competicoes/profissionais/2026/686/">CAMPEONATO CATARINENSE FORT ATACADISTA SÉRIE A 2026</option>
<option value="https://fcf.com.br/competicoes/profissionais/2026/698/">CAMPEONATO CATARINENSE SÉRIE B 2026</option>
<option value="https://fcf.com.br/competicoes/profissionais/2026/724/">CATARINENSE SÉRIE C - 2026</option>
<option value="https://fcf.com.br/competicoes/profissionais/2026/723/">COPA SANTA CATARINA 2026</option>
<option value="https://fcf.com.br/competicoes/profissionais/2026/695/">RECOPA 2026</option>
<option value="https://fcf.com.br/competicoes/profissionais/2025/1/">CAMPEONATO CATARINENSE SÉRIE A 2025</option>
</select>
<a href="https://fcf.com.br/competicoes/profissionais/2026/">2026</a>
`

const seasonPage = `
<div class="fcf-fase-panel" data-fase-nome="INICIAL - TURNO/RETURNO">
  <h4 class="fcf-grupo-nome">GRUPO A</h4>
  <table class="fcf-table fcf-tabela-classificacao">
    <thead><tr>
      <th class="fcf-col-pos">#</th>
      <th>Clube</th>
      <th title="Pontos">PTS</th>
      <th title="Jogos">J</th>
      <th title="Vitórias">V</th>
      <th title="Empates">E</th>
      <th title="Derrotas">D</th>
      <th title="Gols Pró">GP</th>
      <th title="Gols Contra">GC</th>
      <th title="Saldo">SG</th>
    </tr></thead>
    <tbody>
      <tr>
        <td class="fcf-col-pos">1</td>
        <td class="fcf-col-clube"><img src="https://fcf.com.br/wp-content/uploads/fcf-logos/brusque-saf.png" alt="BRUSQUE SAF"><span>BRUSQUE SAF</span></td>
        <td>12</td><td>6</td><td>3</td><td>3</td><td>0</td><td>9</td><td>6</td><td>3</td>
      </tr>
      <tr>
        <td class="fcf-col-pos">2</td>
        <td class="fcf-col-clube"><img src="https://fcf.com.br/wp-content/uploads/fcf-logos/avai.png" alt="AVAÍ"><span>AVAÍ</span></td>
        <td>10</td><td>6</td><td>3</td><td>1</td><td>2</td><td>8</td><td>7</td><td>1</td>
      </tr>
    </tbody>
  </table>
  <div data-rodada-nome="1ª RODADA">
    <div class="fcf-jogo-card fcf-jogo--realizado">
      <span class="fcf-jogo-numero">Jogo 4</span>
      <span class="fcf-jogo-data">Ter, 06/01/2026 – 19:00</span>
      <span class="fcf-jogo-estadio">Arena Condá / Chapecó</span>
      <div class="fcf-time fcf-time--mandante"><img src="https://fcf.com.br/wp-content/uploads/fcf-logos/chapecoense.png" alt="CHAPECOENSE"><span class="fcf-time-nome">CHAPECOENSE</span></div>
      <div class="fcf-placar"><span class="fcf-gol">1</span><span class="fcf-placar-sep">×</span><span class="fcf-gol">1</span></div>
      <div class="fcf-time fcf-time--visitante"><img src="https://fcf.com.br/wp-content/uploads/fcf-logos/brusque-saf.png" alt="BRUSQUE SAF"><span class="fcf-time-nome">BRUSQUE SAF</span></div>
    </div>
    <div class="fcf-jogo-card">
      <span class="fcf-jogo-numero">Jogo 5</span>
      <span class="fcf-jogo-data">07/01/2026</span>
      <div class="fcf-time fcf-time--mandante"><span class="fcf-time-nome">AVAÍ</span></div>
      <div class="fcf-placar"></div>
      <div class="fcf-time fcf-time--visitante"><img src="https://fcf.com.br/wp-content/uploads/fcf-logos/avai-away.png" alt="FIGUEIRENSE"><span class="fcf-time-nome">FIGUEIRENSE</span></div>
    </div>
  </div>
</div>
<div class="fcf-fase-panel" data-fase-nome="QUADRANGULAR">
  <h4 class="fcf-grupo-nome">GRUPO C</h4>
  <table class="fcf-tabela-classificacao">
    <thead><tr>
      <th>#</th><th>Clube</th>
      <th title="Pontos">PTS</th><th title="Jogos">J</th><th title="Vitórias">V</th><th title="Empates">E</th><th title="Derrotas">D</th>
      <th title="Gols Pró">GP</th><th title="Gols Contra">GC</th><th title="Saldo">SG</th>
    </tr></thead>
    <tbody>
      <tr>
        <td>1</td>
        <td class="fcf-col-clube"><img src="https://fcf.com.br/wp-content/uploads/fcf-logos/brusque-saf.png" alt="BRUSQUE SAF"><span>BRUSQUE SAF</span></td>
        <td>3</td><td>1</td><td>1</td><td>0</td><td>0</td><td>2</td><td>0</td><td>2</td>
      </tr>
    </tbody>
  </table>
</div>
`

func TestParseSeasonsKeepsAdultSeriesAndSkipsCups(t *testing.T) {
	refs, err := ParseSeasons(yearPage, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 3 || refs[0].Code != codeA || refs[0].ID != "686" || refs[1].Code != codeB || refs[2].Code != codeC || refs[2].ID != "724" {
		t.Fatalf("refs = %+v", refs)
	}
}

func TestParseYearsUsesTheYearIndexOnly(t *testing.T) {
	years, err := ParseYears(`<a href="/competicoes/profissionais/2025/">2025</a><a href="/competicoes/profissionais/2026/686/">A</a><a href="https://fcf.com.br/competicoes/profissionais/2026/">2026</a>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(years) != 2 || years[0] != 2025 || years[1] != 2026 {
		t.Fatalf("years = %v", years)
	}
}

func TestParsePageKeepsTheTableWithMoreGamesPlayed(t *testing.T) {
	loc, err := saopaulo()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParsePage(seasonPage, 2026, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.matches) != 2 {
		t.Fatalf("matches = %d", len(parsed.matches))
	}
	first := parsed.matches[0]
	if first.Status != "finished" || first.HomeScore == nil || *first.HomeScore != 1 || *first.AwayScore != 1 {
		t.Fatalf("finished match = %+v", first)
	}
	if first.RoundNumber != 1 || first.Phase != "INICIAL - TURNO/RETURNO" || first.Venue != "Arena Condá" || first.City != "Chapecó" {
		t.Fatalf("match detail = %+v", first)
	}
	want := time.Date(2026, 1, 6, 22, 0, 0, 0, time.UTC)
	if first.Kickoff == nil || !first.Kickoff.Equal(want) {
		t.Fatalf("kickoff = %v", first.Kickoff)
	}
	second := parsed.matches[1]
	if second.Status != "scheduled" || second.HomeScore != nil || second.Kickoff != nil {
		t.Fatalf("scheduled match = %+v", second)
	}
	if second.HomeExternalID != "name:AVAI" {
		t.Fatalf("home id = %s", second.HomeExternalID)
	}
	rows := mergeStandings(parsed.tables)
	if len(rows) != 2 {
		t.Fatalf("standings = %+v", rows)
	}
	if rows[0].ExternalTeamID != "brusque-saf" || rows[0].GroupName != "GRUPO A" || rows[0].Played != 6 || rows[0].Points != 12 || rows[0].YellowCards != 0 || rows[0].WinPercentage != 0 {
		t.Fatalf("first row = %+v", rows[0])
	}
	if rows[1].ExternalTeamID != "avai" || rows[1].GoalDifference != 1 {
		t.Fatalf("second row = %+v", rows[1])
	}
}

func TestParsePageUsesRoundOneWhenThePhasePublishesNoRound(t *testing.T) {
	loc, err := saopaulo()
	if err != nil {
		t.Fatal(err)
	}
	body := `<div class="fcf-fase-panel" data-fase-nome="FINAIS"><div data-rodada-nome="">
<div class="fcf-jogo-card fcf-jogo--realizado">
<span class="fcf-jogo-numero">Jogo 65</span>
<span class="fcf-jogo-data">01/03/2026 – 18:00</span>
<div class="fcf-time fcf-time--mandante"><span class="fcf-time-nome">BARRA</span></div>
<div class="fcf-placar"><span class="fcf-gol">3</span><span class="fcf-gol">1</span></div>
<div class="fcf-time fcf-time--visitante"><span class="fcf-time-nome">AVAÍ</span></div>
</div></div></div>`
	parsed, err := ParsePage(body, 2026, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.matches) != 1 || parsed.matches[0].RoundNumber != 1 || parsed.matches[0].Phase != "FINAIS" {
		t.Fatalf("match = %+v", parsed.matches)
	}
	named := strings.Replace(body, `data-rodada-nome=""`, `data-rodada-nome="SEMIFINAIS SEMIFINAIS"`, 1)
	namedParsed, err := ParsePage(named, 2026, loc)
	if err != nil {
		t.Fatal(err)
	}
	if namedParsed.matches[0].RoundNumber != 1 {
		t.Fatalf("named round = %+v", namedParsed.matches[0])
	}
	ordinal := strings.Replace(body, `data-rodada-nome=""`, `data-rodada-nome="10º RODADA"`, 1)
	ordinalParsed, err := ParsePage(ordinal, 2026, loc)
	if err != nil {
		t.Fatal(err)
	}
	if ordinalParsed.matches[0].RoundNumber != 10 {
		t.Fatalf("ordinal round = %+v", ordinalParsed.matches[0])
	}
}

func TestParsePageUsesTheGroupTitleWhenThePhaseNameIsEmpty(t *testing.T) {
	loc, err := saopaulo()
	if err != nil {
		t.Fatal(err)
	}
	body := `<div class="fcf-fase-panel" data-fase-nome="">
<h4 class="fcf-grupo-nome">UNICO</h4>
<div data-rodada-nome="1ª RODADA">
<div class="fcf-jogo-card fcf-jogo--realizado">
<span class="fcf-jogo-numero">Jogo 1</span>
<span class="fcf-jogo-data">18/03/2023 – 16:30</span>
<div class="fcf-time fcf-time--mandante"><span class="fcf-time-nome">AVAÍ</span></div>
<div class="fcf-placar"><span class="fcf-gol">1</span><span class="fcf-gol">0</span></div>
<div class="fcf-time fcf-time--visitante"><span class="fcf-time-nome">FIGUEIRENSE</span></div>
</div></div></div>`
	parsed, err := ParsePage(body, 2023, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.matches) != 1 || parsed.matches[0].Phase != "UNICO" || parsed.matches[0].RoundNumber != 1 {
		t.Fatalf("match = %+v", parsed.matches)
	}
	bare := `<div class="fcf-fase-panel" data-fase-nome=""><div data-rodada-nome="1ª RODADA">
<div class="fcf-jogo-card fcf-jogo--realizado">
<span class="fcf-jogo-numero">Jogo 4</span>
<span class="fcf-jogo-data">27/05/2023 – 15:00</span>
<div class="fcf-time fcf-time--mandante"><span class="fcf-time-nome">CARAVAGGIO</span></div>
<div class="fcf-placar"><span class="fcf-gol">0</span><span class="fcf-gol">2</span></div>
<div class="fcf-time fcf-time--visitante"><span class="fcf-time-nome">GUARANI</span></div>
</div></div></div>`
	bareParsed, err := ParsePage(bare, 2023, loc)
	if err != nil {
		t.Fatal(err)
	}
	if bareParsed.matches[0].Phase != "1ª RODADA" || bareParsed.matches[0].RoundNumber != 1 {
		t.Fatalf("bare phase = %+v", bareParsed.matches[0])
	}
}

func TestDivisionCodeRejectsANumberedSeries(t *testing.T) {
	if _, ok := divisionCode("Série A2"); ok {
		t.Fatal("Série A2 was accepted as Série A")
	}
	if code, ok := divisionCode("Catarinense Série C"); !ok || code != codeC {
		t.Fatalf("code = %s %v", code, ok)
	}
	if strings.Contains(codeA, " ") {
		t.Fatal("code has a space")
	}
}

package fpfpr

import (
	"strings"
	"testing"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
)

func TestParseCatalogKeepsThePublishedDivisionsAndSkipsTheCup(t *testing.T) {
	body := `
	<select>
		<option value="">-- Selecione o campeonato --</option>
		<option value="71">Copa Paraná 2026</option>
		<option value="33">Paranaense 2026</option>
		<option value="44">Segundona 2026</option>
		<option value="66">Terceirona 2026</option>
		<option value="9">Paranaense Feminino 2026</option>
	</select>`
	refs, err := ParseCatalog(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 3 || refs[0].Code != codeA || refs[0].ID != "33" || refs[0].Year != 2026 || refs[1].Code != codeB || refs[2].Code != codeC || refs[2].ID != "66" {
		t.Fatalf("refs = %+v", refs)
	}
}

func TestParsePageKeepsCarouselMatchesAndRoundsThePublishedPercent(t *testing.T) {
	body := `
	<button class="zabmcbf-fase-tab" data-fase-key="primeira-fase">Primeira Fase</button>
	<button class="zabmcbf-fase-tab" data-fase-key="descenso">Descenso</button>
	<div class="zabmcbf-grupo-classificacao">
		<h3 class="zabmcbf-grupo-titulo">1ª FASE - GRUPO A</h3>
		<table class="zabmcbf-tabela">
			<thead><tr>
				<th>CLASSIFICAÇÃO</th><th>P</th><th>J</th><th>V</th><th>E</th><th>D</th><th>GP</th><th>GC</th><th>SG</th><th>%</th>
			</tr></thead>
			<tbody>
				<tr><td><span class="posicaotabela">1</span> LONDRINA</td><td>22</td><td>12</td><td>5</td><td>7</td><td>0</td><td>19</td><td>7</td><td>12</td><td>61.1%</td></tr>
				<tr><td><span class="posicaotabela">4</span> CASCAVEL</td><td>6</td><td>6</td><td>1</td><td>3</td><td>2</td><td>5</td><td>6</td><td>-1</td><td>33.3%</td></tr>
				<tr><td><span class="posicaotabela">6</span> FOZ DO IGUAÇU</td><td>11</td><td>8</td><td>3</td><td>2</td><td>3</td><td>12</td><td>18</td><td>-6</td><td>45.8%</td></tr>
			</tbody>
		</table>
	</div>
	<div class="zabmcbf-carousel-slide" data-rodada="7" data-rodada-label="Descenso">
		<div class="zabmcbf-carousel-jogo" data-fase-key="descenso">
			<a href="https://federacaopr.com.br/campeonato/profissional/2026/33/jogo/1043/">
				<span class="zabmcbf-carousel-jogo-estadio">Willie Davids</span>
				<span class="zabmcbf-carousel-jogo-data">01/02 • Dom • 18:30</span>
				<div class="zabmcbf-carousel-jogo-mandante"><div class="zabmcbf-carousel-jogo-time-nome">GALO MARINGA</div></div>
				<span class="zabmcbf-carousel-jogo-placar-numero">0 x 5</span>
				<div class="zabmcbf-carousel-jogo-visitante"><div class="zabmcbf-carousel-jogo-time-nome">MARINGÁ</div></div>
			</a>
		</div>
		<div class="zabmcbf-carousel-jogo" data-fase-key="primeira-fase">
			<a href="/jogo/900/">
				<span class="zabmcbf-carousel-jogo-data">06/01 • Ter • 16:00</span>
				<div class="zabmcbf-carousel-jogo-mandante"><div class="zabmcbf-carousel-jogo-time-nome">LONDRINA</div></div>
				<span class="zabmcbf-carousel-jogo-placar-numero">0 x 0</span>
				<div class="zabmcbf-carousel-jogo-visitante"><div class="zabmcbf-carousel-jogo-time-nome">CASCAVEL</div></div>
			</a>
		</div>
	</div>
	<div class="zabmcbf-fase-descenso-conteudo">
		<div class="zabmcbf-carousel-jogo" data-fase-key="descenso">
			<a href="/jogo/1043/"><span class="zabmcbf-carousel-jogo-placar-numero">9 x 9</span></a>
		</div>
	</div>`
	loc, err := time.LoadLocation(location)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParsePage(body, 2026, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.matches) != 2 {
		t.Fatalf("matches = %d", len(parsed.matches))
	}
	first := parsed.matches[0]
	if first.ExternalID != "2026-1043" || first.MatchNumber != 1043 || first.Phase != "Descenso" || first.RoundNumber != 7 || first.Status != domain.StatusFinished || first.HomeScore == nil || *first.HomeScore != 0 || first.AwayScore == nil || *first.AwayScore != 5 {
		t.Fatalf("first = %+v", first)
	}
	if first.Kickoff == nil || first.Kickoff.UTC().Format(time.RFC3339) != "2026-02-01T21:30:00Z" || first.Venue != "Willie Davids" || first.HomeExternalID != "galo-maringa" || first.AwayExternalID != "maringa" {
		t.Fatalf("first kickoff/clubs = %+v %s %s", first.Kickoff, first.HomeExternalID, first.AwayExternalID)
	}
	second := parsed.matches[1]
	if second.ExternalID != "2026-900" || second.Status != domain.StatusFinished || second.HomeScore == nil || *second.HomeScore != 0 || second.AwayScore == nil || *second.AwayScore != 0 || second.Phase != "Primeira Fase" {
		t.Fatalf("second = %+v", second)
	}
	rows := mergeStandings(parsed.tables)
	if len(rows) != 3 {
		t.Fatalf("standings = %+v", rows)
	}
	if rows[0].ExternalTeamID != "londrina" || rows[0].Position != 1 || rows[0].Points != 22 || rows[0].Played != 12 || rows[0].GoalDifference != 12 || rows[0].WinPercentage != 61 || rows[0].YellowCards != 0 || rows[0].GroupName != "1ª FASE - GRUPO A" {
		t.Fatalf("leader = %+v", rows[0])
	}
	if rows[1].GoalDifference != -1 || rows[1].WinPercentage != 33 {
		t.Fatalf("cascavel = %+v", rows[1])
	}
	if rows[2].ExternalTeamID != "foz-do-iguacu" || rows[2].WinPercentage != 46 || rows[2].GoalDifference != -6 {
		t.Fatalf("foz = %+v", rows[2])
	}
	if err := uniqueMatches(parsed.matches); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(first.ExternalID, "https") {
		t.Fatal("external id kept a url")
	}
}

func TestParsePageLeavesAnUnscoredMatchScheduled(t *testing.T) {
	body := `
	<button class="zabmcbf-fase-tab" data-fase-key="final">Final</button>
	<div class="zabmcbf-carousel-slide" data-rodada="1" data-rodada-label="Final">
		<div class="zabmcbf-carousel-jogo" data-fase-key="final">
			<a href="/jogo/1/">
				<div class="zabmcbf-carousel-jogo-mandante"><div class="zabmcbf-carousel-jogo-time-nome">A</div></div>
				<span class="zabmcbf-carousel-jogo-placar-numero">x</span>
				<div class="zabmcbf-carousel-jogo-visitante"><div class="zabmcbf-carousel-jogo-time-nome">B</div></div>
			</a>
		</div>
	</div>`
	loc := time.FixedZone("UTC-3", -3*60*60)
	parsed, err := ParsePage(body, 2026, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.matches) != 1 || parsed.matches[0].Status != domain.StatusScheduled || parsed.matches[0].HomeScore != nil || parsed.matches[0].Kickoff != nil {
		t.Fatalf("match = %+v", parsed.matches[0])
	}
}

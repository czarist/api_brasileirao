package cbf

import (
	"os"
	"testing"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
)

func TestParseStandingsUsesTheClubLinkNotTheNextOpponent(t *testing.T) {
	body := readFixture(t, "testdata/standings_sample.html")
	rows, names, err := ParseStandings(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d", len(rows))
	}
	want := []domain.StandingInput{
		{ExternalTeamID: "20016", Position: 1, Points: 73, Played: 33, Wins: 22, Draws: 7, Losses: 4, GoalsFor: 68, GoalsAgainst: 28, GoalDifference: 40, YellowCards: 69, RedCards: 7, WinPercentage: 73},
		{ExternalTeamID: "20002", Position: 2, Points: 70, Played: 33, Wins: 20, Draws: 10, Losses: 3, GoalsFor: 56, GoalsAgainst: 22, GoalDifference: 34, YellowCards: 72, RedCards: 7, WinPercentage: 70},
		{ExternalTeamID: "59849", Position: 3, Points: 57, Played: 33, Wins: 17, Draws: 6, Losses: 10, GoalsFor: 52, GoalsAgainst: 46, GoalDifference: 6, YellowCards: 83, RedCards: 4, WinPercentage: 57},
	}
	for i, row := range want {
		if rows[i] != row {
			t.Fatalf("row %d = %+v", i, rows[i])
		}
	}
	if names["20016"] != "Flamengo" || names["20002"] != "Palmeiras" || names["59849"] != "Cruzeiro" {
		t.Fatalf("names = %+v", names)
	}
}

func TestParseStandingsRejectsEmptyAndBlockedPages(t *testing.T) {
	if _, _, err := ParseStandings([]byte(`<html><body><table></table></body></html>`)); err == nil {
		t.Fatal("empty table was accepted")
	}
	if _, _, err := ParseStandings([]byte(`<html>cf-browser-verification</html>`)); err == nil {
		t.Fatal("block page was accepted")
	}
}

func TestAssembleFixtureStatuses(t *testing.T) {
	collected := time.Date(2026, 10, 30, 15, 0, 0, 0, time.UTC)
	snap, err := Assemble(SerieA, 2026, "1260611", collected,
		readFixture(t, "testdata/matches_sample.json"),
		[]byte("[]"),
		readFixture(t, "testdata/standings_sample.html"))
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]domain.MatchInput{}
	for _, match := range snap.Matches {
		byID[match.ExternalID] = match
	}
	finished := byID["831894"]
	if finished.Status != domain.StatusFinished || *finished.HomeScore != 2 || *finished.AwayScore != 2 {
		t.Fatalf("finished = %+v", finished)
	}
	if finished.Kickoff == nil || finished.Kickoff.Format(time.RFC3339) != "2026-01-28T22:00:00Z" {
		t.Fatalf("kickoff = %v", finished.Kickoff)
	}
	draw := byID["831920"]
	if draw.Status != domain.StatusFinished || *draw.HomeScore != 0 || *draw.AwayScore != 0 {
		t.Fatalf("0-0 was not kept: %+v", draw)
	}
	open := byID["832099"]
	if open.Status != domain.StatusScheduled || open.HomeScore != nil || open.RoundNumber != 21 {
		t.Fatalf("open match = %+v", open)
	}
	if open.Kickoff == nil || open.Kickoff.Format(time.RFC3339) != "2026-11-14T23:30:00Z" {
		t.Fatalf("rescheduled kickoff = %v", open.Kickoff)
	}
	undefined := byID["832251"]
	if undefined.Kickoff != nil || undefined.Status != domain.StatusScheduled || undefined.HomeScore != nil {
		t.Fatalf("undefined date = %+v", undefined)
	}
	if snap.SeasonExternalID != "1260611" || snap.SeasonYear != 2026 {
		t.Fatalf("season = %+v", snap)
	}
}

func TestScheduleChangeKeepsRoundAndCanPostpone(t *testing.T) {
	matches := []byte(`{"Fase Unica":{"jogos":[
		{"ref_jogo":"1","num_jogo":"1","rodada":"21","estadio":"","cidade":"","uf":"","data":" 01/01/1900","hora":null,
		 "mandante":{"nome":"Chapecoense","uf":"SC","url_escudo":"https://conteudo.cbf.com.br/clubes/20086/escudo.jpg","gols":"","panaltis":""},
		 "visitante":{"nome":"Vasco da Gama Saf","uf":"RJ","url_escudo":"https://conteudo.cbf.com.br/clubes/60646/escudo.jpg","gols":"","panaltis":""}},
		{"ref_jogo":"2","num_jogo":"2","rodada":"9","estadio":"","cidade":"","uf":"","data":" 02/03/2026","hora":"16:00",
		 "mandante":{"nome":"Bahia","uf":"BA","url_escudo":"https://conteudo.cbf.com.br/clubes/61377/escudo.jpg","gols":"","panaltis":""},
		 "visitante":{"nome":"Remo","uf":"PA","url_escudo":"https://conteudo.cbf.com.br/clubes/20022/escudo.jpg","gols":"","panaltis":""}}
	]}}`)
	imt := []byte(`[
		{"id_jogo":"1","tipo":"Alteração","data_inicial":"1900-01-01 ::00","data_final":"1900-01-01 ::00","data_solicitacao":"2026-10-01 10:00:00","nome_estadio_final":null,"motivo":"Remarcação de jogo pendente"},
		{"id_jogo":"2","tipo":"Alteração","data_inicial":"2026-03-02 16:00:00","data_final":"2026-03-02 16:00:00","data_solicitacao":"2026-02-01 10:00:00","nome_estadio_final":"Fonte Nova","motivo":"Jogo suspenso por falta de energia"},
		{"id_jogo":"2","tipo":"Alteração","data_inicial":"2026-03-02 16:00:00","data_final":null,"data_solicitacao":"2026-01-01 10:00:00","nome_estadio_final":"Old","motivo":"ignored older row"}
	]`)
	standings := []byte(`<table><tr><td><strong>1</strong><a href="/times/x/20086"><strong>Chapecoense</strong></a></td><td>1</td><td>1</td><td>0</td><td>1</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td></td></tr></table>`)
	// Standings need both clubs that appear... Assemble does not require every match club in the table.
	// The postponed row's team must parse. One standings row is enough for this unit.
	snap, err := Assemble(SerieA, 2026, "99", time.Unix(0, 0).UTC(), matches, imt, standings)
	if err != nil {
		t.Fatal(err)
	}
	postponed := snap.Matches[0]
	if postponed.Status != domain.StatusPostponed || postponed.Kickoff != nil || postponed.RoundNumber != 21 {
		t.Fatalf("postponed = %+v", postponed)
	}
	suspended := snap.Matches[1]
	if suspended.Status != domain.StatusSuspended || suspended.Venue != "Fonte Nova" || suspended.RoundNumber != 9 {
		t.Fatalf("suspended = %+v status %s venue %s", suspended, suspended.Status, suspended.Venue)
	}
}

func TestFinishedScoreBeatsACancellationNote(t *testing.T) {
	loc := time.FixedZone("UTC-3", -3*60*60)
	matches, _, err := ParseMatches([]byte(`{"Fase":{"jogos":[{"ref_jogo":"9","num_jogo":"1","rodada":"1","data":" 01/02/2026","hora":"16:00","estadio":"A","cidade":"B","uf":"SP","mandante":{"nome":"Santos FC","uf":"SP","url_escudo":"https://conteudo.cbf.com.br/clubes/20008/escudo.jpg","gols":"1","panaltis":""},"visitante":{"nome":"Mirassol","uf":"SP","url_escudo":"https://conteudo.cbf.com.br/clubes/20385/escudo.jpg","gols":"0","panaltis":""}}]}}`), loc)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := ParseIMT([]byte(`[{"id_jogo":"9","tipo":"cancelado","data_final":"2026-02-01 16:00:00","data_solicitacao":"2026-02-02 00:00:00","motivo":"cancelado","nome_estadio_final":null}]`), loc)
	if err != nil {
		t.Fatal(err)
	}
	applySchedule(matches, changes)
	if matches[0].Status != domain.StatusFinished {
		t.Fatalf("status = %s", matches[0].Status)
	}
}

func TestSeasonExternalID(t *testing.T) {
	body := readFixture(t, "testdata/catalog_sample.json")
	id, err := SeasonExternalID(body, "1", 2026)
	if err != nil {
		t.Fatal(err)
	}
	if id != "1260611" {
		t.Fatalf("id = %s", id)
	}
	if _, err := SeasonExternalID(body, "1", 2010); err == nil {
		t.Fatal("missing year was accepted")
	}
	if _, err := SeasonExternalID([]byte(`{"campeonatos":{}}`), "1", 2026); err == nil {
		t.Fatal("object catalog was accepted")
	}
}

func TestParseStandingsReadsEscapedGroups(t *testing.T) {
	body := []byte(`<html><script>{\"title\":\"GRUPO A\",\"data\":[{\"cod_time\":\"1\",\"time\":\"One\",\"posicao\":\"1\",\"pontos\":\"10\",\"jogos\":\"4\",\"vitorias\":\"3\",\"empates\":\"1\",\"derrotas\":\"0\",\"gols_pro\":\"5\",\"gols_contra\":\"1\",\"gols_saldo\":\"4\",\"cartoes_amarelo\":\"2\",\"cartoes_vermelho\":\"0\",\"aproveitamento\":\"83\"}]},{\"title\":\"GRUPO B\",\"data\":[{\"cod_time\":\"2\",\"time\":\"Two\",\"posicao\":\"1\",\"pontos\":\"9\",\"jogos\":\"4\",\"vitorias\":\"3\",\"empates\":\"0\",\"derrotas\":\"1\",\"gols_pro\":\"4\",\"gols_contra\":\"2\",\"gols_saldo\":\"2\",\"cartoes_amarelo\":\"1\",\"cartoes_vermelho\":\"0\",\"aproveitamento\":\"75\"}]}</script></html>`)
	rows, names, err := ParseStandings(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || names["1"] != "One" || names["2"] != "Two" {
		t.Fatalf("rows=%d names=%v", len(rows), names)
	}
	if rows[0].GroupName != "GRUPO A" || rows[0].GroupOrder != 0 || rows[0].Position != 1 || rows[0].Points != 10 {
		t.Fatalf("first = %+v", rows[0])
	}
	if rows[1].GroupName != "GRUPO B" || rows[1].GroupOrder != 1 || rows[1].Position != 1 {
		t.Fatalf("second = %+v", rows[1])
	}
}

func TestFirstPointsPhaseSkipsTheKnockout(t *testing.T) {
	body := []byte(`{\"fase_id\":\"2117\",\"fase_nome\":\"Final\",\"fase_tipo\":\"eliminacao\"},\"phasesList\":[{\"fase_id\":\"2013\",\"fase_nome\":\"1ª Fase\",\"fase_tipo\":\"pontuacao\"},{\"fase_id\":\"2098\",\"fase_tipo\":\"pontuacao\"}]`)
	id, ok := firstPointsPhase(body)
	if !ok || id != "2013" {
		t.Fatalf("phase = %q ok=%v", id, ok)
	}
}

func TestShortName(t *testing.T) {
	if ShortName("São Paulo") != "SAO" || ShortName("Vitória") != "VIT" || ShortName("Some New Club") != "SOM" {
		t.Fatalf("short names = %s %s %s", ShortName("São Paulo"), ShortName("Vitória"), ShortName("Some New Club"))
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

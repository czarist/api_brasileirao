package fsfse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
)

func TestFetchReadsTheSeasonPageAndSkipsCups(t *testing.T) {
	const index = `<div>
<a class="camp-card" href="/campeonatos/sergipano-a1-2026/">A1</a>
<a class="camp-card" href="/campeonatos/sergipano-serie-a3-2025/">A3</a>
<a class="camp-card" href="/campeonatos/copa-lotese-2026/">Copa</a>
<a class="camp-card" href="/campeonatos/sergipano-sub-20-a1-2026/">Sub</a>
</div>`
	const season = `<div>
<h3 class="fase-title">Fase Classificatória</h3>
<div class="classificacao-full" data-fase-id="9">
<table><tr>
  <td class="table-class__pos">1</td>
  <td><div class="table-class__clube"><img src="/escudos/20034.jpg"><span>Sergipe</span></div></td>
  <td>1</td><td>1</td><td>0</td><td>0</td><td>3</td><td>0</td><td>+3</td><td>3</td>
</tr><tr>
  <td class="table-class__pos">2</td>
  <td><div class="table-class__clube"><img src="/escudos/64368.jpg"><span>Confiança</span></div></td>
  <td>1</td><td>0</td><td>0</td><td>1</td><td>0</td><td>3</td><td>-3</td><td>0</td>
</tr></table>
</div>
<div class="jogos-group" data-group="f9r1">
  <h3 class="jogos-group__title">RODADA 1</h3>
  <div class="jogos-group__date">sábado - 10 de janeiro de 2026</div>
  <article class="match-panel">
    <div class="match-panel__team"><span class="match-panel__name">Sergipe</span><img src="/escudos/20034.jpg"></div>
    <span class="match-panel__placar">0 - 0</span>
    <div class="match-panel__team"><img src="/escudos/64368.jpg"><span class="match-panel__name">Confiança</span></div>
    <div class="match-panel__meta"><span>16:00</span><span>Batistão</span></div>
    <a href="/jogo/50762/sumula/">Súmula</a>
  </article>
</div>
</div>`
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/campeonatos/sergipano-a1-2026/" {
			_, _ = w.Write([]byte(season))
			return
		}
		_, _ = w.Write([]byte(index))
	}))
	defer srv.Close()
	client, err := NewClient(ClientOptions{BaseURL: srv.URL, MinInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	source := NewSource(client)
	plans, err := source.Plan(context.Background(), Codes())
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 2 || plans[0].Code != "sergipano-a1" || plans[0].Year != 2026 || plans[1].Code != "sergipano-a3" || plans[1].Year != 2025 {
		t.Fatalf("plans = %+v", plans)
	}
	snap, err := source.Fetch(context.Background(), "sergipano-a1", 2026, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if snap.Source != domain.SourceFSFSE || snap.SeasonExternalID != "sergipano-a1-2026" || snap.CompetitionName != "Sergipano A1" || len(snap.Matches) != 1 || len(snap.Standings) != 2 {
		t.Fatalf("snap = %+v", snap)
	}
	if snap.Matches[0].Status != domain.StatusFinished || *snap.Matches[0].HomeScore != 0 || snap.Matches[0].Phase != "Fase Classificatória" {
		t.Fatalf("match = %+v", snap.Matches[0])
	}
	if snap.Standings[1].GoalDifference != -3 || snap.Teams[0].State != "SE" {
		t.Fatalf("standing = %+v team = %+v", snap.Standings[1], snap.Teams[0])
	}
	for _, path := range paths {
		if path != "/campeonatos/" && path != "/campeonatos/sergipano-a1-2026/" {
			t.Fatalf("path = %s", path)
		}
	}
	if err := snap.Validate(Limits()); err == nil {
		t.Fatal("one match passed the eight-match limit")
	}
}

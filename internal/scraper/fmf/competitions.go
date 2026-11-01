// Package fmf collects the adult professional competitions published by the
// Federação Mineira de Futebol. The division names are the ones on that catalog.
package fmf

import "github.com/czarist/api_brasileirao/internal/domain"

// Competition is one championship on the FMF public catalog.
// Division is the fixed d= query value on ProxJogos.aspx, not a season id.
type Competition struct {
	Code       string
	Name       string
	Division   string
	MinMatches int
	MinRows    int
	MaxRows    int
}

// The three adult professional divisions. FMF does not label them Série A, B, and C.
// Módulo I seasons counted on 2026-10-31 had 53 and 75 matches. Módulo II had 74.
// Segunda Divisão had 36. The floors reject a one-round fragment.
var catalog = []Competition{
	{Code: "mineiro-modulo-i", Name: "Campeonato Mineiro Módulo I", Division: "1", MinMatches: 30, MinRows: 8, MaxRows: 24},
	{Code: "mineiro-modulo-ii", Name: "Campeonato Mineiro Módulo II", Division: "2", MinMatches: 30, MinRows: 8, MaxRows: 24},
	{Code: "mineiro-segunda-divisao", Name: "Campeonato Mineiro Segunda Divisão", Division: "3", MinMatches: 20, MinRows: 8, MaxRows: 48},
}

// Lookup returns a supported competition.
func Lookup(code string) (Competition, bool) {
	for _, item := range catalog {
		if item.Code == code {
			return item, true
		}
	}
	return Competition{}, false
}

// Codes returns the catalog codes in menu order.
func Codes() []string {
	codes := make([]string, len(catalog))
	for i, item := range catalog {
		codes[i] = item.Code
	}
	return codes
}

// Limits is the completeness check for this competition.
func (c Competition) Limits() domain.Limits {
	return domain.Limits{MinMatches: c.MinMatches, MinStandings: c.MinRows, MaxStandings: c.MaxRows}
}

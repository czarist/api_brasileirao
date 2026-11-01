// Package fsfse collects the adult professional divisions from the public
// Sergipe federation catalog. Cups, women's, and youth competitions on the
// same host are not collected. The host is fixed. A3 is collected only in a
// year whose card the catalog publishes.
package fsfse

import "github.com/czarist/api_brasileirao/internal/domain"

const (
	host     = "fsf-se.com.br"
	source   = domain.SourceFSFSE
	location = "America/Maceio"
)

type division struct {
	code string
	name string
	key  string
}

var divisions = []division{
	{code: "sergipano-a1", name: "Sergipano A1", key: "a1"},
	{code: "sergipano-a2", name: "Sergipano A2", key: "a2"},
	{code: "sergipano-a3", name: "Sergipano A3", key: "a3"},
}

// Limits is the completeness check for one adult Sergipe division.
func Limits() domain.Limits {
	return domain.Limits{MinMatches: 8, MinStandings: 4, MaxStandings: 48}
}

// Codes returns the adult divisions this catalog can publish.
func Codes() []string {
	out := make([]string, len(divisions))
	for i, item := range divisions {
		out[i] = item.code
	}
	return out
}

// Lookup reports whether code is an adult Sergipe division.
func Lookup(code string) (string, bool) {
	item, ok := divisionByCode(code)
	if !ok {
		return "", false
	}
	return item.name, true
}

func divisionByCode(code string) (division, bool) {
	for _, item := range divisions {
		if item.code == code {
			return item, true
		}
	}
	return division{}, false
}

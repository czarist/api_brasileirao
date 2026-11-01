// Package fafam collects the adult professional divisions from the public
// Amazonas federation catalog. Women's, youth, and cup competitions on the
// same host are not collected. The host is fixed. There is no adult Série C.
// Years that publish "Campeonato Amazonense" without a série stay under that
// name.
package fafam

import "github.com/czarist/api_brasileirao/internal/domain"

const (
	host     = "fafamazonas.com.br"
	source   = domain.SourceFAFAM
	location = "America/Manaus"
)

type division struct {
	code string
	name string
	key  string
}

var divisions = []division{
	{code: "amazonense-serie-a", name: "Amazonense Série A", key: "a"},
	{code: "amazonense-serie-b", name: "Amazonense Série B", key: "b"},
	{code: "amazonense", name: "Campeonato Amazonense", key: "am"},
}

// Limits is the completeness check for one adult Amazonas division.
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

// Lookup reports whether code is an adult Amazonas division.
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

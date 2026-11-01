// Package fgfrs collects the adult professional divisions from the Rio Grande
// do Sul federation. The host is www.fgf.com.br. This is not the Goiás site
// fgf.esp.br. Copa FGF and Recopa Gaúcha are not collected. The published
// names are Gauchão, Gauchão Série A2, and Gauchão Série B.
package fgfrs

import "github.com/czarist/api_brasileirao/internal/domain"

const (
	host     = "www.fgf.com.br"
	source   = domain.SourceFGFRS
	location = "America/Sao_Paulo"
)

type division struct {
	id   string
	code string
	name string
}

var divisions = []division{
	{id: "23", code: "gauchao", name: "Gauchão"},
	{id: "24", code: "gauchao-serie-a2", name: "Gauchão Série A2"},
	{id: "25", code: "gauchao-serie-b", name: "Gauchão Série B"},
}

// Limits is the completeness check for one adult Gaúcho division.
func Limits() domain.Limits {
	return domain.Limits{MinMatches: 8, MinStandings: 4, MaxStandings: 48}
}

// Codes returns the adult divisions this catalog publishes.
func Codes() []string {
	out := make([]string, 0, len(divisions))
	for _, item := range divisions {
		out = append(out, item.code)
	}
	return out
}

// Lookup reports whether code is an adult Gaúcho division.
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

func allowedDivision(id string) bool {
	for _, item := range divisions {
		if item.id == id {
			return true
		}
	}
	return false
}

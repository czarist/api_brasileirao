// Package fbf collects the adult Baianão from the public FBF catalog.
// Women's, youth, and other competitions on the same host are not collected.
// The host is fixed. There is no adult Série C in this catalog.
package fbf

import "github.com/czarist/api_brasileirao/internal/domain"

const (
	host      = "www.fbf.org.br"
	source    = domain.SourceFBF
	location  = "America/Bahia"
	divisionA = "1"
	divisionB = "2"
	codeA     = "baiano-serie-a"
	nameA     = "Baianão Série A"
	codeB     = "baiano-serie-b"
	nameB     = "Baianão Série B"
)

// Limits is the completeness check for one adult Baianão division.
func Limits() domain.Limits {
	return domain.Limits{MinMatches: 8, MinStandings: 4, MaxStandings: 48}
}

// Codes returns the adult divisions this catalog publishes.
func Codes() []string {
	return []string{codeA, codeB}
}

// Lookup reports whether code is an adult Baianão division.
func Lookup(code string) (string, bool) {
	switch code {
	case codeA:
		return nameA, true
	case codeB:
		return nameB, true
	default:
		return "", false
	}
}

func divisionOf(code string) (string, bool) {
	switch code {
	case codeA:
		return divisionA, true
	case codeB:
		return divisionB, true
	default:
		return "", false
	}
}

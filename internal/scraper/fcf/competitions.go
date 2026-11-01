// Package fcf collects the adult Campeonato Catarinense from the public FCF catalog.
// Copa Santa Catarina, Recopa, women's, and youth competitions on the same host
// are not collected. The host is fixed. The catalog publishes Série A, Série B,
// and Série C.
package fcf

import "github.com/czarist/api_brasileirao/internal/domain"

const (
	host     = "fcf.com.br"
	source   = domain.SourceFCF
	location = "America/Sao_Paulo"
	codeA    = "catarinense-serie-a"
	nameA    = "Campeonato Catarinense Série A"
	codeB    = "catarinense-serie-b"
	nameB    = "Campeonato Catarinense Série B"
	codeC    = "catarinense-serie-c"
	nameC    = "Campeonato Catarinense Série C"
)

// Limits is the completeness check for one adult Catarinense division.
func Limits() domain.Limits {
	// Série C 2025 publishes three clubs. Two is still a table. One row is not.
	return domain.Limits{MinMatches: 8, MinStandings: 2, MaxStandings: 48}
}

// Codes returns the adult divisions this catalog publishes.
func Codes() []string {
	return []string{codeA, codeB, codeC}
}

// Lookup reports whether code is an adult Catarinense division.
func Lookup(code string) (string, bool) {
	switch code {
	case codeA:
		return nameA, true
	case codeB:
		return nameB, true
	case codeC:
		return nameC, true
	default:
		return "", false
	}
}

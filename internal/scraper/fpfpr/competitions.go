// Package fpfpr collects the adult professional divisions from the public
// Paraná federation catalog. Copa Paraná on the same host is not collected.
// The host is fixed. The catalog publishes Paranaense, Segundona, and
// Terceirona. It does not publish those divisions under Série A, B, and C.
package fpfpr

import "github.com/czarist/api_brasileirao/internal/domain"

const (
	host     = "federacaopr.com.br"
	source   = domain.SourceFPFPR
	location = "America/Sao_Paulo"
	codeA    = "paranaense"
	nameA    = "Paranaense"
	codeB    = "paranaense-segundona"
	nameB    = "Segundona"
	codeC    = "paranaense-terceirona"
	nameC    = "Terceirona"
)

// Limits is the completeness check for one adult Paraná division.
func Limits() domain.Limits {
	return domain.Limits{MinMatches: 8, MinStandings: 4, MaxStandings: 48}
}

// Codes returns the adult divisions this catalog publishes.
func Codes() []string {
	return []string{codeA, codeB, codeC}
}

// Lookup reports whether code is an adult Paraná division.
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

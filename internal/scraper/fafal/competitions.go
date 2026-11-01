// Package fafal collects the adult Alagoano from the public FAF catalog.
// Youth, women's, and cup pages are skipped. The host is fixed.
package fafal

import "github.com/czarist/api_brasileirao/internal/domain"

const (
	host     = "www.futeboldealagoas.net"
	source   = domain.SourceFAFAL
	location = "America/Maceio"
	codeA    = "alagoano-serie-a"
	nameA    = "Alagoano Série A"
	codeB    = "alagoano-serie-b"
	nameB    = "Alagoano Série B"
)

// Limits is the completeness check for the adult first division.
func Limits() domain.Limits {
	return domain.Limits{MinMatches: 8, MinStandings: 4, MaxStandings: 48}
}

// Codes returns the divisions this catalog can publish.
// Série B is included only for years whose own card names it without a youth mark.
func Codes() []string {
	return []string{codeA, codeB}
}

// Lookup reports whether code is an adult Alagoano division.
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

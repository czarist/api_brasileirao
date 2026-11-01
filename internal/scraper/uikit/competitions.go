// Package uikit collects adult professional state championships from the
// public UIKit catalogs used by some federations. Each site stays on its own
// host. Youth, women's, amateur, and cup pages are skipped.
package uikit

import (
	"regexp"
	"strings"

	"github.com/czarist/api_brasileirao/internal/domain"
)

// Division is one adult professional championship on a federation catalog.
type Division struct {
	Code       string
	Name       string
	MinMatches int
	MinRows    int
	MaxRows    int
	match      func(folded string) bool
}

// Site is one federation host and the divisions its catalog may publish.
type Site struct {
	Source    string
	Host      string
	Location  string
	IndexPath string
	Divisions []Division
}

var (
	serieA = regexp.MustCompile(`SERIE A(?:[^0-9]|$)`)
	serieB = regexp.MustCompile(`SERIE B(?:[^0-9]|$)`)
)

// Sites is the fixed allowlist. A host is added only after its public catalog
// was read. SOURCE_BASE_URL is not used.
func Sites() []Site {
	loose := limits(8, 4, 48)
	return []Site{
		{
			Source:    domain.SourceFMFMA,
			Host:      "www.futebolmaranhense.com.br",
			Location:  "America/Fortaleza",
			IndexPath: "/competicoes/",
			Divisions: []Division{
				div("maranhense-serie-a", "Campeonato Maranhense Série A", loose, func(folded string) bool {
					return strings.Contains(folded, "MARANHENSE") && serieA.MatchString(folded)
				}),
				div("maranhense-serie-b", "Campeonato Maranhense Série B", loose, func(folded string) bool {
					return strings.Contains(folded, "MARANHENSE") && serieB.MatchString(folded)
				}),
			},
		},
		{
			Source:    domain.SourceFMFMT,
			Host:      "www.fmfmt.com.br",
			Location:  "America/Cuiaba",
			IndexPath: "/pt/competicoes/",
			Divisions: []Division{
				div("mato-grossense-primeira-divisao", "Mato-Grossense 1ª Divisão", loose, func(folded string) bool {
					return matoGrossense(folded) && firstDivision(folded)
				}),
				div("mato-grossense-segunda-divisao", "Mato-Grossense II Divisão", loose, func(folded string) bool {
					return matoGrossense(folded) && secondDivision(folded)
				}),
			},
		},
	}
}

func div(code, name string, bounds domain.Limits, match func(string) bool) Division {
	return Division{Code: code, Name: name, MinMatches: bounds.MinMatches, MinRows: bounds.MinStandings, MaxRows: bounds.MaxStandings, match: match}
}

func limits(minMatches, minRows, maxRows int) domain.Limits {
	return domain.Limits{MinMatches: minMatches, MinStandings: minRows, MaxStandings: maxRows}
}

// Lookup returns a division and the site that publishes it.
func Lookup(code string) (Site, Division, bool) {
	for _, site := range Sites() {
		for _, item := range site.Divisions {
			if item.Code == code {
				return site, item, true
			}
		}
	}
	return Site{}, Division{}, false
}

// Codes returns every division in site order.
func Codes() []string {
	var codes []string
	for _, site := range Sites() {
		for _, item := range site.Divisions {
			codes = append(codes, item.Code)
		}
	}
	return codes
}

// Limits is the completeness check for this division.
func (d Division) Limits() domain.Limits {
	return domain.Limits{MinMatches: d.MinMatches, MinStandings: d.MinRows, MaxStandings: d.MaxRows}
}

// Classify maps a published card title to a division of this site.
func (s Site) Classify(title string) (Division, bool) {
	folded := fold(title)
	if skipped(folded) {
		return Division{}, false
	}
	for _, item := range s.Divisions {
		if item.match(folded) {
			return item, true
		}
	}
	return Division{}, false
}

func matoGrossense(folded string) bool {
	return strings.Contains(folded, "MATO-GROSSENSE") || strings.Contains(folded, "MATO GROSSENSE")
}

func secondDivision(folded string) bool {
	return strings.Contains(folded, "II DIVIS") || strings.Contains(folded, "2 DIVIS") || strings.Contains(folded, "SEGUNDA")
}

func firstDivision(folded string) bool {
	if secondDivision(folded) {
		return false
	}
	return strings.Contains(folded, "1 DIVIS") || strings.Contains(folded, "PRIMEIRA DIVIS")
}

func skipped(folded string) bool {
	for _, word := range []string{"SUB", "FEMININ", "BASE", "INFANTIL", "MASTER", "AMADOR", "COPA", "TACA", "TORNEIO", "AMISTOSO", "TREINAMENTO"} {
		if strings.Contains(folded, word) {
			return true
		}
	}
	return false
}

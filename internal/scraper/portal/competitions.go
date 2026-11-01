// Package portal collects adult professional state championships from the
// public competition catalog shared by several federations. Each site stays
// on its own host. Youth, women's, amateur, and cup pages are skipped.
package portal

import (
	"regexp"
	"strings"

	"github.com/czarist/api_brasileirao/internal/domain"
)

var (
	serieA   = regexp.MustCompile(`SERIE A(?:[^0-9]|$)`)
	serieB   = regexp.MustCompile(`SERIE B(?:[^0-9]|$)`)
	division = regexp.MustCompile(`(?:^|[^0-9])([123]) DIVIS`)
	levelA   = regexp.MustCompile(`(?:^|[^A-Z0-9])A([123])(?:[^A-Z0-9]|$)`)
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
	Divisions []Division
}

// Sites is the fixed allowlist. A host is added only after its public catalog
// was read. SOURCE_BASE_URL is not used.
func Sites() []Site {
	loose := limits(8, 4, 40)
	return []Site{
		{
			Source: domain.SourceFFDF,
			Host:   "www.ffdf.com.br",
			Divisions: []Division{
				div("candangao-serie-a", "Candangão Série A", loose, func(folded string) bool {
					return strings.Contains(folded, "CANDANG") && serieA.MatchString(folded) && !serieB.MatchString(folded)
				}),
				div("candangao-serie-b", "Candangão Série B", loose, func(folded string) bool {
					return strings.Contains(folded, "CANDANG") && serieB.MatchString(folded)
				}),
			},
		},
		{
			Source: domain.SourceFGFGO,
			Host:   "fgf.esp.br",
			Divisions: []Division{
				div("goiano-primeira-divisao", "Campeonato Goiano 1ª Divisão", loose, func(folded string) bool {
					return strings.Contains(folded, "GOIANO") && strings.Contains(folded, "PROFISSION") && divisionNumber(folded) == "1"
				}),
				div("goiano-divisao-de-acesso", "Campeonato Goiano Divisão de Acesso", loose, func(folded string) bool {
					return strings.Contains(folded, "GOIANO") && (strings.Contains(folded, "ACESSO") || divisionNumber(folded) == "2") && divisionNumber(folded) != "3"
				}),
				div("goiano-terceira-divisao", "Campeonato Goiano 3ª Divisão", loose, func(folded string) bool {
					return strings.Contains(folded, "GOIANO") && strings.Contains(folded, "PROFISSION") && divisionNumber(folded) == "3"
				}),
			},
		},
		{
			Source: domain.SourceFPFPE,
			Host:   "www.fpf-pe.com.br",
			Divisions: []Division{
				div("pernambucano-a1", "Campeonato Pernambucano A1", loose, func(folded string) bool {
					return strings.Contains(folded, "PERNAMBUCANO") && level(folded) == "1"
				}),
				div("pernambucano-a2", "Campeonato Pernambucano A2", loose, func(folded string) bool {
					return strings.Contains(folded, "PERNAMBUCANO") && level(folded) == "2"
				}),
				div("pernambucano-a3", "Campeonato Pernambucano A3", loose, func(folded string) bool {
					return strings.Contains(folded, "PERNAMBUCANO") && level(folded) == "3"
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
// Titles for youth, women, amateurs, and cups are skipped.
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

func divisionNumber(folded string) string {
	found := division.FindStringSubmatch(folded)
	if found == nil {
		return ""
	}
	return found[1]
}

func level(folded string) string {
	found := levelA.FindStringSubmatch(folded)
	if found == nil {
		return ""
	}
	return found[1]
}

func skipped(folded string) bool {
	for _, word := range []string{"SUB", "FEMININ", "BASE", "INFANTIL", "MASTER", "AMADOR", "COPA", "TACA", "TORNEIO", "AMISTOSO"} {
		if strings.Contains(folded, word) {
			return true
		}
	}
	return false
}

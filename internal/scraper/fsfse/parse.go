package fsfse

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/czarist/api_brasileirao/internal/domain"
	"golang.org/x/net/html"
)

// SeasonRef is one adult card on the championship index.
type SeasonRef struct {
	Code string
	Name string
	Key  string
	Year int
	Slug string
}

type club struct {
	Name  string
	Short string
}

type page struct {
	matches []domain.MatchInput
	tables  [][]domain.StandingInput
	clubs   map[string]club
}

var (
	rodadaRE = regexp.MustCompile(`(?i)RODADA\s+(\d{1,2})`)
	groupRE  = regexp.MustCompile(`^f(\d+)(?:r(\d+))?$`)
	scoreRE  = regexp.MustCompile(`(?i)(\d+)\s*[-x]\s*(\d+)`)
	clockRE  = regexp.MustCompile(`(\d{1,2}):(\d{2})`)
	dateRE   = regexp.MustCompile(`(\d{1,2})\s+DE\s+([A-Z]+)\s+DE\s+(\d{4})`)
	yearRE   = regexp.MustCompile(`\b(19|20)\d{2}\b`)
	jogoRE   = regexp.MustCompile(`^/jogo/(\d+)(?:/|$)`)
)

var errSkipMatch = errors.New("match is not stored for this season")

var months = map[string]time.Month{
	"JANEIRO":   time.January,
	"FEVEREIRO": time.February,
	"MARCO":     time.March,
	"ABRIL":     time.April,
	"MAIO":      time.May,
	"JUNHO":     time.June,
	"JULHO":     time.July,
	"AGOSTO":    time.August,
	"SETEMBRO":  time.September,
	"OUTUBRO":   time.October,
	"NOVEMBRO":  time.November,
	"DEZEMBRO":  time.December,
}

// ParseCatalog reads adult division cards. Cups, women's, and youth cards
// are ignored. A link that is not a championship card is ignored.
func ParseCatalog(body string) ([]SeasonRef, error) {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	var refs []SeasonRef
	seen := map[string]string{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if err != nil || n == nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "camp-card") {
			ref, ok := catalogCard(attr(n, "href"))
			if ok {
				key := ref.Code + "/" + strconv.Itoa(ref.Year)
				if prev, dup := seen[key]; dup && prev != ref.Slug {
					err = fmt.Errorf("%s %d is listed as %s and %s", ref.Code, ref.Year, prev, ref.Slug)
					return
				}
				if _, dup := seen[key]; !dup {
					seen[key] = ref.Slug
					refs = append(refs, ref)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return refs, err
}

func catalogCard(raw string) (SeasonRef, bool) {
	parts := pathParts(raw)
	if len(parts) != 2 || parts[0] != "campeonatos" {
		return SeasonRef{}, false
	}
	item, year, ok := parseSlug(parts[1])
	if !ok {
		return SeasonRef{}, false
	}
	return SeasonRef{Code: item.code, Name: item.name, Key: item.key, Year: year, Slug: parts[1]}, true
}

// ParseSeason reads one season page. Matches and every phase table are in
// the HTML. Cards and the win percentage are not published, so those fields
// stay 0. A match whose date names another year is omitted. A card with no
// jogo id is omitted. A group box title is the standings group. The same
// title in two phases is stored as the phase name plus that title.
func ParseSeason(body, divisionKey string, year int, loc *time.Location) (page, error) {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return page{}, fmt.Errorf("parse season: %w", err)
	}
	out := page{clubs: map[string]club{}}
	phases := map[string]string{}
	orders := map[string]int{}
	nextOrder := 0
	lastTitle := ""
	var named []namedTable
	var walkTables func(*html.Node)
	walkTables = func(n *html.Node) {
		if n == nil || err != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "h3" && hasClass(n, "fase-title") {
			if title := strings.TrimSpace(textOf(n)); title != "" {
				lastTitle = title
			}
			return
		}
		if hasClass(n, "classificacao-full") {
			id := strings.TrimSpace(attr(n, "data-fase-id"))
			title := lastTitle
			if id != "" && title != "" && !skipPhase(title) {
				phases[id] = title
				got, rowErr := parseClassification(n, title, orders, &nextOrder, out.clubs)
				if rowErr != nil {
					err = rowErr
					return
				}
				named = append(named, got...)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkTables(c)
		}
	}
	walkTables(root)
	if err != nil {
		return page{}, err
	}
	for _, table := range disambiguateGroups(named) {
		if len(table.rows) > 0 {
			out.tables = append(out.tables, table.rows)
		}
	}
	var walkMatches func(*html.Node)
	walkMatches = func(n *html.Node) {
		if n == nil || err != nil {
			return
		}
		if hasClass(n, "jogos-group") {
			if rowErr := parseGroup(n, divisionKey, year, loc, phases, &out); rowErr != nil {
				err = rowErr
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkMatches(c)
		}
	}
	walkMatches(root)
	if err != nil {
		return page{}, err
	}
	return out, nil
}

func parseGroup(group *html.Node, divisionKey string, year int, loc *time.Location, phases map[string]string, out *page) error {
	rawID := strings.TrimSpace(attr(group, "data-group"))
	found := groupRE.FindStringSubmatch(rawID)
	if found == nil {
		return fmt.Errorf("match group %q", rawID)
	}
	phase := phases[found[1]]
	title := ""
	round := 1
	if found[2] != "" {
		got, err := strconv.Atoi(found[2])
		if err != nil || got < 1 || got > 40 {
			return fmt.Errorf("round %q is out of range", found[2])
		}
		round = got
	}
	var pending string
	var walk func(*html.Node) error
	walk = func(n *html.Node) error {
		if n == nil {
			return nil
		}
		if n.Type == html.ElementNode {
			if hasClass(n, "jogos-group__title") {
				title = strings.TrimSpace(textOf(n))
				if found[2] == "" {
					if label := rodadaRE.FindStringSubmatch(title); label != nil {
						got, err := strconv.Atoi(label[1])
						if err != nil || got < 1 || got > 40 {
							return fmt.Errorf("round %q is out of range", label[1])
						}
						round = got
					}
				}
				return nil
			}
			if hasClass(n, "jogos-group__date") {
				pending = strings.TrimSpace(textOf(n))
				return nil
			}
			if hasClass(n, "match-panel") {
				name := phase
				if name == "" {
					name = title
				}
				if name == "" || skipPhase(name) || (phase == "" && skipPhase(title)) {
					return nil
				}
				if rodadaRE.MatchString(name) {
					return fmt.Errorf("match group %s has no phase", rawID)
				}
				match, err := parseMatch(n, divisionKey, year, name, round, pending, loc, out.clubs)
				if errors.Is(err, errSkipMatch) {
					return nil
				}
				if err != nil {
					return err
				}
				return addMatch(out, match)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(group)
}

func parseMatch(card *html.Node, divisionKey string, year int, phase string, round int, whenText string, loc *time.Location, clubs map[string]club) (domain.MatchInput, error) {
	if dated, ok := dateYear(whenText); ok && dated != year {
		return domain.MatchInput{}, errSkipMatch
	}
	var teams []clubSide
	var walkTeams func(*html.Node)
	walkTeams = func(n *html.Node) {
		if n == nil || len(teams) == 2 {
			return
		}
		if hasClass(n, "match-panel__team") {
			teams = append(teams, clubSide{
				name: strings.TrimSpace(textOf(findClass(n, "match-panel__name"))),
				id:   badgeID(attr(findTag(n, "img"), "src")),
			})
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkTeams(c)
		}
	}
	walkTeams(card)
	if len(teams) != 2 || teams[0].name == "" || teams[1].name == "" || teams[0].id == "" || teams[1].id == "" {
		return domain.MatchInput{}, fmt.Errorf("match is missing a club")
	}
	if teams[0].id == teams[1].id {
		return domain.MatchInput{}, fmt.Errorf("match lists %s twice", teams[0].name)
	}
	jogo, err := jogoID(card)
	if err != nil {
		return domain.MatchInput{}, err
	}
	when, err := kickoff(whenText, metaText(card), year, loc)
	if err != nil {
		return domain.MatchInput{}, fmt.Errorf("match %s: %w", jogo, err)
	}
	homeScore, awayScore, scored, err := scores(textOf(findClass(card, "match-panel__placar")))
	if err != nil {
		return domain.MatchInput{}, fmt.Errorf("match %s: %w", jogo, err)
	}
	number, err := strconv.Atoi(jogo)
	if err != nil {
		return domain.MatchInput{}, err
	}
	remember(clubs, teams[0].id, teams[0].name, teams[0].name)
	remember(clubs, teams[1].id, teams[1].name, teams[1].name)
	match := domain.MatchInput{
		ExternalID:     fmt.Sprintf("%s-%d-%s", divisionKey, year, jogo),
		MatchNumber:    number,
		RoundNumber:    round,
		Phase:          phase,
		HomeExternalID: teams[0].id,
		AwayExternalID: teams[1].id,
		Kickoff:        when,
		Venue:          venueFrom(card),
		State:          "SE",
	}
	if scored {
		match.Status = domain.StatusFinished
		match.HomeScore = homeScore
		match.AwayScore = awayScore
	} else {
		match.Status = domain.StatusScheduled
	}
	return match, nil
}

type clubSide struct {
	name string
	id   string
}

func parseTable(n *html.Node, group string, orders map[string]int, next *int, clubs map[string]club) ([]domain.StandingInput, error) {
	var rows []domain.StandingInput
	var rowErr error
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if rowErr != nil || node == nil {
			return
		}
		if node.Type == html.ElementNode && node.Data == "tr" && findClass(node, "table-class__pos") != nil {
			row, skip, err := parseStandingRow(node, group, orders, next, clubs, len(rows)+1)
			if err != nil {
				rowErr = err
				return
			}
			if skip {
				return
			}
			rows = append(rows, row)
			return
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	if rowErr != nil {
		return nil, rowErr
	}
	if len(rows) == 0 {
		return nil, nil
	}
	if strings.TrimSpace(group) == "" {
		return nil, fmt.Errorf("standings table has no phase title")
	}
	return rows, nil
}

func parseStandingRow(tr *html.Node, group string, orders map[string]int, next *int, clubs map[string]club, fallbackPos int) (domain.StandingInput, bool, error) {
	posText := strings.TrimSpace(textOf(findClass(tr, "table-class__pos")))
	pos, err := strconv.Atoi(digitsOnly(posText))
	if posText == "" || digitsOnly(posText) == "" {
		pos = fallbackPos
		err = nil
	}
	if err != nil || pos <= 0 {
		return domain.StandingInput{}, false, fmt.Errorf("standing position %q", posText)
	}
	clubNode := findClass(tr, "table-class__clube")
	name := strings.TrimSpace(textOf(clubNode))
	if name == "" {
		name = strings.TrimSpace(attr(findTag(clubNode, "img"), "alt"))
	}
	id := badgeID(attr(findTag(clubNode, "img"), "src"))
	if name == "" || id == "" {
		return domain.StandingInput{}, false, fmt.Errorf("standing has no club")
	}
	var stats []string
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || c.Data != "td" {
			continue
		}
		if hasClass(c, "table-class__pos") || findClass(c, "table-class__clube") != nil {
			continue
		}
		stats = append(stats, strings.TrimSpace(textOf(c)))
	}
	if len(stats) != 8 {
		return domain.StandingInput{}, false, fmt.Errorf("standing %s has %d figures", name, len(stats))
	}
	filled := 0
	values := make([]int, 8)
	for i, raw := range stats {
		if raw == "" {
			continue
		}
		filled++
		values[i], err = strconv.Atoi(strings.TrimPrefix(strings.ReplaceAll(raw, " ", ""), "+"))
		if err != nil {
			return domain.StandingInput{}, false, fmt.Errorf("standing %s %q: %w", name, raw, err)
		}
	}
	if filled == 0 {
		return domain.StandingInput{}, true, nil
	}
	if _, ok := orders[group]; !ok {
		orders[group] = *next
		*next++
	}
	row := domain.StandingInput{
		ExternalTeamID: id,
		GroupName:      group,
		GroupOrder:     orders[group],
		Position:       pos,
		Played:         values[0],
		Wins:           values[1],
		Draws:          values[2],
		Losses:         values[3],
		GoalsFor:       values[4],
		GoalsAgainst:   values[5],
		GoalDifference: values[6],
		Points:         values[7],
	}
	remember(clubs, id, name, name)
	return row, false, nil
}

func addMatch(out *page, match domain.MatchInput) error {
	for _, prev := range out.matches {
		if prev.ExternalID != match.ExternalID {
			continue
		}
		if prev.RoundNumber == match.RoundNumber && prev.Phase == match.Phase && prev.HomeExternalID == match.HomeExternalID && prev.AwayExternalID == match.AwayExternalID && prev.Status == match.Status && prev.Venue == match.Venue && sameScore(prev.HomeScore, match.HomeScore) && sameScore(prev.AwayScore, match.AwayScore) && sameTime(prev.Kickoff, match.Kickoff) {
			return nil
		}
		return fmt.Errorf("match %s is published twice", match.ExternalID)
	}
	out.matches = append(out.matches, match)
	return nil
}

func scores(text string) (*int, *int, bool, error) {
	text = strings.TrimSpace(text)
	folded := fold(text)
	if text == "" || folded == "X" || text == "-" || folded == "VS" {
		return nil, nil, false, nil
	}
	found := scoreRE.FindStringSubmatch(text)
	if found == nil {
		return nil, nil, false, fmt.Errorf("score %q", text)
	}
	home, err := strconv.Atoi(found[1])
	if err != nil {
		return nil, nil, false, err
	}
	away, err := strconv.Atoi(found[2])
	if err != nil {
		return nil, nil, false, err
	}
	return &home, &away, true, nil
}

func kickoff(dateText, meta string, year int, loc *time.Location) (*time.Time, error) {
	found := dateRE.FindStringSubmatch(fold(dateText))
	if found == nil {
		return nil, nil
	}
	day, _ := strconv.Atoi(found[1])
	month, ok := months[found[2]]
	if !ok {
		return nil, fmt.Errorf("date %q", dateText)
	}
	gotYear, _ := strconv.Atoi(found[3])
	if gotYear != year {
		return nil, errSkipMatch
	}
	clock := clockRE.FindStringSubmatch(meta)
	if clock == nil {
		return nil, nil
	}
	hour, _ := strconv.Atoi(clock[1])
	minute, _ := strconv.Atoi(clock[2])
	if hour > 23 || minute > 59 || day < 1 || day > 31 {
		return nil, fmt.Errorf("kickoff %s %s", dateText, meta)
	}
	when := time.Date(year, month, day, hour, minute, 0, 0, loc)
	if when.Day() != day || when.Month() != month || when.Year() != year {
		return nil, fmt.Errorf("kickoff %s", dateText)
	}
	utc := when.UTC()
	return &utc, nil
}

func dateYear(raw string) (int, bool) {
	found := yearRE.FindString(raw)
	if found == "" {
		return 0, false
	}
	year, err := strconv.Atoi(found)
	if err != nil {
		return 0, false
	}
	return year, true
}

func jogoID(card *html.Node) (string, error) {
	var id string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if id != "" || n == nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" {
			parsed, err := url.Parse(strings.TrimSpace(attr(n, "href")))
			if err == nil {
				if found := jogoRE.FindStringSubmatch(parsed.Path); found != nil {
					id = found[1]
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(card)
	if id == "" {
		return "", errSkipMatch
	}
	return id, nil
}

type namedTable struct {
	phase string
	rows  []domain.StandingInput
}

func parseClassification(n *html.Node, phase string, orders map[string]int, next *int, clubs map[string]club) ([]namedTable, error) {
	boxes := findAllClass(n, "grupo-box")
	if len(boxes) == 0 {
		rows, err := parseTable(n, phase, orders, next, clubs)
		if err != nil || len(rows) == 0 {
			return nil, err
		}
		return []namedTable{{phase: phase, rows: rows}}, nil
	}
	used := map[string]int{}
	out := make([]namedTable, 0, len(boxes))
	for _, box := range boxes {
		label := strings.TrimSpace(textOf(findClass(box, "grupo-box__titulo")))
		if label == "" {
			label = phase
		}
		used[label]++
		if used[label] > 1 {
			label = fmt.Sprintf("%s %d", label, used[label])
		}
		rows, err := parseTable(box, label, orders, next, clubs)
		if err != nil {
			return nil, err
		}
		if len(rows) > 0 {
			out = append(out, namedTable{phase: phase, rows: rows})
		}
	}
	return out, nil
}

// disambiguateGroups keeps a box title when it belongs to one phase. The
// same title in a later phase is stored with the phase name in front, so
// position 1 can exist in both.
func disambiguateGroups(tables []namedTable) []namedTable {
	phases := map[string]map[string]struct{}{}
	for _, table := range tables {
		if len(table.rows) == 0 {
			continue
		}
		name := table.rows[0].GroupName
		if phases[name] == nil {
			phases[name] = map[string]struct{}{}
		}
		phases[name][table.phase] = struct{}{}
	}
	for i, table := range tables {
		if len(table.rows) == 0 {
			continue
		}
		name := table.rows[0].GroupName
		if len(phases[name]) < 2 || name == table.phase {
			continue
		}
		qualified := table.phase + " - " + name
		for j := range tables[i].rows {
			tables[i].rows[j].GroupName = qualified
		}
	}
	return tables
}

func findAllClass(n *html.Node, class string) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil {
			return
		}
		if node != n && hasClass(node, class) {
			out = append(out, node)
			return
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

func venueFrom(card *html.Node) string {
	meta := findClass(card, "match-panel__meta")
	var venue string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil || venue != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "span" {
			text := strings.TrimSpace(textOf(n))
			if text != "" && clockRE.FindString(text) == "" {
				venue = cleanLabel(text)
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(meta)
	return venue
}

func metaText(card *html.Node) string {
	return textOf(findClass(card, "match-panel__meta"))
}

func badgeID(src string) string {
	parsed, err := url.Parse(strings.TrimSpace(src))
	if err != nil {
		return ""
	}
	base := path.Base(parsed.Path)
	stem := strings.TrimSuffix(base, path.Ext(base))
	stem = strings.ToLower(strings.TrimSpace(stem))
	if stem == "" || stem == "." || strings.Contains(stem, "/") {
		return ""
	}
	return stem
}

func skipPhase(name string) bool {
	folded := fold(name)
	for _, skip := range []string{"COPA", "FEMININ", "SUB", "BASE", "AMADOR", "MASTER", "INFANTIL"} {
		if strings.Contains(folded, skip) {
			return true
		}
	}
	return false
}

func remember(clubs map[string]club, id, name, short string) {
	if id == "" || name == "" {
		return
	}
	if _, ok := clubs[id]; ok {
		return
	}
	if short == "" {
		short = name
	}
	clubs[id] = club{Name: name, Short: short}
}

func sameScore(a, b *int) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	return a == nil || *a == *b
}

func sameTime(a, b *time.Time) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	return a == nil || a.Equal(*b)
}

func digitsOnly(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func cleanLabel(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimLeftFunc(raw, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	return strings.TrimSpace(raw)
}

func pathParts(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	trimmed := strings.Trim(parsed.Path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func fold(raw string) string {
	replacer := strings.NewReplacer(
		"Á", "A", "À", "A", "Â", "A", "Ã", "A",
		"É", "E", "Ê", "E",
		"Í", "I",
		"Ó", "O", "Ô", "O", "Õ", "O",
		"Ú", "U",
		"Ç", "C",
		"á", "A", "à", "A", "â", "A", "ã", "A",
		"é", "E", "ê", "E",
		"í", "I",
		"ó", "O", "ô", "O", "õ", "O",
		"ú", "U",
		"ç", "C",
	)
	return strings.ToUpper(replacer.Replace(raw))
}

func hasClass(n *html.Node, class string) bool {
	if n == nil || n.Type != html.ElementNode {
		return false
	}
	for _, part := range strings.Fields(attr(n, "class")) {
		if part == class {
			return true
		}
	}
	return false
}

func findClass(n *html.Node, class string) *html.Node {
	if n == nil {
		return nil
	}
	if hasClass(n, class) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findClass(c, class); found != nil {
			return found
		}
	}
	return nil
}

func findTag(n *html.Node, tag string) *html.Node {
	if n == nil {
		return nil
	}
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findTag(c, tag); found != nil {
			return found
		}
	}
	return nil
}

func attr(n *html.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, item := range n.Attr {
		if item.Key == key {
			return item.Val
		}
	}
	return ""
}

func textOf(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

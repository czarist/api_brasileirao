package fafam

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
	"unicode/utf8"

	"github.com/czarist/api_brasileirao/internal/domain"
	"golang.org/x/net/html"
)

// SeasonRef is one adult card on the championship index.
type SeasonRef struct {
	Code   string
	Name   string
	Key    string
	Year   int
	Slug   string
	Round  string
	Prefer int
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
	roundRE       = regexp.MustCompile(`(?i)(\d{1,2})\s*[º°ªA]?\s*RODADA`)
	dateRE        = regexp.MustCompile(`(\d{2})/(\d{2})/(\d{4})(?:\s+(\d{2}):(\d{2}))?`)
	penRE         = regexp.MustCompile(`^(\d+)\s*-\s*(\d+)$`)
	parenRE       = regexp.MustCompile(`^(\d+)\s*\(\s*(\d+)\s*\)$`)
	plainRE       = regexp.MustCompile(`^(\d+)$`)
	championRE    = regexp.MustCompile(`(?i)(?:\s*-\s*\([^)]*\)|\s*\((?:vice[\s-]*)?campe[aã]o\))\s*$`)
	roleRE        = regexp.MustCompile(`(?i)\s*\((?:vc|vice(?:[\s-]*campe[aã]o)?|campe[aã]o|c)\)\s*$`)
	leadingDigits = regexp.MustCompile(`^\d+`)
)

var errSkipMatch = errors.New("match is not part of this season")

// ParseCatalog reads adult division cards from one index page.
// A year listed twice keeps the preferred slug.
func ParseCatalog(body string) ([]SeasonRef, error) {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	var refs []SeasonRef
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if err != nil || n == nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "link-corpo") {
			if ref, ok := catalogCard(attr(n, "href")); ok {
				refs, err = mergeRefs(refs, ref)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return refs, err
}

// CatalogPages reads the index pagination. The current page is included when linked.
func CatalogPages(body string) ([]int, error) {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse catalog pages: %w", err)
	}
	var pages []int
	seen := map[int]struct{}{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" {
			if page, ok := catalogPage(attr(n, "href")); ok {
				if _, dup := seen[page]; !dup {
					seen[page] = struct{}{}
					pages = append(pages, page)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return pages, nil
}

// RoundLinks reads the previous and next round links for one season slug.
func RoundLinks(body, slug string) ([]string, error) {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse rounds: %w", err)
	}
	var rounds []string
	seen := map[string]struct{}{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" {
			if round, ok := roundLink(attr(n, "href"), slug); ok {
				if _, dup := seen[round]; !dup {
					seen[round] = struct{}{}
					rounds = append(rounds, round)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return rounds, nil
}

func catalogCard(raw string) (SeasonRef, bool) {
	parts := pathParts(raw)
	if len(parts) == 4 && parts[0] == "site" {
		parts = parts[1:]
	}
	if len(parts) != 3 || parts[0] != "campeonato" || !digits.MatchString(parts[2]) {
		return SeasonRef{}, false
	}
	item, year, ok := classify(parts[1])
	if !ok {
		return SeasonRef{}, false
	}
	return SeasonRef{
		Code: item.code, Name: item.name, Key: item.key, Year: year,
		Slug: parts[1], Round: parts[2], Prefer: preferSlug(parts[1]),
	}, true
}

func catalogPage(raw string) (int, bool) {
	parts := pathParts(raw)
	if len(parts) == 3 && parts[0] == "site" {
		parts = parts[1:]
	}
	if len(parts) != 2 || parts[0] != "campeonatos" || !digits.MatchString(parts[1]) {
		return 0, false
	}
	page, err := strconv.Atoi(parts[1])
	if err != nil || page < 1 || page > 20 {
		return 0, false
	}
	return page, true
}

func roundLink(raw, slug string) (string, bool) {
	parts := pathParts(raw)
	if len(parts) == 4 && parts[0] == "site" {
		parts = parts[1:]
	}
	if len(parts) != 3 || parts[0] != "campeonato" || parts[1] != slug || !digits.MatchString(parts[2]) {
		return "", false
	}
	return parts[2], true
}

func mergeRefs(refs []SeasonRef, ref SeasonRef) ([]SeasonRef, error) {
	for i, prev := range refs {
		if prev.Code != ref.Code || prev.Year != ref.Year {
			continue
		}
		if prev.Slug == ref.Slug {
			return refs, nil
		}
		if prev.Prefer == ref.Prefer {
			return nil, fmt.Errorf("%s %d is listed as %s and %s", ref.Code, ref.Year, prev.Slug, ref.Slug)
		}
		if ref.Prefer < prev.Prefer {
			refs[i] = ref
		}
		return refs, nil
	}
	return append(refs, ref), nil
}

// ParseRound reads one round page. Knockout blocks repeat on every round, and
// a finished fixture is sometimes printed under two headings. The caller keeps
// the first copy when the score and the kickoff agree. Cards and the win
// percentage are not published. A goals-penalties cell such as 0-5 stores the
// goals and the penalty score. A club image that never appears with a name is
// omitted.
func ParseRound(body, divisionKey string, year int, loc *time.Location) (page, error) {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return page{}, fmt.Errorf("parse round: %w", err)
	}
	out := page{clubs: map[string]club{}}
	names := map[string]string{}
	orders := map[string]int{}
	nextOrder := 0
	phase := ""
	section := ""
	group := ""
	roundLabel := ""
	var walkTables func(*html.Node)
	walkTables = func(n *html.Node) {
		if n == nil || err != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "h1" {
			title := repairText(strings.TrimSpace(textOf(n)))
			if title != "" && !skipPhase(title) {
				if strings.HasPrefix(fold(title), "GRUPO") {
					group = groupTitle(section, title)
				} else if hasClass(n, "titulo") {
					phase = title
					group = ""
					if strings.Contains(fold(title), "TURNO") {
						section = title
					}
				}
			}
			return
		}
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "titulo") {
			title := repairText(strings.TrimSpace(textOf(n)))
			if title != "" && strings.Contains(fold(title), "GRUPO") {
				group = groupTitle(section, title)
			}
			return
		}
		if n.Type == html.ElementNode && n.Data == "table" && hasClass(n, "tabela-campeonato-interna") {
			title := group
			if title == "" {
				title = phase
			}
			rows, rowErr := parseTable(n, title, names, orders, &nextOrder, out.clubs)
			if rowErr != nil {
				err = rowErr
				return
			}
			if len(rows) > 0 {
				out.tables = append(out.tables, rows)
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
	aliasClubIDs(&out, names)
	phase = ""
	roundLabel = ""
	var walkMatches func(*html.Node)
	walkMatches = func(n *html.Node) {
		if n == nil || err != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "h1" {
			title := repairText(strings.TrimSpace(textOf(n)))
			if title != "" && hasClass(n, "titulo") && !skipPhase(title) && !strings.HasPrefix(fold(title), "GRUPO") {
				phase = title
			}
		}
		if hasClass(n, "rodada") && n.Data == "div" {
			if title := strings.TrimSpace(textOf(n)); title != "" {
				roundLabel = title
			}
		}
		if n.Data == "ul" && (hasClass(n, "rodada-jogos") || hasClass(n, "rodada-jogos-final") || hasClass(n, "rodada-jogo-final")) {
			round := 1
			if found := roundRE.FindStringSubmatch(roundLabel); found != nil {
				got, convErr := strconv.Atoi(found[1])
				if convErr != nil || got < 1 || got > 40 {
					err = fmt.Errorf("round %q is out of range", found[1])
					return
				}
				round = got
			}
			name := phase
			if name == "" {
				name = strings.TrimSpace(roundLabel)
			}
			if name != "" && !skipPhase(name) && !roundRE.MatchString(name) {
				if rowErr := parseMatchList(n, divisionKey, year, name, round, loc, names, &out); rowErr != nil {
					err = rowErr
					return
				}
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

func parseMatchList(list *html.Node, divisionKey string, year int, phase string, round int, loc *time.Location, names map[string]string, out *page) error {
	var walk func(*html.Node) error
	walk = func(n *html.Node) error {
		if n == nil {
			return nil
		}
		if n.Type == html.ElementNode && n.Data == "li" {
			match, err := parseMatch(n, divisionKey, year, phase, round, loc, names, out.clubs)
			if errors.Is(err, errSkipMatch) {
				return nil
			}
			if err != nil {
				return err
			}
			return addMatch(out, match)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(list)
}

func parseMatch(li *html.Node, divisionKey string, year int, phase string, round int, loc *time.Location, names map[string]string, clubs map[string]club) (domain.MatchInput, error) {
	whenText := textOf(findClass(li, "data"))
	when, dateStamp, err := kickoff(whenText, year, loc)
	if errors.Is(err, errSkipMatch) {
		return domain.MatchInput{}, err
	}
	if err != nil {
		return domain.MatchInput{}, err
	}
	homeImg := attr(findTag(findClass(li, "time1"), "img"), "src")
	awayImg := attr(findTag(findClass(li, "time2"), "img"), "src")
	homeName, homeOK := clubFromImage(homeImg, names, clubs)
	awayName, awayOK := clubFromImage(awayImg, names, clubs)
	if !homeOK || !awayOK {
		return domain.MatchInput{}, errSkipMatch
	}
	homeID := clubID(homeName)
	awayID := clubID(awayName)
	if homeID == "" || awayID == "" {
		return domain.MatchInput{}, fmt.Errorf("match is missing a club")
	}
	if homeID == awayID {
		return domain.MatchInput{}, fmt.Errorf("match lists %s twice", homeName)
	}
	homeRaw, awayRaw := scorePair(li)
	homeGoals, homePens, homeScored, err := sideScore(homeRaw)
	if err != nil {
		return domain.MatchInput{}, err
	}
	awayGoals, awayPens, awayScored, err := sideScore(awayRaw)
	if err != nil {
		return domain.MatchInput{}, err
	}
	if homeScored != awayScored || (homePens == nil) != (awayPens == nil) {
		return domain.MatchInput{}, fmt.Errorf("match %s x %s has one score", homeName, awayName)
	}
	remember(clubs, homeID, homeName)
	remember(clubs, awayID, awayName)
	if dateStamp == "" {
		dateStamp = "nodate"
	}
	match := domain.MatchInput{
		ExternalID:     fmt.Sprintf("%s-%d-%s-%s-%s", divisionKey, year, dateStamp, homeID, awayID),
		RoundNumber:    round,
		Phase:          phase,
		HomeExternalID: homeID,
		AwayExternalID: awayID,
		Kickoff:        when,
		Venue:          venueFrom(whenText),
		State:          "AM",
	}
	if homeScored {
		match.Status = domain.StatusFinished
		match.HomeScore = homeGoals
		match.AwayScore = awayGoals
		match.HomePenalties = homePens
		match.AwayPenalties = awayPens
	} else {
		match.Status = domain.StatusScheduled
	}
	return match, nil
}

func scorePair(li *html.Node) (string, string) {
	var scores []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil || len(scores) == 2 {
			return
		}
		if hasClass(n, "resultado") {
			scores = append(scores, strings.TrimSpace(textOf(n)))
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(li)
	if len(scores) == 0 {
		return "", ""
	}
	if len(scores) == 1 {
		return scores[0], ""
	}
	return scores[0], scores[1]
}

func sideScore(raw string) (*int, *int, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "-" || strings.EqualFold(raw, "x") {
		return nil, nil, false, nil
	}
	if strings.Count(raw, "(") > 1 {
		return nil, nil, false, errSkipMatch
	}
	if found := parenRE.FindStringSubmatch(raw); found != nil {
		goals, err := strconv.Atoi(found[1])
		if err != nil {
			return nil, nil, false, err
		}
		pens, err := strconv.Atoi(found[2])
		if err != nil {
			return nil, nil, false, err
		}
		return &goals, &pens, true, nil
	}
	if found := penRE.FindStringSubmatch(raw); found != nil {
		goals, err := strconv.Atoi(found[1])
		if err != nil {
			return nil, nil, false, err
		}
		pens, err := strconv.Atoi(found[2])
		if err != nil {
			return nil, nil, false, err
		}
		return &goals, &pens, true, nil
	}
	if found := plainRE.FindStringSubmatch(raw); found != nil {
		goals, err := strconv.Atoi(found[1])
		if err != nil {
			return nil, nil, false, err
		}
		return &goals, nil, true, nil
	}
	return nil, nil, false, fmt.Errorf("score %q", raw)
}

func parseTable(table *html.Node, group string, names map[string]string, orders map[string]int, next *int, clubs map[string]club) ([]domain.StandingInput, error) {
	var rows []domain.StandingInput
	var rowErr error
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if rowErr != nil || n == nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "tr" && findClass(n, "coluna-time") != nil {
			row, skip, err := parseStandingRow(n, group, names, orders, next, clubs, len(rows)+1)
			if err != nil {
				rowErr = err
				return
			}
			if !skip {
				rows = append(rows, row)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(table)
	if rowErr != nil {
		return nil, rowErr
	}
	if len(rows) == 0 {
		return nil, nil
	}
	if strings.TrimSpace(group) == "" {
		return nil, fmt.Errorf("standings table has no group title")
	}
	return rows, nil
}

func parseStandingRow(tr *html.Node, group string, names map[string]string, orders map[string]int, next *int, clubs map[string]club, fallbackPos int) (domain.StandingInput, bool, error) {
	cell := findClass(tr, "coluna-time")
	posText := strings.TrimSpace(textOf(findTag(cell, "b")))
	posRaw := digitsOnly(posText)
	pos := fallbackPos
	if posRaw != "" {
		got, err := strconv.Atoi(posRaw)
		if err != nil || got <= 0 {
			return domain.StandingInput{}, false, fmt.Errorf("standing position %q", posText)
		}
		pos = got
	}
	name := cleanClubName(textOf(findClass(cell, "time")))
	if name == "" {
		return domain.StandingInput{}, false, fmt.Errorf("standing has no club")
	}
	img := imageKey(attr(findTag(cell, "img"), "src"))
	if img != "" {
		if _, ok := names[img]; !ok {
			names[img] = name
		}
	}
	var stats []string
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "td" && hasClass(c, "coluna-dados") {
			stats = append(stats, strings.TrimSpace(textOf(c)))
		}
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
		got, err := strconv.Atoi(strings.TrimPrefix(strings.ReplaceAll(raw, " ", ""), "+"))
		if err != nil {
			return domain.StandingInput{}, false, fmt.Errorf("standing %s %q: %w", name, raw, err)
		}
		values[i] = got
	}
	if filled == 0 {
		return domain.StandingInput{}, true, nil
	}
	id := clubID(name)
	if id == "" {
		return domain.StandingInput{}, false, fmt.Errorf("standing %s has no id", name)
	}
	if _, ok := orders[group]; !ok {
		orders[group] = *next
		*next++
	}
	remember(clubs, id, name)
	return domain.StandingInput{
		ExternalTeamID: id,
		GroupName:      group,
		GroupOrder:     orders[group],
		Position:       pos,
		Points:         values[0],
		Played:         values[1],
		Wins:           values[2],
		Draws:          values[3],
		Losses:         values[4],
		GoalsFor:       values[5],
		GoalsAgainst:   values[6],
		GoalDifference: values[7],
	}, false, nil
}

func addMatch(out *page, match domain.MatchInput) error {
	for _, prev := range out.matches {
		if prev.ExternalID != match.ExternalID {
			continue
		}
		if prev.Status == match.Status && sameScore(prev.HomeScore, match.HomeScore) && sameScore(prev.AwayScore, match.AwayScore) && sameScore(prev.HomePenalties, match.HomePenalties) && sameScore(prev.AwayPenalties, match.AwayPenalties) && sameTime(prev.Kickoff, match.Kickoff) {
			return nil
		}
		return fmt.Errorf("match %s is published twice", match.ExternalID)
	}
	out.matches = append(out.matches, match)
	return nil
}

func kickoff(raw string, year int, loc *time.Location) (*time.Time, string, error) {
	found := dateRE.FindStringSubmatch(raw)
	if found == nil {
		return nil, "", nil
	}
	day, _ := strconv.Atoi(found[1])
	month, _ := strconv.Atoi(found[2])
	gotYear, _ := strconv.Atoi(found[3])
	if gotYear != year {
		return nil, "", errSkipMatch
	}
	stamp := fmt.Sprintf("%02d%02d%04d", day, month, gotYear)
	if found[4] == "" {
		return nil, stamp, nil
	}
	hour, _ := strconv.Atoi(found[4])
	minute, _ := strconv.Atoi(found[5])
	if hour > 23 || minute > 59 || day < 1 || day > 31 || month < 1 || month > 12 {
		return nil, "", fmt.Errorf("kickoff %q", raw)
	}
	when := time.Date(year, time.Month(month), day, hour, minute, 0, 0, loc)
	if when.Day() != day || int(when.Month()) != month || when.Year() != year {
		return nil, "", fmt.Errorf("kickoff %q", raw)
	}
	utc := when.UTC()
	return &utc, stamp, nil
}

func venueFrom(raw string) string {
	parts := strings.Split(raw, " - ")
	if len(parts) < 2 {
		return ""
	}
	return repairText(strings.TrimSpace(parts[len(parts)-1]))
}

// repairText undoes a club or place name whose UTF-8 bytes were stored as
// Latin-1 characters. A name that is already Unicode is left as published.
func repairText(raw string) string {
	if raw == "" || !strings.ContainsRune(raw, 'Ã') {
		return raw
	}
	buf := make([]byte, 0, len(raw))
	for _, r := range raw {
		if r > 0xFF {
			return raw
		}
		buf = append(buf, byte(r))
	}
	if !utf8.Valid(buf) {
		return raw
	}
	repaired := string(buf)
	if repaired == raw || strings.ContainsRune(repaired, 'Ã') || strings.ContainsRune(repaired, '\uFFFD') {
		return raw
	}
	return repaired
}

func groupTitle(section, title string) string {
	if section == "" || fold(section) == fold(title) {
		return title
	}
	return section + " - " + title
}

func cleanClubName(name string) string {
	name = repairText(name)
	name = strings.ReplaceAll(name, "*", "")
	name = strings.Join(strings.Fields(name), " ")
	for {
		next := strings.TrimSpace(championRE.ReplaceAllString(name, ""))
		next = strings.TrimSpace(roleRE.ReplaceAllString(next, ""))
		next = strings.Join(strings.Fields(next), " ")
		if next == name {
			return name
		}
		name = next
	}
}

func aliasClubIDs(out *page, names map[string]string) {
	ids := make([]string, 0, len(out.clubs))
	for id := range out.clubs {
		ids = append(ids, id)
	}
	canon := map[string]string{}
	for _, id := range ids {
		best := id
		for _, other := range ids {
			if id != other && len(id) >= 4 && len(other) > len(best) && sameClubSuffix(id, other) {
				best = other
			}
		}
		canon[id] = best
	}
	for i, table := range out.tables {
		for j, row := range table {
			best := canon[row.ExternalTeamID]
			if best == "" || best == row.ExternalTeamID {
				continue
			}
			row.ExternalTeamID = best
			out.tables[i][j] = row
		}
	}
	for id, best := range canon {
		if id != best {
			delete(out.clubs, id)
		}
	}
	for img, name := range names {
		id := clubID(name)
		best := canon[id]
		if best == "" || best == id {
			continue
		}
		if item, ok := out.clubs[best]; ok && item.Name != "" {
			names[img] = item.Name
		}
	}
}

func sameClubSuffix(shorter, longer string) bool {
	if len(shorter) >= len(longer) || !strings.HasPrefix(longer, shorter) {
		return false
	}
	switch longer[len(shorter):] {
	case "ac", "clube", "ec", "fc", "futebol", "futebolclube", "saf":
		return true
	default:
		return false
	}
}

func clubFromImage(src string, names map[string]string, clubs map[string]club) (string, bool) {
	key := imageKey(src)
	if name, ok := names[key]; ok && name != "" {
		return name, true
	}
	stem := imageStem(key)
	if len(stem) < 4 {
		return "", false
	}
	exact := ""
	hit := ""
	count := 0
	for id, item := range clubs {
		if item.Name == "" {
			continue
		}
		if id == stem {
			exact = item.Name
		}
		if strings.Contains(id, stem) || strings.Contains(stem, id) {
			hit = item.Name
			count++
		}
	}
	if exact != "" {
		return exact, true
	}
	if count == 1 {
		return hit, true
	}
	return "", false
}

func imageStem(key string) string {
	base := strings.TrimSuffix(key, path.Ext(key))
	base = leadingDigits.ReplaceAllString(base, "")
	var b strings.Builder
	for _, r := range fold(base) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

func imageKey(src string) string {
	parsed, err := url.Parse(strings.TrimSpace(src))
	if err != nil {
		return ""
	}
	base := strings.ToLower(path.Base(parsed.Path))
	if base == "" || base == "." || base == "/" {
		return ""
	}
	return base
}

func clubID(name string) string {
	name = cleanClubName(name)
	var b strings.Builder
	for _, r := range fold(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

func skipPhase(name string) bool {
	folded := fold(name)
	for _, skip := range []string{"FEMININ", "SUB", "COPA", "BASE", "AMADOR", "MASTER", "INFANTIL"} {
		if strings.Contains(folded, skip) {
			return true
		}
	}
	return false
}

func remember(clubs map[string]club, id, name string) {
	if id == "" || name == "" {
		return
	}
	if _, ok := clubs[id]; ok {
		return
	}
	clubs[id] = club{Name: name, Short: name}
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

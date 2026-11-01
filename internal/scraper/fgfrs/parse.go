package fgfrs

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
	"golang.org/x/net/html"
)

// PhaseRef is one phase link on a year page.
type PhaseRef struct {
	ID   string
	Name string
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
	rodadaRE   = regexp.MustCompile(`(?i)RODADA\s+(\d{1,2})`)
	scoreRE    = regexp.MustCompile(`(?i)(\d+)\s*x\s*(\d+)`)
	clockRE    = regexp.MustCompile(`(\d{2})/(\d{2})\s+(\d{2}):(\d{2})`)
	ordinalRE  = regexp.MustCompile(`^\d+[º°o]?$`)
	digitsRE   = regexp.MustCompile(`\d+`)
	alteraRE   = regexp.MustCompile(`(?i)\s*\d*\s*alter`)
	slugYearRE = regexp.MustCompile(`(?:^|-)(\d{2})-(\d{2})-(\d{4})$`)
)

// errSkipMatch is a slug whose date belongs to another catalog year.
// A missing date and the placeholder year 1900 stay in the requested season.
var errSkipMatch = errors.New("match slug belongs to another year")

// ParseYears reads the year select for one division. Option URLs are not
// requested. Only the year number is kept, and only for this division id.
func ParseYears(body, divisionID string) ([]int, error) {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse years: %w", err)
	}
	var years []int
	seen := map[int]struct{}{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if err != nil || n == nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "option" {
			year, ok := yearOption(attr(n, "value"), divisionID)
			if ok {
				if _, dup := seen[year]; !dup {
					seen[year] = struct{}{}
					years = append(years, year)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return years, err
}

func yearOption(raw, divisionID string) (int, bool) {
	parts := pathParts(raw)
	if len(parts) != 4 || parts[0] != "competicoes" || parts[1] != "profissional" || parts[2] != divisionID || !fourDigit(parts[3]) {
		return 0, false
	}
	year, err := strconv.Atoi(parts[3])
	if err != nil || year < 1990 || year > 2100 {
		return 0, false
	}
	return year, true
}

// ParsePhases reads the phase buttons for one division and year.
// Copa, Recopa, women's, and youth labels are skipped. The href is not followed.
func ParsePhases(body, divisionID, year string) ([]PhaseRef, error) {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse phases: %w", err)
	}
	var refs []PhaseRef
	seen := map[string]struct{}{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if err != nil || n == nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "menu_comp") {
			ref, ok := phaseLink(n, divisionID, year)
			if ok {
				if _, dup := seen[ref.ID]; dup {
					err = fmt.Errorf("phase %s is listed twice", ref.ID)
					return
				}
				seen[ref.ID] = struct{}{}
				refs = append(refs, ref)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return refs, err
}

// foreignYearOnly reports that this division's phase buttons all point at
// another catalog year. The year URL is then another season, not an empty shell.
func foreignYearOnly(body, divisionID, year string) (bool, error) {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("parse phases: %w", err)
	}
	same, other := 0, 0
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "menu_comp") {
			switch phaseLinkYear(n, divisionID) {
			case year:
				same++
			case "":
			default:
				other++
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return same == 0 && other > 0, nil
}

func phaseLinkYear(n *html.Node, divisionID string) string {
	parts := pathParts(attr(n, "href"))
	if len(parts) != 5 || parts[0] != "competicoes" || parts[1] != "profissional" || parts[2] != divisionID || !fourDigit(parts[3]) || !digits.MatchString(parts[4]) {
		return ""
	}
	return parts[3]
}

func phaseLink(n *html.Node, divisionID, year string) (PhaseRef, bool) {
	parts := pathParts(attr(n, "href"))
	name := strings.TrimSpace(textOf(n))
	if len(parts) != 5 || parts[0] != "competicoes" || parts[1] != "profissional" || parts[2] != divisionID || parts[3] != year || !digits.MatchString(parts[4]) {
		return PhaseRef{}, false
	}
	if name == "" || skipPhase(name) {
		return PhaseRef{}, false
	}
	return PhaseRef{ID: parts[4], Name: name}, true
}

func skipPhase(name string) bool {
	folded := fold(name)
	for _, skip := range []string{"COPA", "RECOPA", "FEMININ", "SUB", "BASE", "AMADOR", "MASTER", "INFANTIL"} {
		if strings.Contains(folded, skip) {
			return true
		}
	}
	return false
}

// ParsePhase reads one phase page. The phase name comes from the year page,
// because a knockout page does not repeat it on each match. Cards and the
// win percentage are stored when the table publishes them.
func ParsePhase(body string, divisionID string, year int, phase string, loc *time.Location) (page, error) {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return page{}, fmt.Errorf("parse phase: %w", err)
	}
	out := page{clubs: map[string]club{}}
	orders := map[string]int{}
	nextOrder := 0
	group := ""
	var walkTables func(*html.Node)
	walkTables = func(n *html.Node) {
		if n == nil || err != nil {
			return
		}
		if n.Type != html.ElementNode {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walkTables(c)
			}
			return
		}
		if hasClass(n, "titulo-grupo") {
			if title := strings.TrimSpace(textOf(n)); title != "" {
				group = title
			}
			return
		}
		if hasClass(n, "tabela") {
			rows, rowErr := parseTable(n, group, orders, &nextOrder, out.clubs)
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
	round := 1
	var walkMatches func(*html.Node)
	walkMatches = func(n *html.Node) {
		if n == nil || err != nil {
			return
		}
		if n.Type != html.ElementNode {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walkMatches(c)
			}
			return
		}
		if n.Data == "h4" {
			if found := rodadaRE.FindStringSubmatch(textOf(n)); found != nil {
				got, convErr := strconv.Atoi(found[1])
				if convErr != nil || got < 1 || got > 40 {
					err = fmt.Errorf("round %q is out of range", found[1])
					return
				}
				round = got
			}
		}
		if hasClass(n, "carousel-conteudo") || hasClass(n, "bordas2") {
			match, rowErr := parseMatch(n, divisionID, year, phase, round, loc, out.clubs)
			if errors.Is(rowErr, errSkipMatch) {
				return
			}
			if rowErr != nil {
				err = rowErr
				return
			}
			out.matches = append(out.matches, match)
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

func parseTable(n *html.Node, group string, orders map[string]int, next *int, clubs map[string]club) ([]domain.StandingInput, error) {
	var rows []domain.StandingInput
	var rowErr error
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if rowErr != nil || node == nil {
			return
		}
		if node.Type == html.ElementNode && node.Data == "tr" && findClass(node, "posicao-time") != nil {
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
		return nil, fmt.Errorf("standings table has no group title")
	}
	return rows, nil
}

func parseStandingRow(tr *html.Node, group string, orders map[string]int, next *int, clubs map[string]club, fallbackPos int) (domain.StandingInput, bool, error) {
	full := findClass(tr, "posicao-time")
	if full == nil {
		return domain.StandingInput{}, false, fmt.Errorf("standing has no club")
	}
	posText := textOf(findTag(full, "b"))
	posRaw := digitsRE.FindString(posText)
	var pos int
	var err error
	if posRaw == "" {
		pos = fallbackPos
	} else {
		pos, err = strconv.Atoi(posRaw)
	}
	if err != nil || pos <= 0 {
		return domain.StandingInput{}, false, fmt.Errorf("standing position %q", posText)
	}
	name := strings.TrimSpace(attr(findTag(full, "img"), "title"))
	if name == "" {
		return domain.StandingInput{}, false, fmt.Errorf("standing has no club name")
	}
	id := clubID(attr(findTag(full, "img"), "src"))
	if id == "" {
		return domain.StandingInput{}, false, fmt.Errorf("standing %s has no badge", name)
	}
	short := name
	if small := findClass(tr, "posicao-time2"); small != nil {
		if got := cleanLabel(textOf(small)); got != "" {
			short = got
		}
	}
	row := domain.StandingInput{
		ExternalTeamID: id,
		GroupName:      group,
		Position:       pos,
	}
	filled := 0
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || c.Data != "td" {
			continue
		}
		key := statKey(attr(c, "title"))
		if key == "" {
			continue
		}
		raw := strings.TrimSpace(textOf(c))
		if raw == "" {
			continue
		}
		filled++
		var value int
		if key == "pct" && (strings.Contains(raw, ".") || strings.Contains(raw, ",")) {
			value, err = publishedPercent(raw)
		} else {
			value, err = atoiCell(strings.TrimSuffix(strings.TrimSpace(raw), "%"))
		}
		if err != nil {
			return domain.StandingInput{}, false, fmt.Errorf("standing %s %q: %w", key, raw, err)
		}
		switch key {
		case "pts":
			row.Points = value
		case "j":
			row.Played = value
		case "v":
			row.Wins = value
		case "e":
			row.Draws = value
		case "d":
			row.Losses = value
		case "gp":
			row.GoalsFor = value
		case "gc":
			row.GoalsAgainst = value
		case "sg":
			row.GoalDifference = value
		case "yellow":
			row.YellowCards = value
		case "red":
			row.RedCards = value
		case "pct":
			row.WinPercentage = value
		}
	}
	if filled == 0 {
		return domain.StandingInput{}, true, nil
	}
	if _, ok := orders[group]; !ok {
		orders[group] = *next
		*next++
	}
	row.GroupOrder = orders[group]
	remember(clubs, id, name, short)
	return row, false, nil
}

func statKey(title string) string {
	folded := fold(title)
	switch {
	case strings.Contains(folded, "PONTO"):
		return "pts"
	case strings.Contains(folded, "JOGO"):
		return "j"
	case strings.Contains(folded, "VITOR"):
		return "v"
	case strings.Contains(folded, "EMPAT"):
		return "e"
	case strings.Contains(folded, "DERROT"):
		return "d"
	case strings.Contains(folded, "APROVEIT") || strings.Contains(title, "%"):
		return "pct"
	case strings.Contains(folded, "PRO"):
		return "gp"
	case strings.Contains(folded, "CONTRA"):
		return "gc"
	case strings.Contains(folded, "SALDO"):
		return "sg"
	case strings.Contains(folded, "AMAREL"):
		return "yellow"
	case strings.Contains(folded, "VERMELH"):
		return "red"
	default:
		return ""
	}
}

func parseMatch(card *html.Node, divisionID string, year int, phase string, round int, loc *time.Location, clubs map[string]club) (domain.MatchInput, error) {
	homeImg := findTag(findClass(card, "mandante"), "img")
	awayImg := findTag(findClass(card, "visitante"), "img")
	homeName := strings.TrimSpace(attr(homeImg, "title"))
	awayName := strings.TrimSpace(attr(awayImg, "title"))
	homeID := clubID(attr(homeImg, "src"))
	awayID := clubID(attr(awayImg, "src"))
	if homeName == "" || awayName == "" || homeID == "" || awayID == "" {
		return domain.MatchInput{}, fmt.Errorf("match is missing a club")
	}
	if homeID == awayID {
		return domain.MatchInput{}, fmt.Errorf("match lists %s twice", homeName)
	}
	slug := matchSlug(card)
	if slug == "" {
		return domain.MatchInput{}, fmt.Errorf("match %s x %s has no id", homeName, awayName)
	}
	if slugYear, ok := slugDateYear(slug); ok && slugYear != 1900 && slugYear != year {
		return domain.MatchInput{}, errSkipMatch
	}
	remember(clubs, homeID, homeName, homeName)
	remember(clubs, awayID, awayName, awayName)
	whenText := textOf(findClass(card, "conteudo-descricao"))
	if whenText == "" {
		whenText = textOf(findClass(card, "conteudo-descricao2"))
	}
	when, err := kickoff(whenText, year, loc)
	if err != nil {
		return domain.MatchInput{}, err
	}
	scoreNode := findClass(card, "contra")
	if scoreNode == nil {
		scoreNode = findClass(card, "contra2")
	}
	scoreText := textWithoutLinks(scoreNode)
	home, away, scored, err := scores(scoreText)
	if err != nil {
		return domain.MatchInput{}, fmt.Errorf("match %s: %w", slug, err)
	}
	match := domain.MatchInput{
		ExternalID:     fmt.Sprintf("%s-%d-%s", divisionID, year, slug),
		RoundNumber:    round,
		Phase:          phase,
		HomeExternalID: homeID,
		AwayExternalID: awayID,
		Kickoff:        when,
		Venue:          venueFrom(whenText),
		State:          "RS",
	}
	if scored {
		match.Status = domain.StatusFinished
		match.HomeScore = home
		match.AwayScore = away
	} else {
		match.Status = domain.StatusScheduled
	}
	return match, nil
}

func slugDateYear(slug string) (int, bool) {
	found := slugYearRE.FindStringSubmatch(slug)
	if found == nil {
		return 0, false
	}
	year, err := strconv.Atoi(found[3])
	if err != nil || year < 1900 || year > 2100 {
		return 0, false
	}
	return year, true
}

func matchSlug(card *html.Node) string {
	var slug string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if slug != "" || n == nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" {
			parts := pathParts(attr(n, "href"))
			if len(parts) >= 2 && parts[len(parts)-2] == "jogo" {
				slug = parts[len(parts)-1]
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(card)
	return slug
}

func scores(text string) (*int, *int, bool, error) {
	text = strings.TrimSpace(text)
	if text == "" || strings.EqualFold(text, "x") || text == "-" {
		return nil, nil, false, nil
	}
	m := scoreRE.FindStringSubmatch(text)
	if m == nil {
		return nil, nil, false, fmt.Errorf("score %q", text)
	}
	home, err := strconv.Atoi(m[1])
	if err != nil {
		return nil, nil, false, err
	}
	away, err := strconv.Atoi(m[2])
	if err != nil {
		return nil, nil, false, err
	}
	return &home, &away, true, nil
}

func kickoff(raw string, year int, loc *time.Location) (*time.Time, error) {
	m := clockRE.FindStringSubmatch(raw)
	if m == nil {
		return nil, nil
	}
	day, _ := strconv.Atoi(m[1])
	month, _ := strconv.Atoi(m[2])
	hour, _ := strconv.Atoi(m[3])
	minute, _ := strconv.Atoi(m[4])
	local := time.Date(year, time.Month(month), day, hour, minute, 0, 0, loc)
	if local.Year() != year || local.Month() != time.Month(month) || local.Day() != day {
		return nil, fmt.Errorf("kickoff %s is not in %d", raw, year)
	}
	when := local.UTC()
	return &when, nil
}

func venueFrom(raw string) string {
	i := strings.Index(raw, " - ")
	if i < 0 {
		return ""
	}
	rest := strings.TrimSpace(raw[i+3:])
	if parts := alteraRE.Split(rest, 2); len(parts) == 2 {
		rest = parts[0]
	}
	return strings.Trim(strings.TrimSpace(rest), " -")
}

func clubID(raw string) string {
	parts := pathParts(raw)
	if len(parts) == 0 {
		return ""
	}
	base := parts[len(parts)-1]
	ext := path.Ext(base)
	base = strings.TrimSuffix(base, ext)
	return strings.ToLower(base)
}

func cleanLabel(raw string) string {
	var kept []string
	for _, field := range strings.Fields(raw) {
		if ordinalRE.MatchString(field) {
			continue
		}
		kept = append(kept, field)
	}
	return strings.Join(kept, " ")
}

func publishedPercent(text string) (int, error) {
	text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), "%"))
	text = strings.ReplaceAll(text, ",", ".")
	negative := strings.HasPrefix(text, "-")
	text = strings.TrimPrefix(text, "-")
	parts := strings.Split(text, ".")
	if len(parts) == 0 || len(parts) > 2 || parts[0] == "" {
		return 0, fmt.Errorf("percentage %q", text)
	}
	whole, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("percentage %q", text)
	}
	if len(parts) == 2 && parts[1] != "" && parts[1][0] >= '5' {
		whole++
	}
	if negative {
		whole = -whole
	}
	return whole, nil
}

func atoiCell(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "+")
	raw = strings.ReplaceAll(raw, "\u2212", "-")
	if raw == "" {
		return 0, fmt.Errorf("empty number")
	}
	return strconv.Atoi(raw)
}

func remember(clubs map[string]club, id, name, short string) {
	if id == "" || name == "" {
		return
	}
	if short == "" {
		short = name
	}
	if _, ok := clubs[id]; ok {
		return
	}
	clubs[id] = club{Name: name, Short: short}
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

func textWithoutLinks(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
			b.WriteByte(' ')
		}
		if node.Type == html.ElementNode && (node.Data == "script" || node.Data == "style" || node.Data == "a") {
			return
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
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
			b.WriteByte(' ')
		}
		if node.Type == html.ElementNode && (node.Data == "script" || node.Data == "style") {
			return
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
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

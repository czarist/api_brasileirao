package fpfpr

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
	"golang.org/x/net/html"
)

// SeasonRef is one adult division the catalog listed.
type SeasonRef struct {
	Code string
	Year int
	ID   string
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
	yearRE  = regexp.MustCompile(`\b((?:19|20)\d{2})\b`)
	jogoRE  = regexp.MustCompile(`/jogo/(\d+)/?$`)
	clockRE = regexp.MustCompile(`(\d{2})/(\d{2}).{0,80}?(\d{2}):(\d{2})`)
	scoreRE = regexp.MustCompile(`(?i)^\s*(\d+)\s*x\s*(\d+)\s*$`)
)

// ParseCatalog reads the professional select.
// Copa Paraná and any women's or youth option are ignored.
func ParseCatalog(body string) ([]SeasonRef, error) {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	var refs []SeasonRef
	seen := map[string]struct{}{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if err != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "option" {
			ref, ok, rowErr := seasonOption(n)
			if rowErr != nil {
				err = rowErr
				return
			}
			if ok {
				key := ref.Code + "/" + strconv.Itoa(ref.Year)
				if _, dup := seen[key]; dup {
					err = fmt.Errorf("catalog lists %s %d twice", ref.Code, ref.Year)
					return
				}
				seen[key] = struct{}{}
				refs = append(refs, ref)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	if err != nil {
		return nil, err
	}
	return refs, nil
}

func seasonOption(n *html.Node) (SeasonRef, bool, error) {
	id := strings.TrimSpace(attr(n, "value"))
	title := strings.TrimSpace(textOf(n))
	if id == "" || title == "" {
		return SeasonRef{}, false, nil
	}
	if !digits.MatchString(id) {
		return SeasonRef{}, false, fmt.Errorf("catalog id %q is not numeric", id)
	}
	code, ok := divisionCode(title)
	if !ok {
		return SeasonRef{}, false, nil
	}
	years := yearRE.FindAllStringSubmatch(title, -1)
	if len(years) == 0 {
		return SeasonRef{}, false, fmt.Errorf("option %q has no year", title)
	}
	year, err := strconv.Atoi(years[len(years)-1][1])
	if err != nil || year < 1990 || year > 2100 {
		return SeasonRef{}, false, fmt.Errorf("option %q has a year outside the range", title)
	}
	return SeasonRef{Code: code, Year: year, ID: id}, true, nil
}

func divisionCode(title string) (string, bool) {
	folded := fold(title)
	for _, skip := range []string{"COPA", "FEMININ", "SUB", "BASE", "AMADOR", "MASTER", "INFANTIL"} {
		if strings.Contains(folded, skip) {
			return "", false
		}
	}
	switch {
	case strings.Contains(folded, "TERCEIRONA"):
		return codeC, true
	case strings.Contains(folded, "SEGUNDONA"):
		return codeB, true
	case strings.Contains(folded, "PARANAENSE"):
		return codeA, true
	default:
		return "", false
	}
}

// ParsePage reads one season page. Cards and a win percentage that the table
// does not publish stay 0. A decimal percentage is rounded by the first decimal
// digit. The same club keeps the row with more games played.
func ParsePage(body string, year int, loc *time.Location) (page, error) {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return page{}, fmt.Errorf("parse season: %w", err)
	}
	out := page{clubs: map[string]club{}}
	phases := map[string]string{}
	orders := map[string]int{}
	nextOrder := 0
	var walk func(*html.Node, int, string, bool)
	walk = func(n *html.Node, round int, label string, inSlide bool) {
		if n == nil || err != nil {
			return
		}
		if n.Type != html.ElementNode {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c, round, label, inSlide)
			}
			return
		}
		if hasClass(n, "zabmcbf-fase-tab") {
			if key := strings.TrimSpace(attr(n, "data-fase-key")); key != "" {
				if name := strings.TrimSpace(textOf(n)); name != "" {
					phases[key] = name
				}
			}
		}
		if hasClass(n, "zabmcbf-carousel-slide") {
			inSlide = true
			label = strings.TrimSpace(attr(n, "data-rodada-label"))
			got, convErr := strconv.Atoi(strings.TrimSpace(attr(n, "data-rodada")))
			if convErr != nil || got < 1 || got > 40 {
				err = fmt.Errorf("round %q is out of range", attr(n, "data-rodada"))
				return
			}
			round = got
		}
		if hasClass(n, "zabmcbf-grupo-classificacao") {
			rows, rowErr := parseGroup(n, orders, &nextOrder, out.clubs)
			if rowErr != nil {
				err = rowErr
				return
			}
			if len(rows) > 0 {
				out.tables = append(out.tables, rows)
			}
			return
		}
		if inSlide && hasClass(n, "zabmcbf-carousel-jogo") {
			match, rowErr := parseMatch(n, phases, round, label, year, loc, out.clubs)
			if rowErr != nil {
				err = rowErr
				return
			}
			out.matches = append(out.matches, match)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, round, label, inSlide)
		}
	}
	walk(root, 0, "", false)
	if err != nil {
		return page{}, err
	}
	return out, nil
}

func parseGroup(n *html.Node, orders map[string]int, next *int, clubs map[string]club) ([]domain.StandingInput, error) {
	title := strings.TrimSpace(textOf(findClass(n, "zabmcbf-grupo-titulo")))
	if title == "" {
		return nil, fmt.Errorf("standings group has no title")
	}
	if _, ok := orders[title]; !ok {
		orders[title] = *next
		*next++
	}
	table := findClass(n, "zabmcbf-tabela")
	if table == nil {
		return nil, fmt.Errorf("group %s has no table", title)
	}
	var header []string
	var rows []domain.StandingInput
	var rowErr error
	var walk func(*html.Node, bool)
	walk = func(node *html.Node, head bool) {
		if rowErr != nil || node.Type != html.ElementNode {
			return
		}
		switch node.Data {
		case "thead":
			head = true
		case "tbody":
			head = false
		case "tr":
			if head {
				header = headerLabels(node)
			} else if len(header) > 0 {
				row, ok, err := parseStandingRow(node, header, title, orders[title], clubs)
				if err != nil {
					rowErr = err
					return
				}
				if ok {
					rows = append(rows, row)
				}
			}
			return
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c, head)
		}
	}
	walk(table, false)
	if rowErr != nil {
		return nil, rowErr
	}
	return rows, nil
}

func headerLabels(tr *html.Node) []string {
	var labels []string
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && (c.Data == "th" || c.Data == "td") {
			labels = append(labels, fold(textOf(c)))
		}
	}
	return labels
}

func parseStandingRow(tr *html.Node, header []string, group string, order int, clubs map[string]club) (domain.StandingInput, bool, error) {
	var pos int
	var name string
	var nums []string
	seenClub := false
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || (c.Data != "td" && c.Data != "th") {
			continue
		}
		if !seenClub {
			got, clubName, err := clubCell(c)
			if err != nil {
				return domain.StandingInput{}, false, err
			}
			pos = got
			name = clubName
			seenClub = true
			continue
		}
		nums = append(nums, strings.TrimSpace(textOf(c)))
	}
	if name == "" {
		return domain.StandingInput{}, false, nil
	}
	id := slug(name)
	remember(clubs, id, name)
	row := domain.StandingInput{
		ExternalTeamID: id,
		GroupName:      group,
		GroupOrder:     order,
		Position:       pos,
	}
	if row.Position <= 0 {
		return domain.StandingInput{}, false, fmt.Errorf("standing for %s has no position", name)
	}
	stats := map[string]int{}
	// The first header cell is the club. The numbers follow it.
	for i, label := range header {
		if i == 0 {
			continue
		}
		idx := i - 1
		if idx >= len(nums) {
			break
		}
		key := statKey(label)
		if key == "" {
			continue
		}
		var value int
		var err error
		if key == "pct" {
			value, err = publishedPercent(nums[idx])
		} else {
			value, err = atoiCell(nums[idx])
		}
		if err != nil {
			return domain.StandingInput{}, false, fmt.Errorf("standing %s %q: %w", key, nums[idx], err)
		}
		stats[key] = value
	}
	row.Points = stats["pts"]
	row.Played = stats["j"]
	row.Wins = stats["v"]
	row.Draws = stats["e"]
	row.Losses = stats["d"]
	row.GoalsFor = stats["gp"]
	row.GoalsAgainst = stats["gc"]
	row.GoalDifference = stats["sg"]
	row.WinPercentage = stats["pct"]
	return row, true, nil
}

func statKey(label string) string {
	switch {
	case strings.Contains(label, "%") || strings.Contains(label, "APROVEIT"):
		return "pct"
	case label == "GP" || strings.Contains(label, "PRO") || strings.Contains(label, "MARC"):
		return "gp"
	case label == "GC" || strings.Contains(label, "CONTRA"):
		return "gc"
	case label == "SG" || strings.Contains(label, "SALDO"):
		return "sg"
	case label == "J" || strings.Contains(label, "JOGO"):
		return "j"
	case label == "V" || strings.Contains(label, "VITOR"):
		return "v"
	case label == "E" || strings.Contains(label, "EMPAT"):
		return "e"
	case label == "D" || strings.Contains(label, "DERROT"):
		return "d"
	case label == "P" || strings.Contains(label, "PONTO"):
		return "pts"
	default:
		return ""
	}
}

func clubCell(td *html.Node) (int, string, error) {
	posNode := findClass(td, "posicaotabela")
	if posNode == nil {
		return 0, "", fmt.Errorf("standing has no position")
	}
	pos, err := strconv.Atoi(strings.TrimSpace(textOf(posNode)))
	if err != nil || pos <= 0 {
		return 0, "", fmt.Errorf("standing position %q", textOf(posNode))
	}
	name := strings.TrimSpace(textExcept(td, posNode))
	if name == "" {
		return 0, "", fmt.Errorf("standing has no club")
	}
	return pos, name, nil
}

func parseMatch(card *html.Node, phases map[string]string, round int, label string, year int, loc *time.Location, clubs map[string]club) (domain.MatchInput, error) {
	link := findTag(card, "a")
	if link == nil {
		return domain.MatchInput{}, fmt.Errorf("match has no link")
	}
	path := pathOf(attr(link, "href"))
	m := jogoRE.FindStringSubmatch(path)
	if m == nil {
		return domain.MatchInput{}, fmt.Errorf("match link %q has no id", attr(link, "href"))
	}
	key := strings.TrimSpace(attr(card, "data-fase-key"))
	phase := phases[key]
	if phase == "" {
		phase = strings.TrimSpace(label)
	}
	if phase == "" {
		phase = key
	}
	if phase == "" {
		return domain.MatchInput{}, fmt.Errorf("match %s has no phase", m[1])
	}
	homeName := strings.TrimSpace(textOf(findClass(card, "zabmcbf-carousel-jogo-mandante")))
	awayName := strings.TrimSpace(textOf(findClass(card, "zabmcbf-carousel-jogo-visitante")))
	// The side node also contains the badge alt, which repeats the name.
	homeName = firstLine(homeName)
	awayName = firstLine(awayName)
	if homeName == "" || awayName == "" {
		return domain.MatchInput{}, fmt.Errorf("match %s is missing a club", m[1])
	}
	homeID := slug(homeName)
	awayID := slug(awayName)
	if homeID == "" || awayID == "" || homeID == awayID {
		return domain.MatchInput{}, fmt.Errorf("match %s lists the same club twice", m[1])
	}
	remember(clubs, homeID, homeName)
	remember(clubs, awayID, awayName)
	match := domain.MatchInput{
		ExternalID:     strconv.Itoa(year) + "-" + m[1],
		MatchNumber:    atoiOrZero(m[1]),
		RoundNumber:    round,
		Phase:          phase,
		HomeExternalID: homeID,
		AwayExternalID: awayID,
		State:          "PR",
	}
	when, err := kickoff(textOf(findClass(card, "zabmcbf-carousel-jogo-data")), year, loc)
	if err != nil {
		return domain.MatchInput{}, err
	}
	match.Kickoff = when
	match.Venue = strings.TrimSpace(textOf(findClass(card, "zabmcbf-carousel-jogo-estadio")))
	home, away, scored, err := scores(textOf(findClass(card, "zabmcbf-carousel-jogo-placar-numero")))
	if err != nil {
		return domain.MatchInput{}, fmt.Errorf("match %s: %w", m[1], err)
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

func scores(text string) (*int, *int, bool, error) {
	text = strings.TrimSpace(text)
	if text == "" || text == "x" || text == "X" || text == "-" {
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
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	m := clockRE.FindStringSubmatch(raw)
	if m == nil {
		return nil, nil
	}
	day, _ := strconv.Atoi(m[1])
	month, _ := strconv.Atoi(m[2])
	hour, _ := strconv.Atoi(m[3])
	minute, _ := strconv.Atoi(m[4])
	when := time.Date(year, time.Month(month), day, hour, minute, 0, 0, loc).UTC()
	return &when, nil
}

func firstLine(raw string) string {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return ""
	}
	// The visible name is the text before a repeated badge alt. Keep every
	// word; the alt is the same phrase, so a doubled name is cut in half.
	if len(fields)%2 == 0 {
		half := strings.Join(fields[:len(fields)/2], " ")
		if half == strings.Join(fields[len(fields)/2:], " ") {
			return half
		}
	}
	return strings.Join(fields, " ")
}

// publishedPercent rounds the first decimal digit. 61.1 becomes 61 and
// 56.7 becomes 57. The source prints a percent sign.
func publishedPercent(text string) (int, error) {
	text = strings.TrimSpace(text)
	text = strings.TrimSuffix(text, "%")
	text = strings.TrimSpace(text)
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
	if len(parts) == 2 {
		frac := parts[1]
		if frac == "" || !digits.MatchString(frac) {
			return 0, fmt.Errorf("percentage %q", text)
		}
		if frac[0] >= '5' {
			whole++
		}
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

func atoiOrZero(raw string) int {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return n
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

func pathOf(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Path
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

func slug(raw string) string {
	folded := fold(raw)
	var b strings.Builder
	dash := false
	for _, r := range folded {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteByte(byte(r - 'A' + 'a'))
			dash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
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

func textExcept(n, skip *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == skip {
			return
		}
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
			b.WriteByte(' ')
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

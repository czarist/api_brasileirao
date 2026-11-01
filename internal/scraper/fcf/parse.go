package fcf

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

// SeasonRef is one adult division the catalog listed for a year.
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
	yearPath = regexp.MustCompile(`^/competicoes/profissionais/(\d{4})/?$`)
	seasonID = regexp.MustCompile(`^/competicoes/profissionais/(\d{4})/(\d+)/?$`)
	clockRE  = regexp.MustCompile(`(\d{2})/(\d{2})/(\d{4})(?:\s*[\x{2013}\x{2014}-]\s*(\d{2}):(\d{2}))?`)
	roundRE  = regexp.MustCompile(`(\d{1,2})`)
	jogoRE   = regexp.MustCompile(`(?i)jogo\s*(\d+)`)
)

// ParseYears reads the professional index and returns the years it links.
func ParseYears(body string) ([]int, error) {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	seen := map[int]struct{}{}
	var years []int
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			if year, ok := yearFromHref(attr(n, "href")); ok {
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
	if len(years) == 0 {
		return nil, fmt.Errorf("catalog lists no professional years")
	}
	return years, nil
}

func yearFromHref(raw string) (int, bool) {
	path := pathOf(raw)
	m := yearPath.FindStringSubmatch(path)
	if m == nil {
		return 0, false
	}
	year, err := strconv.Atoi(m[1])
	if err != nil || year < 1990 || year > 2100 {
		return 0, false
	}
	return year, true
}

// ParseSeasons reads the division select on one year page.
// Copa, Recopa, women's, and youth options are ignored.
// Two options for the same division are an error.
func ParseSeasons(body string, year int) ([]SeasonRef, error) {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse year page: %w", err)
	}
	var refs []SeasonRef
	seen := map[string]struct{}{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "option" {
			code, ok := divisionCode(textOf(n))
			path := pathOf(attr(n, "value"))
			m := seasonID.FindStringSubmatch(path)
			if ok && m != nil {
				got, convErr := strconv.Atoi(m[1])
				if convErr == nil && got == year {
					if _, dup := seen[code]; dup {
						err = fmt.Errorf("year %d lists %s twice", year, code)
						return
					}
					seen[code] = struct{}{}
					refs = append(refs, SeasonRef{Code: code, Year: year, ID: m[2]})
				}
			}
		}
		if err != nil {
			return
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

func divisionCode(title string) (string, bool) {
	folded := fold(title)
	for _, skip := range []string{"COPA", "RECOPA", "FEMININ", "SUB", "BASE", "AMADOR", "MASTER", "INFANTIL"} {
		if strings.Contains(folded, skip) {
			return "", false
		}
	}
	switch {
	case series(folded, "C"):
		return codeC, true
	case series(folded, "B"):
		return codeB, true
	case series(folded, "A"):
		return codeA, true
	default:
		return "", false
	}
}

func series(folded, letter string) bool {
	token := "SERIE " + letter
	i := strings.Index(folded, token)
	if i < 0 {
		return false
	}
	end := i + len(token)
	if end < len(folded) && folded[end] >= '0' && folded[end] <= '9' {
		return false
	}
	return true
}

// ParsePage reads every phase on a season page. The table does not publish
// cards or a win percentage, so those figures are 0. A later row for a club
// replaces an earlier one only when it has more games played.
func ParsePage(body string, year int, loc *time.Location) (page, error) {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return page{}, fmt.Errorf("parse season: %w", err)
	}
	out := page{clubs: map[string]club{}}
	var group string
	orders := map[string]int{}
	nextOrder := 0
	var walk func(*html.Node, string, string)
	walk = func(n *html.Node, phase, round string) {
		if err != nil {
			return
		}
		// html.Parse returns a document node. Only elements carry classes.
		if n.Type != html.ElementNode {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c, phase, round)
			}
			return
		}
		if hasClass(n, "fcf-fase-panel") {
			if name := strings.TrimSpace(attr(n, "data-fase-nome")); name != "" {
				phase = name
			}
			group = ""
		}
		if name := strings.TrimSpace(attr(n, "data-rodada-nome")); name != "" {
			round = name
		}
		if hasClass(n, "fcf-grupo-nome") {
			group = strings.TrimSpace(textOf(n))
			if group != "" {
				if _, ok := orders[group]; !ok {
					orders[group] = nextOrder
					nextOrder++
				}
			}
		}
		if hasClass(n, "fcf-tabela-classificacao") {
			rows, rowErr := parseTable(n, group, orders[group], out.clubs)
			if rowErr != nil {
				err = rowErr
				return
			}
			if len(rows) > 0 {
				out.tables = append(out.tables, rows)
			}
		}
		if hasClass(n, "fcf-jogo-card") {
			match, rowErr := parseMatch(n, phase, group, round, year, loc, out.clubs)
			if rowErr != nil {
				err = rowErr
				return
			}
			out.matches = append(out.matches, match)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if err != nil {
				return
			}
			walk(c, phase, round)
		}
	}
	walk(root, "", "")
	if err != nil {
		return page{}, err
	}
	return out, nil
}

func parseTable(table *html.Node, group string, order int, clubs map[string]club) ([]domain.StandingInput, error) {
	var header []string
	var rows []domain.StandingInput
	var rowErr error
	var walk func(*html.Node, bool)
	walk = func(n *html.Node, head bool) {
		if rowErr != nil || n.Type != html.ElementNode {
			return
		}
		switch n.Data {
		case "thead":
			head = true
		case "tbody":
			head = false
		case "tr":
			if head {
				header = headerLabels(n)
			} else if len(header) > 0 {
				row, ok, err := parseStandingRow(n, header, group, order, clubs)
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
		for c := n.FirstChild; c != nil; c = c.NextSibling {
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
			labels = append(labels, fold(attr(c, "title")+" "+textOf(c)))
		}
	}
	return labels
}

func parseStandingRow(tr *html.Node, header []string, group string, order int, clubs map[string]club) (domain.StandingInput, bool, error) {
	var cells []string
	var clubName, clubID string
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || (c.Data != "td" && c.Data != "th") {
			continue
		}
		cells = append(cells, strings.TrimSpace(textOf(c)))
		if hasClass(c, "fcf-col-clube") || (len(cells) == 2 && clubName == "") {
			clubName, clubID = clubFrom(c)
		}
	}
	if clubName == "" || len(cells) == 0 {
		return domain.StandingInput{}, false, nil
	}
	if clubID == "" {
		clubID = "name:" + slug(clubName)
	}
	remember(clubs, clubID, clubName)
	row := domain.StandingInput{
		ExternalTeamID: clubID,
		GroupName:      group,
		GroupOrder:     order,
	}
	stats := map[string]int{}
	for i, label := range header {
		if i >= len(cells) {
			break
		}
		key := statKey(label)
		if key == "" || key == "club" {
			continue
		}
		if key == "pos" {
			pos, err := atoiCell(cells[i])
			if err != nil || pos <= 0 {
				return domain.StandingInput{}, false, fmt.Errorf("standing position %q", cells[i])
			}
			row.Position = pos
			continue
		}
		value, err := atoiCell(cells[i])
		if err != nil {
			return domain.StandingInput{}, false, fmt.Errorf("standing %s %q: %w", key, cells[i], err)
		}
		stats[key] = value
	}
	if row.Position <= 0 {
		return domain.StandingInput{}, false, fmt.Errorf("standing for %s has no position", clubName)
	}
	row.Points = stats["pts"]
	row.Played = stats["j"]
	row.Wins = stats["v"]
	row.Draws = stats["e"]
	row.Losses = stats["d"]
	row.GoalsFor = stats["gp"]
	row.GoalsAgainst = stats["gc"]
	row.GoalDifference = stats["sg"]
	row.YellowCards = stats["ca"]
	row.RedCards = stats["cv"]
	row.WinPercentage = stats["pct"]
	return row, true, nil
}

func statKey(label string) string {
	switch {
	case strings.Contains(label, "PONTO") || strings.TrimSpace(label) == "PTS":
		return "pts"
	case strings.Contains(label, "JOGO"):
		return "j"
	case strings.Contains(label, "VITOR"):
		return "v"
	case strings.Contains(label, "EMPAT"):
		return "e"
	case strings.Contains(label, "DERROT"):
		return "d"
	case strings.Contains(label, "CONTRA"):
		return "gc"
	case strings.Contains(label, "PRO") || strings.Contains(label, "MARC"):
		return "gp"
	case strings.Contains(label, "SALDO"):
		return "sg"
	case strings.Contains(label, "AMAREL") || strings.TrimSpace(label) == "CA":
		return "ca"
	case strings.Contains(label, "VERMELH") || strings.TrimSpace(label) == "CV":
		return "cv"
	case strings.Contains(label, "%") || strings.Contains(label, "APROVEIT"):
		return "pct"
	case strings.Contains(label, "CLUBE") || strings.Contains(label, "TIME"):
		return "club"
	case strings.TrimSpace(label) == "#" || strings.Contains(label, "POS"):
		return "pos"
	default:
		return ""
	}
}

func parseMatch(card *html.Node, phase, group, round string, year int, loc *time.Location, clubs map[string]club) (domain.MatchInput, error) {
	// A panel may leave data-fase-nome empty. The group title is the next
	// published name. If that is empty too, the round label is the name.
	if strings.TrimSpace(phase) == "" {
		phase = strings.TrimSpace(group)
	}
	if strings.TrimSpace(phase) == "" {
		phase = strings.TrimSpace(round)
	}
	if strings.TrimSpace(phase) == "" {
		return domain.MatchInput{}, fmt.Errorf("match has no phase")
	}
	roundNumber, err := roundNumber(round)
	if err != nil {
		return domain.MatchInput{}, err
	}
	homeName, homeID := side(card, "fcf-time--mandante")
	awayName, awayID := side(card, "fcf-time--visitante")
	if homeName == "" || awayName == "" {
		return domain.MatchInput{}, fmt.Errorf("match is missing a club")
	}
	if homeID == "" {
		homeID = "name:" + slug(homeName)
	}
	if awayID == "" {
		awayID = "name:" + slug(awayName)
	}
	if homeID == awayID {
		return domain.MatchInput{}, fmt.Errorf("match lists the same club twice")
	}
	remember(clubs, homeID, homeName)
	remember(clubs, awayID, awayName)
	jogo := jogoNumber(textOf(findClass(card, "fcf-jogo-numero")))
	if jogo == "" {
		jogo = "0"
	}
	match := domain.MatchInput{
		ExternalID:     slug(phase) + "-r" + strconv.Itoa(roundNumber) + "-j" + jogo + "-" + homeID + "-" + awayID,
		MatchNumber:    atoiOrZero(jogo),
		RoundNumber:    roundNumber,
		Phase:          phase,
		HomeExternalID: homeID,
		AwayExternalID: awayID,
		State:          "SC",
	}
	when, err := kickoff(textOf(findClass(card, "fcf-jogo-data")), year, loc)
	if err != nil {
		return domain.MatchInput{}, err
	}
	match.Kickoff = when
	venue, city := splitVenue(textOf(findClass(card, "fcf-jogo-estadio")))
	match.Venue = venue
	match.City = city
	home, away, scored, err := scores(card)
	if err != nil {
		return domain.MatchInput{}, err
	}
	class := attr(card, "class")
	switch {
	case strings.Contains(class, "adiado"):
		match.Status = domain.StatusPostponed
	case strings.Contains(class, "cancel"):
		match.Status = domain.StatusCancelled
	case strings.Contains(class, "suspens"):
		match.Status = domain.StatusSuspended
	case scored:
		match.Status = domain.StatusFinished
		match.HomeScore = home
		match.AwayScore = away
	case strings.Contains(class, "realizado"):
		return domain.MatchInput{}, fmt.Errorf("match %s is finished and has no score", match.ExternalID)
	default:
		match.Status = domain.StatusScheduled
	}
	return match, nil
}

func scores(card *html.Node) (*int, *int, bool, error) {
	placar := findClass(card, "fcf-placar")
	if placar == nil {
		return nil, nil, false, nil
	}
	var nums []int
	for c := placar.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && hasClass(c, "fcf-gol") {
			value, err := atoiCell(textOf(c))
			if err != nil {
				return nil, nil, false, fmt.Errorf("score %q: %w", textOf(c), err)
			}
			nums = append(nums, value)
		}
	}
	switch len(nums) {
	case 0:
		return nil, nil, false, nil
	case 2:
		return &nums[0], &nums[1], true, nil
	default:
		return nil, nil, false, fmt.Errorf("score has %d figures", len(nums))
	}
}

func side(card *html.Node, class string) (string, string) {
	node := findClass(card, class)
	if node == nil {
		return "", ""
	}
	return clubFrom(node)
}

func clubFrom(n *html.Node) (string, string) {
	var name, id string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "img" {
			if alt := strings.TrimSpace(attr(node, "alt")); alt != "" && name == "" {
				name = alt
			}
			if id == "" {
				id = logoID(attr(node, "src"))
			}
		}
		if node.Type == html.ElementNode && node.Data == "span" {
			if text := strings.TrimSpace(textOf(node)); text != "" {
				name = text
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return name, id
}

func logoID(src string) string {
	path := pathOf(src)
	base := path
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	base = strings.TrimSuffix(base, ".png")
	base = strings.TrimSuffix(base, ".jpg")
	base = strings.TrimSuffix(base, ".jpeg")
	base = strings.TrimSuffix(base, ".webp")
	base = strings.TrimSuffix(base, ".svg")
	if base == "" || strings.HasPrefix(src, "data:") || strings.Contains(base, " ") {
		return ""
	}
	return base
}

func kickoff(raw string, year int, loc *time.Location) (*time.Time, error) {
	m := clockRE.FindStringSubmatch(raw)
	if m == nil {
		return nil, nil
	}
	day, _ := strconv.Atoi(m[1])
	month, _ := strconv.Atoi(m[2])
	got, _ := strconv.Atoi(m[3])
	if got != year {
		return nil, fmt.Errorf("kickoff year %d is not %d", got, year)
	}
	if m[4] == "" {
		return nil, nil
	}
	hour, _ := strconv.Atoi(m[4])
	minute, _ := strconv.Atoi(m[5])
	when := time.Date(got, time.Month(month), day, hour, minute, 0, 0, loc).UTC()
	return &when, nil
}

func splitVenue(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	parts := strings.SplitN(raw, " / ", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return raw, ""
}

func roundNumber(raw string) (int, error) {
	// Knockout panels publish the phase name ("SEMIFINAIS", "FINAL") and no
	// round number. The phase is stored on the match. Those matches use round 1.
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 1, nil
	}
	m := roundRE.FindStringSubmatch(raw)
	if m == nil {
		return 1, nil
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 || n > 40 {
		return 0, fmt.Errorf("round %q is out of range", raw)
	}
	return n, nil
}

func jogoNumber(raw string) string {
	m := jogoRE.FindStringSubmatch(raw)
	if m == nil {
		return ""
	}
	return m[1]
}

func atoiOrZero(raw string) int {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return n
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
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func hasClass(n *html.Node, class string) bool {
	for _, part := range strings.Fields(attr(n, "class")) {
		if part == class {
			return true
		}
	}
	return false
}

func attr(n *html.Node, key string) string {
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

func findClass(n *html.Node, class string) *html.Node {
	if n == nil {
		return nil
	}
	if n.Type == html.ElementNode && hasClass(n, class) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findClass(c, class); found != nil {
			return found
		}
	}
	return nil
}

package fbf

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	nethtml "golang.org/x/net/html"

	"github.com/czarist/api_brasileirao/internal/domain"
)

// Phase is one published stage of a season.
type Phase struct {
	ID   string
	Name string
}

// Club is the published name and the short printed on a match card.
type Club struct {
	Name  string
	Short string
}

// Page is the matches and tables of one phase.
type Page struct {
	Matches []domain.MatchInput
	Tables  [][]domain.StandingInput
	Clubs   map[string]Club
}

var (
	svgRE       = regexp.MustCompile(`(?is)<svg\b[^>]*>.*?</svg>`)
	xmlRE       = regexp.MustCompile(`(?is)<\?xml[^>]*\?>`)
	doctypeRE   = regexp.MustCompile(`(?is)<!DOCTYPE[^>]*>`)
	rodadaRE    = regexp.MustCompile(`(?i)^(?:Rodada\s+([0-9]+)|([0-9]+)\s*(?:ª|º|°)?\s*Rodada)$`)
	bareRoundRE = regexp.MustCompile(`^[0-9]{1,2}$`)
	penaltySpan = regexp.MustCompile(`^\([0-9]+\)$`)
	whenRE      = regexp.MustCompile(`(\d{2})/(\d{2})/(\d{4})(?:\s+(\d{2}):(\d{2}))?`)
	matchPath   = regexp.MustCompile(`(?i)/jogo/detalhes/([0-9]+)`)
	seasonPath  = regexp.MustCompile(`^/competicoes/([12])/([0-9]{4})$`)
)

// ParseYears reads the season select. The selected attribute is ignored:
// some division pages mark every option selected. An option whose path is
// another division is skipped. A year whose label disagrees with the path fails.
func ParseYears(page, division string) ([]int, error) {
	doc, err := parseDoc(page)
	if err != nil {
		return nil, err
	}
	var years []int
	seen := map[int]struct{}{}
	foundSelect := false
	var walk func(*nethtml.Node) error
	walk = func(n *nethtml.Node) error {
		if n.Type == nethtml.ElementNode && n.Data == "select" {
			foundSelect = true
		}
		if n.Type == nethtml.ElementNode && n.Data == "option" {
			year, ok, err := optionYear(n, division)
			if err != nil {
				return err
			}
			if ok {
				if _, dup := seen[year]; !dup {
					seen[year] = struct{}{}
					years = append(years, year)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(doc); err != nil {
		return nil, err
	}
	if !foundSelect {
		return nil, fmt.Errorf("catalog has no season select")
	}
	return years, nil
}

func optionYear(n *nethtml.Node, division string) (int, bool, error) {
	raw := strings.TrimSpace(attr(n, "value"))
	if raw == "" {
		return 0, false, nil
	}
	parsed, err := urlParse(raw)
	if err != nil {
		return 0, false, nil
	}
	found := seasonPath.FindStringSubmatch(strings.TrimRight(parsed, "/"))
	if found == nil || found[1] != division {
		return 0, false, nil
	}
	year, err := strconv.Atoi(found[2])
	if err != nil || year < 1990 || year > 2100 {
		return 0, false, fmt.Errorf("catalog year %s is out of range", found[2])
	}
	label := textContent(n)
	if label != found[2] {
		return 0, false, fmt.Errorf("catalog option %s is labeled %q", found[2], label)
	}
	return year, true, nil
}

// ParsePhases reads the phase tabs for one season, in page order.
// A repeated phase id with the same name is skipped. A link for another
// division or year is skipped.
func ParsePhases(page, division string, year int) ([]Phase, error) {
	doc, err := parseDoc(page)
	if err != nil {
		return nil, err
	}
	var phases []Phase
	seen := map[string]string{}
	var walk func(*nethtml.Node) error
	walk = func(n *nethtml.Node) error {
		if n.Type == nethtml.ElementNode && n.Data == "ul" && hasClass(n, "fase-links") {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				phase, ok, err := phaseLink(c, division, year)
				if err != nil {
					return err
				}
				if !ok {
					continue
				}
				if previous, dup := seen[phase.ID]; dup {
					if previous != phase.Name {
						return fmt.Errorf("phase %s is published as %q and %q", phase.ID, previous, phase.Name)
					}
					continue
				}
				seen[phase.ID] = phase.Name
				phases = append(phases, phase)
			}
			return nil
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(doc); err != nil {
		return nil, err
	}
	return phases, nil
}

func phaseLink(n *nethtml.Node, division string, year int) (Phase, bool, error) {
	if n.Type != nethtml.ElementNode {
		var found Phase
		var ok bool
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			phase, phaseOK, err := phaseLink(c, division, year)
			if err != nil {
				return Phase{}, false, err
			}
			if phaseOK && !ok {
				found = phase
				ok = true
			}
		}
		return found, ok, nil
	}
	if n.Data == "a" {
		id, matchYear, matchDivision, ok := phasePath(attr(n, "href"))
		if !ok || matchDivision != division || matchYear != year {
			return Phase{}, false, nil
		}
		name := textContent(n)
		if name == "" {
			return Phase{}, false, fmt.Errorf("phase %s has no published name", id)
		}
		return Phase{ID: id, Name: name}, true, nil
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		phase, ok, err := phaseLink(c, division, year)
		if err != nil || ok {
			return phase, ok, err
		}
	}
	return Phase{}, false, nil
}

func phasePath(raw string) (id string, year int, division string, ok bool) {
	path, err := urlParse(raw)
	if err != nil {
		return "", 0, "", false
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "competicoes" || !adultDivision(parts[1]) || !fourDigit(parts[2]) || !digits.MatchString(parts[3]) {
		return "", 0, "", false
	}
	year, err = strconv.Atoi(parts[2])
	if err != nil {
		return "", 0, "", false
	}
	return parts[3], year, parts[1], true
}

// PublishedPath reads og:url when the page prints one.
func PublishedPath(page string) (string, bool) {
	doc, err := parseDoc(page)
	if err != nil {
		return "", false
	}
	var path string
	var walk func(*nethtml.Node)
	walk = func(n *nethtml.Node) {
		if path != "" {
			return
		}
		if n.Type == nethtml.ElementNode && n.Data == "meta" && strings.EqualFold(attr(n, "property"), "og:url") {
			parsed, err := urlParse(attr(n, "content"))
			if err == nil {
				path = strings.TrimRight(parsed, "/")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return path, path != ""
}

func pathMatchesSeason(path, division string, year int) bool {
	path = strings.TrimRight(path, "/")
	prefix := fmt.Sprintf("/competicoes/%s/%d", division, year)
	if path == prefix {
		return true
	}
	rest, ok := strings.CutPrefix(path, prefix+"/")
	return ok && digits.MatchString(rest)
}

// ParsePage reads one phase. Empty score spans stay unset. A decimal win
// percentage is rounded to the nearest integer because the stored column is
// an integer. Cards are not in this table, so yellow and red are left at 0.
func ParsePage(page, phase string, year int, loc *time.Location) (Page, error) {
	doc, err := parseDoc(page)
	if err != nil {
		return Page{}, err
	}
	clubs := map[string]Club{}
	tables, err := parseStandings(doc, clubs)
	if err != nil {
		return Page{}, err
	}
	matches, err := parseMatches(doc, phase, year, loc, clubs)
	if err != nil {
		return Page{}, err
	}
	matches, err = linkPlaceholderClubs(matches, clubs)
	if err != nil {
		return Page{}, err
	}
	return Page{Matches: matches, Tables: tables, Clubs: clubs}, nil
}

func parseMatches(doc *nethtml.Node, phase string, year int, loc *time.Location, clubs map[string]Club) ([]domain.MatchInput, error) {
	var matches []domain.MatchInput
	var walk func(*nethtml.Node) error
	walk = func(n *nethtml.Node) error {
		if n.Type == nethtml.ElementNode && n.Data == "a" {
			found := matchPath.FindStringSubmatch(attr(n, "href"))
			if found != nil {
				match, err := matchFrom(n, found[1], phase, year, loc, clubs)
				if err != nil {
					return err
				}
				matches = append(matches, match)
				return nil
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(doc); err != nil {
		return nil, err
	}
	return matches, nil
}

func matchFrom(anchor *nethtml.Node, id, phase string, year int, loc *time.Location, clubs map[string]Club) (domain.MatchInput, error) {
	card := cardAncestor(anchor)
	if card == nil {
		return domain.MatchInput{}, fmt.Errorf("match %s has no card", id)
	}
	round, err := roundOf(card)
	if err != nil {
		return domain.MatchInput{}, fmt.Errorf("match %s: %w", id, err)
	}
	home, away, err := sides(card)
	if err != nil {
		return domain.MatchInput{}, fmt.Errorf("match %s: %w", id, err)
	}
	homeID, err := clubKey(home.path, home.short)
	if err != nil {
		return domain.MatchInput{}, fmt.Errorf("match %s: %w", id, err)
	}
	awayID, err := clubKey(away.path, away.short)
	if err != nil {
		return domain.MatchInput{}, fmt.Errorf("match %s: %w", id, err)
	}
	if homeID == awayID {
		return domain.MatchInput{}, fmt.Errorf("match %s lists the same club twice", id)
	}
	homeScore, awayScore, homePenalties, awayPenalties, err := scores(card)
	if err != nil {
		return domain.MatchInput{}, fmt.Errorf("match %s: %w", id, err)
	}
	kickoff, venue, err := clockAndVenue(card, year, loc)
	if err != nil {
		return domain.MatchInput{}, fmt.Errorf("match %s: %w", id, err)
	}
	status, err := statusOf(textContent(card), homeScore != nil)
	if err != nil {
		return domain.MatchInput{}, fmt.Errorf("match %s: %w", id, err)
	}
	remember(clubs, homeID, "", home.short)
	remember(clubs, awayID, "", away.short)
	return domain.MatchInput{
		ExternalID:     id,
		RoundNumber:    round,
		Phase:          phase,
		HomeExternalID: homeID,
		AwayExternalID: awayID,
		Kickoff:        kickoff,
		Status:         status,
		HomeScore:      homeScore,
		AwayScore:      awayScore,
		HomePenalties:  homePenalties,
		AwayPenalties:  awayPenalties,
		Venue:          venue,
	}, nil
}

type side struct {
	path  string
	short string
}

func sides(card *nethtml.Node) (side, side, error) {
	box := findClass(card, "flex-confronto-card")
	if box == nil {
		return side{}, side{}, fmt.Errorf("match has no clubs")
	}
	var imgs []*nethtml.Node
	var walk func(*nethtml.Node)
	walk = func(n *nethtml.Node) {
		if n.Type == nethtml.ElementNode && n.Data == "img" {
			if _, err := clubPath(attr(n, "src")); err == nil {
				imgs = append(imgs, n)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(box)
	if len(imgs) != 2 {
		return side{}, side{}, fmt.Errorf("match has %d club badges", len(imgs))
	}
	home, err := sideFrom(imgs[0])
	if err != nil {
		return side{}, side{}, err
	}
	away, err := sideFrom(imgs[1])
	if err != nil {
		return side{}, side{}, err
	}
	return home, away, nil
}

func sideFrom(img *nethtml.Node) (side, error) {
	path, err := clubPath(attr(img, "src"))
	if err != nil {
		return side{}, err
	}
	short := ""
	if img.Parent != nil {
		if span := findClass(img.Parent, "time-confronto"); span != nil {
			short = textContent(span)
		}
	}
	return side{path: path, short: short}, nil
}

func scores(card *nethtml.Node) (*int, *int, *int, *int, error) {
	box := findClass(card, "hora-confronto-card")
	if box == nil {
		return nil, nil, nil, nil, fmt.Errorf("match has no score")
	}
	var numbers []string
	var penalties []string
	for c := box.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != nethtml.ElementNode || c.Data != "span" {
			continue
		}
		text := textContent(c)
		if text == "" {
			continue
		}
		if penaltySpan.MatchString(text) {
			penalties = append(penalties, text)
			continue
		}
		numbers = append(numbers, text)
	}
	home, away, err := pairFigures(numbers, "score")
	if err != nil {
		return nil, nil, nil, nil, err
	}
	homePenalties, awayPenalties, err := penaltyPair(penalties)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return home, away, homePenalties, awayPenalties, nil
}

// penaltyPair reads a shootout. Older cards print (0) on matches that were
// not decided on penalties. Those zeros are a placeholder. A shootout
// publishes two figures and at least one of them is non-zero, so a published
// 0 on the other side is kept.
func penaltyPair(penalties []string) (*int, *int, error) {
	if len(penalties) == 0 {
		return nil, nil, nil
	}
	allZero := true
	for _, item := range penalties {
		if item != "(0)" {
			allZero = false
			break
		}
	}
	if allZero {
		return nil, nil, nil
	}
	return pairFigures(penalties, "penalty")
}

func pairFigures(numbers []string, kind string) (*int, *int, error) {
	switch len(numbers) {
	case 0:
		return nil, nil, nil
	case 2:
		leftText := numbers[0]
		rightText := numbers[1]
		if kind == "penalty" {
			leftText = strings.Trim(leftText, "()")
			rightText = strings.Trim(rightText, "()")
		}
		left, err := strconv.Atoi(leftText)
		if err != nil || left < 0 {
			return nil, nil, fmt.Errorf("%s %q", kind, numbers[0])
		}
		right, err := strconv.Atoi(rightText)
		if err != nil || right < 0 {
			return nil, nil, fmt.Errorf("%s %q", kind, numbers[1])
		}
		return &left, &right, nil
	default:
		return nil, nil, fmt.Errorf("%s has %d figures", kind, len(numbers))
	}
}

func clockAndVenue(card *nethtml.Node, year int, loc *time.Location) (*time.Time, string, error) {
	box := findClass(card, "data-hora-card-jogo")
	if box == nil {
		return nil, "", fmt.Errorf("match has no clock")
	}
	text := textContent(box)
	found := whenRE.FindStringSubmatchIndex(text)
	if found == nil {
		if strings.Contains(fold(text), "DEFINIR") {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("clock %q", text)
	}
	publishedYear, err := strconv.Atoi(text[found[6]:found[7]])
	if err != nil || publishedYear != year {
		return nil, "", fmt.Errorf("clock year %s does not match %d", text[found[6]:found[7]], year)
	}
	venue := strings.TrimSpace(text[:found[0]])
	if strings.Contains(fold(venue), "DEFINIR") {
		venue = ""
	}
	if found[8] < 0 || found[10] < 0 {
		return nil, venue, nil
	}
	day, _ := strconv.Atoi(text[found[2]:found[3]])
	month, _ := strconv.Atoi(text[found[4]:found[5]])
	hour, _ := strconv.Atoi(text[found[8]:found[9]])
	minute, _ := strconv.Atoi(text[found[10]:found[11]])
	when := time.Date(publishedYear, time.Month(month), day, hour, minute, 0, 0, loc)
	if when.Day() != day || when.Month() != time.Month(month) || when.Hour() != hour {
		return nil, "", fmt.Errorf("clock %q", text)
	}
	utc := when.UTC()
	return &utc, venue, nil
}

func statusOf(text string, scored bool) (domain.MatchStatus, error) {
	folded := fold(text)
	switch {
	case strings.Contains(folded, "CANCEL"):
		if scored {
			return "", fmt.Errorf("cancelled match has a score")
		}
		return domain.StatusCancelled, nil
	case strings.Contains(folded, "SUSPENS"):
		if scored {
			return "", fmt.Errorf("suspended match has a score")
		}
		return domain.StatusSuspended, nil
	case strings.Contains(folded, "ADIAD"), strings.Contains(folded, "REMARC"):
		if scored {
			return "", fmt.Errorf("postponed match has a score")
		}
		return domain.StatusPostponed, nil
	default:
		if scored {
			return domain.StatusFinished, nil
		}
		return domain.StatusScheduled, nil
	}
}

func roundOf(card *nethtml.Node) (int, error) {
	for node := card; node != nil; node = node.Parent {
		if hasClass(node, "carousel-item") {
			break
		}
		for sib := node.PrevSibling; sib != nil; sib = sib.PrevSibling {
			if number, ok := findRodada(sib); ok {
				return number, nil
			}
		}
	}
	return 0, fmt.Errorf("match has no round")
}

func rodadaNumber(text string) (int, bool) {
	found := rodadaRE.FindStringSubmatch(text)
	raw := ""
	if found != nil {
		raw = found[1]
		if raw == "" {
			raw = found[2]
		}
	} else if bareRoundRE.MatchString(text) {
		// Some knockout carousels print only the round number.
		raw = text
	}
	if raw == "" {
		return 0, false
	}
	number, err := strconv.Atoi(raw)
	if err != nil || number <= 0 || number > 40 {
		return 0, false
	}
	return number, true
}

func findRodada(n *nethtml.Node) (int, bool) {
	if n.Type == nethtml.ElementNode && n.Data == "h4" {
		if number, ok := rodadaNumber(textContent(n)); ok {
			return number, true
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if number, ok := findRodada(c); ok {
			return number, true
		}
	}
	return 0, false
}

func cardAncestor(n *nethtml.Node) *nethtml.Node {
	for n != nil {
		if n.Type == nethtml.ElementNode && hasClass(n, "flex-column") {
			return n
		}
		n = n.Parent
	}
	return nil
}

func parseStandings(doc *nethtml.Node, clubs map[string]Club) ([][]domain.StandingInput, error) {
	var tables [][]domain.StandingInput
	var walk func(*nethtml.Node) error
	walk = func(n *nethtml.Node) error {
		if n.Type == nethtml.ElementNode && n.Data == "table" {
			rows, ok, err := tableRows(n, clubs)
			if err != nil {
				return err
			}
			if ok && len(rows) > 0 {
				group := groupBefore(n)
				for i := range rows {
					if rows[i].GroupName == "" {
						rows[i].GroupName = group
					}
				}
				tables = append(tables, splitGroups(rows)...)
			}
			return nil
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(doc); err != nil {
		return nil, err
	}
	return tables, nil
}

func splitGroups(rows []domain.StandingInput) [][]domain.StandingInput {
	if len(rows) == 0 {
		return nil
	}
	var tables [][]domain.StandingInput
	current := rows[0].GroupName
	start := 0
	for i := range rows {
		if rows[i].GroupName != current {
			tables = append(tables, rows[start:i])
			current = rows[i].GroupName
			start = i
		}
	}
	tables = append(tables, rows[start:])
	return tables
}

func tableRows(table *nethtml.Node, clubs map[string]Club) ([]domain.StandingInput, bool, error) {
	headers := headerLabels(table)
	columns, ok := standingColumns(headers)
	if !ok {
		return nil, false, nil
	}
	var rows []domain.StandingInput
	seen := map[string]struct{}{}
	group := ""
	for _, tr := range bodyRows(table) {
		cells := childElements(tr, "td")
		if len(cells) == 0 {
			continue
		}
		if len(cells) == 1 && attr(cells[0], "colspan") != "" {
			label := textContent(cells[0])
			if _, isRound := rodadaNumber(label); label != "" && !isNoiseHeading(label) && !isRound {
				group = label
			}
			continue
		}
		row, club, parsed, err := standingFrom(cells, columns, group)
		if err != nil {
			return nil, false, err
		}
		if !parsed {
			continue
		}
		if _, dup := seen[row.ExternalTeamID]; dup {
			return nil, false, fmt.Errorf("standings list %s twice", club.Name)
		}
		seen[row.ExternalTeamID] = struct{}{}
		remember(clubs, row.ExternalTeamID, club.Name, "")
		rows = append(rows, row)
	}
	return rows, true, nil
}

func standingFrom(cells []*nethtml.Node, columns map[string]int, group string) (domain.StandingInput, Club, bool, error) {
	clubIndex, ok := columns["CLUB"]
	if !ok || clubIndex >= len(cells) {
		return domain.StandingInput{}, Club{}, false, nil
	}
	clubCell := cells[clubIndex]
	img := findTag(clubCell, "img")
	if img == nil {
		return domain.StandingInput{}, Club{}, false, nil
	}
	path, err := clubPath(attr(img, "src"))
	if err != nil {
		return domain.StandingInput{}, Club{}, false, err
	}
	nameNode := findClass(clubCell, "team-meta__place")
	if nameNode == nil {
		return domain.StandingInput{}, Club{}, false, fmt.Errorf("standings row %s has no published name", path)
	}
	name := textContent(nameNode)
	if name == "" {
		return domain.StandingInput{}, Club{}, false, fmt.Errorf("standings row %s has no published name", path)
	}
	position, err := leadingInt(textContent(clubCell))
	if err != nil || position <= 0 {
		return domain.StandingInput{}, Club{}, false, fmt.Errorf("standings row %s has no position", name)
	}
	value := func(label string) (int, error) {
		index := columns[label]
		if index >= len(cells) {
			return 0, fmt.Errorf("standings row %s is missing %s", name, label)
		}
		if label == "%" {
			return publishedPercent(textContent(cells[index]))
		}
		return publishedInt(textContent(cells[index]))
	}
	points, err := value("P")
	if err != nil {
		return domain.StandingInput{}, Club{}, false, err
	}
	played, err := value("J")
	if err != nil {
		return domain.StandingInput{}, Club{}, false, err
	}
	wins, err := value("V")
	if err != nil {
		return domain.StandingInput{}, Club{}, false, err
	}
	draws, err := value("E")
	if err != nil {
		return domain.StandingInput{}, Club{}, false, err
	}
	losses, err := value("D")
	if err != nil {
		return domain.StandingInput{}, Club{}, false, err
	}
	goalsFor, err := value("GP")
	if err != nil {
		return domain.StandingInput{}, Club{}, false, err
	}
	goalsAgainst, err := value("GC")
	if err != nil {
		return domain.StandingInput{}, Club{}, false, err
	}
	goalDifference, err := value("SG")
	if err != nil {
		return domain.StandingInput{}, Club{}, false, err
	}
	percent, err := value("%")
	if err != nil {
		return domain.StandingInput{}, Club{}, false, err
	}
	id, err := clubKey(path, name)
	if err != nil {
		return domain.StandingInput{}, Club{}, false, err
	}
	return domain.StandingInput{
		ExternalTeamID: id,
		GroupName:      group,
		Position:       position,
		Played:         played,
		Wins:           wins,
		Draws:          draws,
		Losses:         losses,
		GoalsFor:       goalsFor,
		GoalsAgainst:   goalsAgainst,
		GoalDifference: goalDifference,
		Points:         points,
		YellowCards:    0,
		RedCards:       0,
		WinPercentage:  percent,
	}, Club{Name: name}, true, nil
}

func headerLabels(table *nethtml.Node) []string {
	for c := table.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == nethtml.ElementNode && c.Data == "thead" {
			var labels []string
			for _, tr := range childElements(c, "tr") {
				for _, cell := range childElements(tr, "th") {
					labels = append(labels, textContent(cell))
				}
			}
			return labels
		}
	}
	return nil
}

func standingColumns(headers []string) (map[string]int, bool) {
	columns := map[string]int{}
	for i, header := range headers {
		switch fold(header) {
		case "POSICAO", "POS":
			columns["CLUB"] = i
		case "P", "J", "V", "E", "D", "GP", "GC", "SG", "%":
			columns[fold(header)] = i
		}
	}
	for _, label := range []string{"CLUB", "P", "J", "V", "E", "D", "GP", "GC", "SG", "%"} {
		if _, ok := columns[label]; !ok {
			return nil, false
		}
	}
	return columns, true
}

func bodyRows(table *nethtml.Node) []*nethtml.Node {
	var rows []*nethtml.Node
	for c := table.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != nethtml.ElementNode {
			continue
		}
		if c.Data == "tbody" {
			rows = append(rows, childElements(c, "tr")...)
		}
	}
	return rows
}

func groupBefore(n *nethtml.Node) string {
	for sib := n.PrevSibling; sib != nil; sib = sib.PrevSibling {
		if text, ok := firstHeading(sib); ok {
			if _, isRound := rodadaNumber(text); isRound || isNoiseHeading(text) {
				return ""
			}
			return text
		}
	}
	if n.Parent != nil && n.Parent.Data != "body" && n.Parent.Data != "html" {
		return groupBefore(n.Parent)
	}
	return ""
}

func firstHeading(n *nethtml.Node) (string, bool) {
	if n.Type == nethtml.ElementNode && n.Data == "h4" {
		text := textContent(n)
		if text != "" {
			return text, true
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if text, ok := firstHeading(c); ok {
			return text, true
		}
	}
	return "", false
}

func isNoiseHeading(text string) bool {
	switch fold(text) {
	case "CLASSIFICACAO", "POSICAO", "P", "J", "V", "E", "D", "GP", "GC", "SG", "%":
		return true
	default:
		return false
	}
}

// placeholderBadge is the image the catalog prints when a club has no badge.
// Several clubs share that file, so it is not an identity.
func placeholderBadge(path string) bool {
	base := path
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	return strings.EqualFold(base, "default.png")
}

func clubKey(path, label string) (string, error) {
	if !placeholderBadge(path) {
		return path, nil
	}
	id := nameID(label)
	if id == "" {
		return "", fmt.Errorf("placeholder badge has no published name")
	}
	return id, nil
}

// nameID is the identity of a club published without its own badge.
func nameID(name string) string {
	folded := fold(name)
	var b strings.Builder
	b.WriteString("name:")
	dash := false
	for _, r := range folded {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteByte(byte(r + ('a' - 'A')))
			dash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > len("name:") {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	id := strings.TrimRight(b.String(), "-")
	if id == "name:" {
		return ""
	}
	return id
}

// linkPlaceholderClubs rewrites a match side that only printed a short next
// to the shared badge onto the standings club whose name starts with that
// short. A short that matches two published names fails the page.
func linkPlaceholderClubs(matches []domain.MatchInput, clubs map[string]Club) ([]domain.MatchInput, error) {
	type named struct {
		id   string
		name string
	}
	var namedClubs []named
	for id, club := range clubs {
		if strings.HasPrefix(id, "name:") && club.Name != "" {
			namedClubs = append(namedClubs, named{id: id, name: club.Name})
		}
	}
	sort.Slice(namedClubs, func(i, j int) bool {
		return namedClubs[i].id < namedClubs[j].id
	})
	resolve := func(id string) (string, error) {
		club, ok := clubs[id]
		if !ok || !strings.HasPrefix(id, "name:") || club.Name != "" {
			return id, nil
		}
		short := fold(club.Short)
		if len(strings.ReplaceAll(short, " ", "")) < 3 {
			return id, nil
		}
		var hits []named
		for _, item := range namedClubs {
			if strings.HasPrefix(fold(item.name), short) {
				hits = append(hits, item)
			}
		}
		switch len(hits) {
		case 0:
			return id, nil
		case 1:
			remember(clubs, hits[0].id, "", club.Short)
			return hits[0].id, nil
		default:
			return "", fmt.Errorf("short %s matches more than one club", club.Short)
		}
	}
	for i := range matches {
		home, err := resolve(matches[i].HomeExternalID)
		if err != nil {
			return nil, fmt.Errorf("match %s: %w", matches[i].ExternalID, err)
		}
		away, err := resolve(matches[i].AwayExternalID)
		if err != nil {
			return nil, fmt.Errorf("match %s: %w", matches[i].ExternalID, err)
		}
		if home == away {
			return nil, fmt.Errorf("match %s lists the same club twice", matches[i].ExternalID)
		}
		matches[i].HomeExternalID = home
		matches[i].AwayExternalID = away
	}
	return matches, nil
}

func remember(clubs map[string]Club, id, name, short string) {
	if id == "" {
		return
	}
	club := clubs[id]
	if name != "" && club.Name == "" {
		club.Name = name
	}
	if short != "" && club.Short == "" {
		club.Short = short
	}
	clubs[id] = club
}

func clubPath(src string) (string, error) {
	src = strings.TrimSpace(html.UnescapeString(src))
	if src == "" {
		return "", fmt.Errorf("empty badge")
	}
	path, err := urlParse(src)
	if err != nil {
		return "", err
	}
	path = strings.TrimSpace(path)
	if strings.Contains(path, "..") || !strings.Contains(strings.ToLower(path), "/images/clubes/") {
		return "", fmt.Errorf("badge %q", src)
	}
	return path, nil
}

func urlParse(raw string) (string, error) {
	raw = strings.TrimSpace(html.UnescapeString(raw))
	if raw == "" {
		return "", fmt.Errorf("empty url")
	}
	if strings.HasPrefix(raw, "/") {
		if i := strings.IndexAny(raw, "?#"); i >= 0 {
			raw = raw[:i]
		}
		return raw, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	return parsed.Path, nil
}

func publishedInt(text string) (int, error) {
	text = strings.TrimSpace(text)
	text = strings.TrimRight(text, "*")
	text = strings.TrimSuffix(text, "º")
	text = strings.TrimSuffix(text, "°")
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, fmt.Errorf("empty number")
	}
	number, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("number %q", text)
	}
	return number, nil
}

// publishedPercent rounds the first decimal digit. 66.67 becomes 67 and
// 70.37 becomes 70. The source prints the percentage with decimals.
func publishedPercent(text string) (int, error) {
	text = strings.TrimSpace(text)
	text = strings.TrimRight(text, "*")
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

func leadingInt(text string) (int, error) {
	text = strings.TrimSpace(text)
	i := 0
	for i < len(text) && text[i] >= '0' && text[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, fmt.Errorf("no integer in %q", text)
	}
	return strconv.Atoi(text[:i])
}

func parseDoc(page string) (*nethtml.Node, error) {
	page = xmlRE.ReplaceAllString(page, "")
	page = doctypeRE.ReplaceAllString(page, "")
	page = svgRE.ReplaceAllString(page, "")
	doc, err := nethtml.Parse(strings.NewReader(page))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	return doc, nil
}

func childElements(n *nethtml.Node, tag string) []*nethtml.Node {
	var out []*nethtml.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == nethtml.ElementNode && c.Data == tag {
			out = append(out, c)
		}
	}
	return out
}

func findClass(n *nethtml.Node, class string) *nethtml.Node {
	if n.Type == nethtml.ElementNode && hasClass(n, class) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findClass(c, class); found != nil {
			return found
		}
	}
	return nil
}

func findTag(n *nethtml.Node, tag string) *nethtml.Node {
	if n.Type == nethtml.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findTag(c, tag); found != nil {
			return found
		}
	}
	return nil
}

func hasClass(n *nethtml.Node, class string) bool {
	for _, part := range strings.Fields(attr(n, "class")) {
		if part == class {
			return true
		}
	}
	return false
}

func attr(n *nethtml.Node, key string) string {
	for _, item := range n.Attr {
		if strings.EqualFold(item.Key, key) {
			return item.Val
		}
	}
	return ""
}

func textContent(n *nethtml.Node) string {
	var b strings.Builder
	var walk func(*nethtml.Node)
	walk = func(node *nethtml.Node) {
		if node.Type == nethtml.TextNode {
			b.WriteString(node.Data)
			b.WriteByte(' ')
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == nethtml.ElementNode && (c.Data == "script" || c.Data == "style" || c.Data == "svg") {
				continue
			}
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

func fold(text string) string {
	text = strings.ToUpper(text)
	text = strings.NewReplacer(
		"Á", "A", "À", "A", "Ã", "A", "Â", "A",
		"É", "E", "Ê", "E",
		"Í", "I",
		"Ó", "O", "Ô", "O", "Õ", "O",
		"Ú", "U",
		"Ç", "C",
		"º", " ", "ª", " ",
	).Replace(text)
	return strings.Join(strings.Fields(text), " ")
}

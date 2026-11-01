package fmf

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/czarist/api_brasileirao/internal/domain"
	nethtml "golang.org/x/net/html"
)

// Season is one year on a division's catalog select.
type Season struct {
	Year       int
	ExternalID string
	Name       string
}

// Page is one division season document.
type Page struct {
	Season    Season
	Teams     []domain.TeamInput
	Matches   []domain.MatchInput
	Standings []domain.StandingInput
}

var (
	optionRE = regexp.MustCompile(`<option([^>]*)>([^<]*)</option>`)
	valueRE  = regexp.MustCompile(`value="([0-9]+)"`)
	yearRE   = regexp.MustCompile(`(19|20)[0-9]{2}\s*$`)
	phaseRE  = regexp.MustCompile(`href="#(fase_1-[0-9]+)"[^>]*>([^<]+)</a>`)
	faseRE   = regexp.MustCompile(`id="(fase_1-[0-9]+)"`)
	roundRE  = regexp.MustCompile(`<h3 class="box-title_es">\s*RODADA\s+([0-9]+)\s*</h3>`)
	boldRE   = regexp.MustCompile(`<b>([^<]*)</b>`)
	dateRE   = regexp.MustCompile(`(?i)([0-9]{1,2}) de ([a-zA-Zà-üÀ-Ü]+) de ([0-9]{4})`)
	clockRE  = regexp.MustCompile(`^([0-9]{1,2}):([0-9]{2})$`)
	// The score row is three plain cells. A wider match would swallow the next fixture.
	// Older pages omit the game number and data-jogo, and publish a map code instead.
	matchRE = regexp.MustCompile(`class='col-md-1'>([^<]*)</div>[\s\S]{0,400}?align='right'>([^<]+)</div>[\s\S]{0,700}?<table width='100%'><tr><td>([^<]*)</td><td>([^<]*)</td><td>([^<]*)</td></tr></table>[\s\S]{0,500}?align='left'>([^<]+)</div>[\s\S]{0,300}?Jogo\s*([0-9]*)<br>([^<]*)<br>([^<]*)</div>[\s\S]{0,500}?(?:data-jogo="([0-9]+)"|GoogleMaps\.aspx\?cod=([0-9A-Fa-f]{16,}))`)
	imgRE   = regexp.MustCompile(`src="([^"]*escudos/[^"]+)"`)
	jogoRE  = regexp.MustCompile(`data-jogo="([0-9]+)"`)
	codRE   = regexp.MustCompile(`GoogleMaps\.aspx\?cod=([0-9A-Fa-f]{16,})`)
	logoRE  = regexp.MustCompile(`Foto_Logo_([0-9]+)`)
)

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

// ParseCatalog reads the year select for one division. Options for another
// division are ignored, so a page for the wrong division does not yield seasons.
func ParseCatalog(body []byte, division string) ([]Season, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, fmt.Errorf("catalog page is empty")
	}
	var seasons []Season
	seen := map[int]struct{}{}
	for _, match := range optionRE.FindAllSubmatch(body, -1) {
		attrs := string(match[1])
		label := clean(html.UnescapeString(string(match[2])))
		if !labelMatches(division, label) {
			continue
		}
		value := valueRE.FindStringSubmatch(attrs)
		if value == nil {
			return nil, fmt.Errorf("catalog option %q has no numeric id", label)
		}
		yearText := yearRE.FindString(label)
		year, err := strconv.Atoi(strings.TrimSpace(yearText))
		if err != nil {
			return nil, fmt.Errorf("catalog option %q has no year", label)
		}
		if _, ok := seen[year]; ok {
			return nil, fmt.Errorf("catalog lists %d twice", year)
		}
		seen[year] = struct{}{}
		seasons = append(seasons, Season{Year: year, ExternalID: value[1], Name: label})
	}
	if len(seasons) == 0 {
		return nil, fmt.Errorf("catalog has no seasons for division %s", division)
	}
	sort.Slice(seasons, func(i, j int) bool { return seasons[i].Year < seasons[j].Year })
	return seasons, nil
}

// ParsePage reads the selected season, its matches, and the classification
// tables. A later phase table that only repeats clubs already listed is kept
// out of the standings. The matches stay.
func ParsePage(body []byte, division string) (Page, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return Page{}, fmt.Errorf("season page is empty")
	}
	seasons, err := ParseCatalog(body, division)
	if err != nil {
		return Page{}, err
	}
	selected, err := selectedSeason(body, seasons)
	if err != nil {
		return Page{}, err
	}
	rawMatches, err := parseMatches(body)
	if err != nil {
		return Page{}, err
	}
	if len(rawMatches) == 0 {
		return Page{}, fmt.Errorf("season page has no matches")
	}
	rawRows, err := parseStandings(body)
	if err != nil {
		return Page{}, err
	}
	if len(rawRows) == 0 {
		return Page{}, fmt.Errorf("season page has no standings")
	}
	page, err := assemble(selected, rawMatches, rawRows)
	if err != nil {
		return Page{}, err
	}
	return page, nil
}

func selectedSeason(body []byte, seasons []Season) (Season, error) {
	byID := make(map[string]Season, len(seasons))
	for _, season := range seasons {
		byID[season.ExternalID] = season
	}
	var found *Season
	for _, match := range optionRE.FindAllSubmatch(body, -1) {
		attrs := string(match[1])
		if !strings.Contains(attrs, "selected") {
			continue
		}
		value := valueRE.FindStringSubmatch(attrs)
		if value == nil {
			continue
		}
		season, ok := byID[value[1]]
		if !ok {
			continue
		}
		if found != nil {
			return Season{}, fmt.Errorf("season page selects more than one year")
		}
		copy := season
		found = &copy
	}
	if found == nil {
		return Season{}, fmt.Errorf("season page does not select a catalog year")
	}
	return *found, nil
}

type rawMatch struct {
	id      string
	number  int
	round   int
	phase   string
	home    string
	away    string
	homeImg string
	awayImg string
	clock   string
	date    string
	homeSet bool
	awaySet bool
	homeN   int
	awayN   int
	venue   string
	city    string
}

func parseMatches(body []byte) ([]rawMatch, error) {
	text := string(body)
	start := strings.Index(text, `id="tab_1-1"`)
	end := strings.Index(text, `id="tab_2-2"`)
	if start < 0 || end < start {
		return nil, fmt.Errorf("season page has no match list")
	}
	chunk := text[start:end]
	names := map[string]string{}
	for _, match := range phaseRE.FindAllStringSubmatch(chunk, -1) {
		if _, ok := names[match[1]]; !ok {
			names[match[1]] = clean(html.UnescapeString(match[2]))
		}
	}
	marks := faseRE.FindAllStringSubmatchIndex(chunk, -1)
	if len(marks) == 0 {
		return nil, fmt.Errorf("season page has no phases")
	}
	var out []rawMatch
	seen := map[string]rawMatch{}
	for i, mark := range marks {
		id := chunk[mark[2]:mark[3]]
		phase := names[id]
		if phase == "" {
			return nil, fmt.Errorf("phase %s has no name", id)
		}
		next := len(chunk)
		if i+1 < len(marks) {
			next = marks[i+1][0]
		}
		rows, err := parsePhase(chunk[mark[0]:next], phase)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			prev, ok := seen[row.id]
			if !ok {
				seen[row.id] = row
				out = append(out, row)
				continue
			}
			if !sameMatch(prev, row) {
				return nil, fmt.Errorf("match %s is published twice with different details", row.id)
			}
			if prev.homeImg == "" && row.homeImg != "" {
				prev.homeImg = row.homeImg
				prev.awayImg = row.awayImg
				seen[row.id] = prev
				out[indexOf(out, row.id)] = prev
			}
		}
	}
	published := map[string]struct{}{}
	ids := jogoRE.FindAllStringSubmatch(chunk, -1)
	if len(ids) == 0 {
		ids = codRE.FindAllStringSubmatch(chunk, -1)
	}
	for _, match := range ids {
		published[strings.ToLower(match[1])] = struct{}{}
	}
	if len(published) != len(out) {
		var missing []string
		for id := range published {
			if _, ok := seen[id]; !ok {
				missing = append(missing, id)
			}
		}
		sort.Strings(missing)
		shown := missing
		if len(shown) > 8 {
			shown = shown[:8]
		}
		return nil, fmt.Errorf("season page published %d matches and the parser kept %d (%s)", len(published), len(out), strings.Join(shown, ", "))
	}
	return out, nil
}

func indexOf(rows []rawMatch, id string) int {
	for i := range rows {
		if rows[i].id == id {
			return i
		}
	}
	return 0
}

func sameMatch(a, b rawMatch) bool {
	if a.homeImg != "" && b.homeImg != "" && (a.homeImg != b.homeImg || a.awayImg != b.awayImg) {
		return false
	}
	return a.number == b.number && a.round == b.round && a.phase == b.phase &&
		a.home == b.home && a.away == b.away && a.homeSet == b.homeSet && a.awaySet == b.awaySet &&
		a.homeN == b.homeN && a.awayN == b.awayN && a.venue == b.venue && a.city == b.city && a.date == b.date && a.clock == b.clock
}

type marker struct {
	pos   int
	kind  string
	round int
	date  string
	match rawMatch
}

func parsePhase(chunk, phase string) ([]rawMatch, error) {
	var marks []marker
	for _, match := range roundRE.FindAllStringSubmatchIndex(chunk, -1) {
		number, err := strconv.Atoi(chunk[match[2]:match[3]])
		if err != nil || number <= 0 {
			return nil, fmt.Errorf("phase %s has a round that is not a positive number", phase)
		}
		marks = append(marks, marker{pos: match[0], kind: "round", round: number})
	}
	for _, match := range boldRE.FindAllStringSubmatchIndex(chunk, -1) {
		text := html.UnescapeString(chunk[match[2]:match[3]])
		if dateRE.FindString(text) == "" {
			continue
		}
		marks = append(marks, marker{pos: match[0], kind: "date", date: clean(text)})
	}
	for _, match := range matchRE.FindAllStringSubmatchIndex(chunk, -1) {
		row, err := rawFromMatch(chunk, match, phase)
		if err != nil {
			return nil, err
		}
		marks = append(marks, marker{pos: match[0], kind: "match", match: row})
	}
	sort.SliceStable(marks, func(i, j int) bool { return marks[i].pos < marks[j].pos })
	var (
		round int
		date  string
		out   []rawMatch
	)
	for _, mark := range marks {
		switch mark.kind {
		case "round":
			round = mark.round
		case "date":
			date = mark.date
		default:
			if round == 0 {
				return nil, fmt.Errorf("match %s has no round", mark.match.id)
			}
			row := mark.match
			row.round = round
			row.date = date
			out = append(out, row)
		}
	}
	return out, nil
}

func rawFromMatch(chunk string, loc []int, phase string) (rawMatch, error) {
	// Groups: 1 clock, 2 home, 3 home score, 4 separator, 5 away score,
	// 6 away, 7 number, 8 venue, 9 city, 10 data-jogo, 11 map code.
	part := func(n int) string {
		start, end := loc[n*2], loc[n*2+1]
		if start < 0 || end < start {
			return ""
		}
		return chunk[start:end]
	}
	id := part(10)
	if id == "" {
		id = part(11)
	}
	id = strings.ToLower(id)
	if id == "" {
		return rawMatch{}, fmt.Errorf("match is missing an id")
	}
	number := 0
	if part(7) != "" {
		parsed, err := strconv.Atoi(part(7))
		if err != nil || parsed <= 0 {
			return rawMatch{}, fmt.Errorf("match %s has no game number", id)
		}
		number = parsed
	}
	if !strings.EqualFold(clean(part(4)), "X") {
		return rawMatch{}, fmt.Errorf("match %s has no score", id)
	}
	homeN, homeSet, err := scoreCell(part(3))
	if err != nil {
		return rawMatch{}, fmt.Errorf("match %s: %w", id, err)
	}
	awayN, awaySet, err := scoreCell(part(5))
	if err != nil {
		return rawMatch{}, fmt.Errorf("match %s: %w", id, err)
	}
	if homeSet != awaySet {
		return rawMatch{}, fmt.Errorf("match %s has only one score", id)
	}
	imgs := imgRE.FindAllStringSubmatch(chunk[loc[0]:loc[1]], -1)
	row := rawMatch{
		id:      id,
		number:  number,
		phase:   phase,
		home:    clean(html.UnescapeString(part(2))),
		away:    clean(html.UnescapeString(part(6))),
		clock:   clean(part(1)),
		homeSet: homeSet,
		awaySet: awaySet,
		homeN:   homeN,
		awayN:   awayN,
		venue:   clean(html.UnescapeString(part(8))),
		city:    clean(html.UnescapeString(part(9))),
	}
	if len(imgs) >= 1 {
		row.homeImg = imgs[0][1]
	}
	if len(imgs) >= 2 {
		row.awayImg = imgs[1][1]
	}
	if row.home == "" || row.away == "" {
		return rawMatch{}, fmt.Errorf("match %s is missing a club", row.id)
	}
	return row, nil
}

// publishedInt reads a table figure. A trailing * is the FMF footnote for a
// points deduction. The number in the cell is the figure that is stored.
func publishedInt(raw string) (int, error) {
	text := strings.TrimSpace(raw)
	text = strings.TrimRight(text, "*")
	text = strings.TrimSpace(text)
	number, err := strconv.Atoi(text)
	if err != nil {
		return 0, err
	}
	return number, nil
}

func scoreCell(raw string) (int, bool, error) {
	text := strings.TrimSpace(strings.ReplaceAll(html.UnescapeString(raw), "\u00a0", ""))
	if text == "" || text == "-" {
		return 0, false, nil
	}
	number, err := strconv.Atoi(text)
	if err != nil || number < 0 {
		return 0, false, fmt.Errorf("score %q", text)
	}
	return number, true, nil
}

type rawRow struct {
	group string
	name  string
	row   domain.StandingInput
}

func parseStandings(body []byte) ([]rawRow, error) {
	doc, err := nethtml.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse standings html: %w", err)
	}
	tab := findID(doc, "tab_2-2")
	if tab == nil {
		return nil, fmt.Errorf("season page has no classification")
	}
	var tables [][]rawRow
	var walk func(*nethtml.Node)
	walk = func(n *nethtml.Node) {
		if err != nil || n == nil {
			return
		}
		if n.Type == nethtml.ElementNode && n.Data == "table" {
			rows, rowErr := tableRows(n)
			if rowErr != nil {
				err = rowErr
				return
			}
			if len(rows) > 0 {
				tables = append(tables, rows)
			}
		}
		for c := n.FirstChild; c != nil && err == nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(tab)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	var out []rawRow
	for _, table := range tables {
		var fresh, repeated int
		for _, row := range table {
			if _, ok := seen[fold(row.name)]; ok {
				repeated++
			} else {
				fresh++
			}
		}
		if fresh == 0 {
			continue
		}
		if repeated > 0 {
			return nil, fmt.Errorf("standings table mixes a later phase with the classification")
		}
		for _, row := range table {
			seen[fold(row.name)] = struct{}{}
			out = append(out, row)
		}
	}
	return out, nil
}

func tableRows(table *nethtml.Node) ([]rawRow, error) {
	var trs []*nethtml.Node
	var walk func(*nethtml.Node)
	walk = func(n *nethtml.Node) {
		if n.Type == nethtml.ElementNode && n.Data == "tr" {
			trs = append(trs, n)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(table)
	header := map[string]int{}
	var rows []rawRow
	group := ""
	groupOrder := map[string]int{}
	nextGroup := 0
	for _, tr := range trs {
		cells := rowCells(tr)
		if len(cells) == 0 {
			continue
		}
		if _, ok := indexOfHeader(cells, "EQUIPE"); ok && indexHas(cells, "PG") {
			header = map[string]int{}
			for i, cell := range cells {
				header[fold(cell)] = i
			}
			continue
		}
		if len(nonEmpty(cells)) == 1 && !isHeaderWord(nonEmpty(cells)[0]) {
			group = nonEmpty(cells)[0]
			if _, ok := groupOrder[group]; !ok {
				groupOrder[group] = nextGroup
				nextGroup++
			}
			continue
		}
		if len(header) == 0 {
			continue
		}
		row, name, ok, err := standingFromCells(cells, header, group, groupOrder[group])
		if err != nil {
			return nil, err
		}
		if ok {
			rows = append(rows, rawRow{group: group, name: name, row: row})
		}
	}
	return rows, nil
}

func standingFromCells(cells []string, header map[string]int, group string, groupOrder int) (domain.StandingInput, string, bool, error) {
	name, ok := cellAt(cells, header, "EQUIPE")
	if !ok || name == "" || fold(name) == "EQUIPE" {
		return domain.StandingInput{}, "", false, nil
	}
	positionText, ok := cellAt(cells, header, "#")
	if !ok {
		positionText = cells[0]
	}
	position, err := strconv.Atoi(positionText)
	if err != nil || position <= 0 {
		return domain.StandingInput{}, "", false, nil
	}
	values := map[string]int{}
	for _, key := range []string{"PG", "J", "V", "E", "D", "GP", "GC", "SG"} {
		text, found := cellAt(cells, header, key)
		if !found {
			return domain.StandingInput{}, "", false, fmt.Errorf("standings row %s is missing %s", name, key)
		}
		number, convErr := publishedInt(text)
		if convErr != nil {
			return domain.StandingInput{}, "", false, fmt.Errorf("standings row %s has %s %q", name, key, text)
		}
		values[key] = number
	}
	win := 0
	if text, found := cellAt(cells, header, "%"); found && text != "" {
		win, err = publishedInt(text)
		if err != nil {
			return domain.StandingInput{}, "", false, fmt.Errorf("standings row %s has win percentage %q", name, text)
		}
	}
	return domain.StandingInput{
		GroupName:      group,
		GroupOrder:     groupOrder,
		Position:       position,
		Played:         values["J"],
		Wins:           values["V"],
		Draws:          values["E"],
		Losses:         values["D"],
		GoalsFor:       values["GP"],
		GoalsAgainst:   values["GC"],
		GoalDifference: values["SG"],
		Points:         values["PG"],
		WinPercentage:  win,
	}, name, true, nil
}

func assemble(season Season, matches []rawMatch, rows []rawRow) (Page, error) {
	type club struct {
		id   string
		name string
	}
	clubs := map[string]*club{}
	ensure := func(name, image string) (*club, error) {
		key := fold(name)
		item := clubs[key]
		if item == nil {
			item = &club{name: name}
			clubs[key] = item
		}
		if image != "" {
			id := badgeID(image)
			if id == "" {
				return nil, fmt.Errorf("club %s has a badge without an id", name)
			}
			if item.id != "" && item.id != id {
				return nil, fmt.Errorf("club %s is published with two badge ids", name)
			}
			item.id = id
		}
		return item, nil
	}
	for _, match := range matches {
		if _, err := ensure(match.home, match.homeImg); err != nil {
			return Page{}, err
		}
		if _, err := ensure(match.away, match.awayImg); err != nil {
			return Page{}, err
		}
	}
	for _, row := range rows {
		item := clubs[fold(row.name)]
		if item == nil {
			item = &club{name: row.name}
			clubs[fold(row.name)] = item
		}
		item.name = row.name
	}
	for _, item := range clubs {
		if item.id == "" {
			item.id = "name:" + slug(item.name)
		}
	}
	loc, err := saoPaulo()
	if err != nil {
		return Page{}, err
	}
	page := Page{Season: season}
	for _, match := range matches {
		home := clubs[fold(match.home)]
		away := clubs[fold(match.away)]
		if home.id == away.id {
			return Page{}, fmt.Errorf("match %s lists club %s on both sides", match.id, home.name)
		}
		kick, err := kickoffAt(loc, match.date, match.clock)
		if err != nil {
			return Page{}, fmt.Errorf("match %s: %w", match.id, err)
		}
		item := domain.MatchInput{
			ExternalID:     match.id,
			MatchNumber:    match.number,
			RoundNumber:    match.round,
			Phase:          match.phase,
			HomeExternalID: home.id,
			AwayExternalID: away.id,
			Kickoff:        kick,
			Venue:          match.venue,
			City:           match.city,
			Status:         domain.StatusScheduled,
		}
		if match.homeSet {
			homeScore := match.homeN
			awayScore := match.awayN
			item.HomeScore = &homeScore
			item.AwayScore = &awayScore
			item.Status = domain.StatusFinished
		}
		page.Matches = append(page.Matches, item)
	}
	groupOrder := map[string]int{}
	nextGroup := 0
	for _, row := range rows {
		item := clubs[fold(row.name)]
		order, ok := groupOrder[row.group]
		if !ok {
			order = nextGroup
			groupOrder[row.group] = order
			nextGroup++
		}
		row.row.ExternalTeamID = item.id
		row.row.GroupOrder = order
		page.Standings = append(page.Standings, row.row)
	}
	names := make([]string, 0, len(clubs))
	for key := range clubs {
		names = append(names, key)
	}
	sort.Strings(names)
	for _, key := range names {
		item := clubs[key]
		page.Teams = append(page.Teams, domain.TeamInput{
			ExternalID: item.id,
			Name:       item.name,
			ShortName:  item.name,
		})
	}
	return page, nil
}

func kickoffAt(loc *time.Location, dateText, clock string) (*time.Time, error) {
	if dateText == "" || clock == "" {
		return nil, nil
	}
	found := dateRE.FindStringSubmatch(dateText)
	if found == nil {
		return nil, fmt.Errorf("date %q", dateText)
	}
	day, err := strconv.Atoi(found[1])
	if err != nil {
		return nil, fmt.Errorf("date %q", dateText)
	}
	month, ok := months[fold(found[2])]
	if !ok {
		return nil, fmt.Errorf("month %q", found[2])
	}
	year, err := strconv.Atoi(found[3])
	if err != nil {
		return nil, fmt.Errorf("date %q", dateText)
	}
	parts := clockRE.FindStringSubmatch(clock)
	if parts == nil {
		return nil, fmt.Errorf("clock %q", clock)
	}
	hour, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, err
	}
	minute, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, err
	}
	if hour > 23 || minute > 59 || day < 1 || day > 31 {
		return nil, fmt.Errorf("clock %s on %s", clock, dateText)
	}
	when := time.Date(year, month, day, hour, minute, 0, 0, loc).UTC()
	return &when, nil
}

func badgeID(src string) string {
	if match := logoRE.FindStringSubmatch(src); match != nil {
		return match[1]
	}
	base := src
	if cut := strings.LastIndex(base, "/"); cut >= 0 {
		base = base[cut+1:]
	}
	base = strings.TrimSuffix(base, ".png")
	base = strings.TrimSuffix(base, ".jpg")
	base = strings.TrimSuffix(base, ".gif")
	base = strings.TrimSuffix(base, ".jpeg")
	base = strings.TrimSuffix(base, ".svg")
	if base == "" || strings.Contains(base, "/") {
		return ""
	}
	return "badge:" + base
}

func labelMatches(division, label string) bool {
	folded := fold(label)
	switch division {
	case "1":
		return strings.Contains(folded, "MODULO I") && !strings.Contains(folded, "MODULO II")
	case "2":
		return strings.Contains(folded, "MODULO II")
	case "3":
		return strings.Contains(folded, "SEGUNDA DIVISAO")
	default:
		return false
	}
}

func clean(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func fold(value string) string {
	value = strings.ToUpper(clean(value))
	var b strings.Builder
	for _, r := range value {
		switch r {
		case 'Á', 'À', 'Ã', 'Â':
			r = 'A'
		case 'É', 'Ê':
			r = 'E'
		case 'Í':
			r = 'I'
		case 'Ó', 'Ô', 'Õ':
			r = 'O'
		case 'Ú':
			r = 'U'
		case 'Ç':
			r = 'C'
		}
		b.WriteRune(r)
	}
	return b.String()
}

func slug(value string) string {
	folded := strings.ToLower(fold(value))
	var b strings.Builder
	dash := false
	for _, r := range folded {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
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

func findID(n *nethtml.Node, id string) *nethtml.Node {
	if n.Type == nethtml.ElementNode {
		for _, attr := range n.Attr {
			if attr.Key == "id" && attr.Val == id {
				return n
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findID(c, id); found != nil {
			return found
		}
	}
	return nil
}

func rowCells(tr *nethtml.Node) []string {
	var cells []string
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == nethtml.ElementNode && c.Data == "td" {
			cells = append(cells, clean(nodeText(c)))
		}
	}
	return cells
}

func nodeText(n *nethtml.Node) string {
	var b strings.Builder
	var walk func(*nethtml.Node)
	walk = func(n *nethtml.Node) {
		if n.Type == nethtml.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func indexOfHeader(cells []string, want string) (int, bool) {
	for i, cell := range cells {
		if fold(cell) == want {
			return i, true
		}
	}
	return 0, false
}

func indexHas(cells []string, want string) bool {
	_, ok := indexOfHeader(cells, want)
	return ok
}

func cellAt(cells []string, header map[string]int, key string) (string, bool) {
	index, ok := header[key]
	if !ok || index < 0 || index >= len(cells) {
		return "", false
	}
	return cells[index], true
}

func nonEmpty(cells []string) []string {
	var out []string
	for _, cell := range cells {
		if cell != "" {
			out = append(out, cell)
		}
	}
	return out
}

func isHeaderWord(value string) bool {
	switch fold(value) {
	case "#", "EQUIPE", "PG", "J", "V", "E", "D", "GP", "GC", "SG", "%":
		return true
	default:
		return false
	}
}

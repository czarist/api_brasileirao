package portal

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
)

// Link is one competition card on a catalog year page.
type Link struct {
	ID    string
	Title string
	Year  int
	Code  string
}

// Phase is one published stage on the season page.
type Phase struct {
	ID   string
	Name string
}

var (
	optionRE   = regexp.MustCompile(`(?i)<option\b([^>]*)>([0-9]{4})</option>`)
	linkRE     = regexp.MustCompile(`(?is)href="[^"]*jogos\.php\?q=([0-9]+)"[^>]*>(.*?)</a>`)
	headRE     = regexp.MustCompile(`(?is)<h[1-6]\b[^>]*>(.*?)</h[1-6]>`)
	yearRE     = regexp.MustCompile(`(20[0-9]{2})\s*$`)
	phaseRE    = regexp.MustCompile(`(?is)<a\b([^>]*\bfase\b[^>]*)>(.*?)</a>`)
	attrRE     = regexp.MustCompile(`(?i)\b(data-id|data-fase)="([0-9]+)"`)
	pageYear   = regexp.MustCompile(`(?i)fs-title-h3">[^<]*(20[0-9]{2})`)
	roundRE    = regexp.MustCompile(`(?i)^([0-9]+)`)
	whenRE     = regexp.MustCompile(`(?i)(\d{1,2})/(\d{1,2})\s*-\s*(\d{1,2})[:h](\d{2})(?:\s*-\s*(.*))?$`)
	imgRE      = regexp.MustCompile(`(?i)<img\b[^>]*>`)
	attrVal    = regexp.MustCompile(`(?i)\b(alt|data-bs-title|src)="([^"]*)"`)
	shortRE    = regexp.MustCompile(`(?i)<div class="p[es]-2">([^<]+)</div>`)
	scoreRE    = regexp.MustCompile(`(?i)<span class="text-end">([^<]*)</span>[\s\S]{0,240}?<span class="text-start">([^<]*)</span>`)
	jogoRE     = regexp.MustCompile(`(?i)jogo=([0-9]+)`)
	errUnnamed = errors.New("match has no published club")
	badgeRE    = regexp.MustCompile(`(?i)(?:escudo_clubes/([0-9]+)\.|/clubes/([0-9]+)/escudo\.)`)
	colspanRE  = regexp.MustCompile(`(?is)<t[dh]\b[^>]*\bcolspan="?\d+"?[^>]*>(.*?)</t[dh]>`)
	feedRE     = regexp.MustCompile(`(?i)(?:^|[^A-Za-z0-9_])((?:jogos|classificacao)_ajax_ver[23]\.php)`)
	rowRE      = regexp.MustCompile(`(?is)<tr\b[^>]*>(.*?)</tr>`)
	cellRE     = regexp.MustCompile(`(?is)<td\b[^>]*>(.*?)</td>`)
	spanRE     = regexp.MustCompile(`(?is)<span\b[^>]*>([^<]*)</span>`)
	tagRE      = regexp.MustCompile(`(?is)<[^>]+>`)
)

// ParseCatalog reads the year select and the competition cards.
func ParseCatalog(site Site, page string) (selected int, links []Link, years []int, err error) {
	for _, found := range optionRE.FindAllStringSubmatch(page, -1) {
		year, convErr := strconv.Atoi(found[2])
		if convErr != nil {
			return 0, nil, nil, fmt.Errorf("catalog year %q: %w", found[2], convErr)
		}
		years = append(years, year)
		if strings.Contains(strings.ToLower(found[1]), "selected") {
			selected = year
		}
	}
	if selected == 0 || len(years) == 0 {
		return 0, nil, nil, fmt.Errorf("catalog page has no selected year")
	}
	seen := map[string]struct{}{}
	for _, found := range linkRE.FindAllStringSubmatchIndex(page, -1) {
		id := page[found[2]:found[3]]
		inner := page[found[4]:found[5]]
		title := clean(inner)
		if title == "" {
			title = headingBefore(page[:found[0]])
		}
		if title == "" {
			continue
		}
		division, ok := site.Classify(title)
		if !ok {
			continue
		}
		year, convErr := titleYear(title)
		if convErr != nil {
			return 0, nil, nil, convErr
		}
		if year != selected {
			return 0, nil, nil, fmt.Errorf("catalog card %s %q belongs to %d on the %d page", id, title, year, selected)
		}
		key := division.Code + "\x00" + id
		if _, ok := seen[key]; ok {
			continue
		}
		if _, ok := seen[division.Code]; ok {
			return 0, nil, nil, fmt.Errorf("catalog year %d lists %s twice", selected, division.Code)
		}
		seen[key] = struct{}{}
		seen[division.Code] = struct{}{}
		links = append(links, Link{ID: id, Title: title, Year: year, Code: division.Code})
	}
	return selected, links, years, nil
}

func headingBefore(window string) string {
	if len(window) > 900 {
		window = window[len(window)-900:]
	}
	found := headRE.FindAllStringSubmatch(window, -1)
	if len(found) == 0 {
		return ""
	}
	return clean(found[len(found)-1][1])
}

func titleYear(title string) (int, error) {
	found := yearRE.FindStringSubmatch(strings.TrimSpace(title))
	if found == nil {
		return 0, fmt.Errorf("catalog card %q has no year", title)
	}
	year, err := strconv.Atoi(found[1])
	if err != nil {
		return 0, err
	}
	return year, nil
}

// ParsePhases reads the stage tabs. competitionID must be the requested card.
func ParsePhases(page, competitionID string) ([]Phase, error) {
	var phases []Phase
	seen := map[string]struct{}{}
	for _, found := range phaseRE.FindAllStringSubmatch(page, -1) {
		attrs := map[string]string{}
		for _, attr := range attrRE.FindAllStringSubmatch(found[1], -1) {
			attrs[strings.ToLower(attr[1])] = attr[2]
		}
		if attrs["data-id"] == "" || attrs["data-fase"] == "" {
			continue
		}
		if attrs["data-id"] != competitionID {
			return nil, fmt.Errorf("phase belongs to competition %s, not %s", attrs["data-id"], competitionID)
		}
		name := clean(found[2])
		if name == "" {
			return nil, fmt.Errorf("phase %s has no name", attrs["data-fase"])
		}
		if _, ok := seen[attrs["data-fase"]]; ok {
			continue
		}
		seen[attrs["data-fase"]] = struct{}{}
		phases = append(phases, Phase{ID: attrs["data-fase"], Name: name})
	}
	if len(phases) == 0 {
		return nil, fmt.Errorf("season page has no phases")
	}
	return phases, nil
}

// Fixture is one match plus the names printed beside the badges.
type Fixture struct {
	Match     domain.MatchInput
	HomeName  string
	HomeShort string
	AwayName  string
	AwayShort string
}

// PageYear returns the year in the season heading when the page publishes one.
func PageYear(page string) (int, bool) {
	found := pageYear.FindStringSubmatch(page)
	if found == nil {
		return 0, false
	}
	year, err := strconv.Atoi(found[1])
	if err != nil {
		return 0, false
	}
	return year, true
}

// ParseMatchFeed unwraps the JSON document the public page requests.
func ParseMatchFeed(body []byte) (string, error) {
	var doc struct {
		HTML string `json:"html"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", fmt.Errorf("match feed: %w", err)
	}
	return doc.HTML, nil
}

// LinkClubs gives one identity to a short name that the page also printed in full.
// A numeric badge wins over a name derived from the text. A short printed on both
// sides of one match names two clubs, so that short is not used as a link key.
func LinkClubs(fixtures []Fixture) []Fixture {
	type choice struct {
		id, name, short string
	}
	ambiguous := map[string]bool{}
	for _, fixture := range fixtures {
		home := fold(fixture.HomeShort)
		away := fold(fixture.AwayShort)
		if home == "" || home != away {
			continue
		}
		if fixture.Match.HomeExternalID != fixture.Match.AwayExternalID {
			ambiguous[home] = true
		}
	}
	byShort := map[string]choice{}
	consider := func(id, name, short string) {
		key := fold(short)
		if short == "" || ambiguous[key] {
			return
		}
		next := choice{id: id, name: name, short: short}
		current, ok := byShort[key]
		if ok && !richerClub(current.id, current.name, current.short, next.id, next.name, next.short) {
			return
		}
		byShort[key] = next
	}
	for _, fixture := range fixtures {
		consider(fixture.Match.HomeExternalID, fixture.HomeName, fixture.HomeShort)
		consider(fixture.Match.AwayExternalID, fixture.AwayName, fixture.AwayShort)
	}
	apply := func(id, name, short *string) {
		if ambiguous[fold(*short)] {
			return
		}
		if *id != "" && !strings.HasPrefix(*id, "name:") {
			return
		}
		chosen, ok := byShort[fold(*short)]
		if !ok || chosen.id == "" {
			return
		}
		*id = chosen.id
		if chosen.name != "" && (*name == "" || *name == *short) {
			*name = chosen.name
		}
	}
	for i := range fixtures {
		apply(&fixtures[i].Match.HomeExternalID, &fixtures[i].HomeName, &fixtures[i].HomeShort)
		apply(&fixtures[i].Match.AwayExternalID, &fixtures[i].AwayName, &fixtures[i].AwayShort)
	}
	return fixtures
}

func richerClub(currentID, currentName, currentShort, nextID, nextName, nextShort string) bool {
	if currentID == "" && nextID != "" {
		return true
	}
	if strings.HasPrefix(currentID, "name:") && nextID != "" && !strings.HasPrefix(nextID, "name:") {
		return true
	}
	if currentName == currentShort && nextName != "" && nextName != nextShort {
		return true
	}
	return false
}

// ParseMatches reads one phase. The catalog year supplies the missing year.
func ParseMatches(feed, phase string, year int, loc *time.Location) ([]Fixture, error) {
	parts := strings.Split(feed, `title-rodada">`)
	if len(parts) < 2 {
		return nil, nil
	}
	var matches []Fixture
	seen := map[string]Fixture{}
	for _, part := range parts[1:] {
		end := strings.Index(part, "</div>")
		if end < 0 {
			return nil, fmt.Errorf("round title is not closed")
		}
		roundTitle := clean(part[:end])
		round := 1
		if found := roundRE.FindStringSubmatch(fold(roundTitle)); found != nil {
			number, err := strconv.Atoi(found[1])
			if err != nil {
				return nil, err
			}
			round = number
		}
		blocks := strings.Split(part[end:], "pr-jogos")
		for _, block := range blocks[1:] {
			match, err := parseMatch(block, phase, round, year, loc)
			if errors.Is(err, errUnnamed) {
				continue
			}
			if err != nil {
				return nil, err
			}
			if previous, ok := seen[match.Match.ExternalID]; ok {
				if sameScore(previous.Match, match.Match) {
					continue
				}
				return nil, fmt.Errorf("match %s is published twice with different scores", match.Match.ExternalID)
			}
			seen[match.Match.ExternalID] = match
			matches = append(matches, match)
		}
	}
	return matches, nil
}

func sameScore(left, right domain.MatchInput) bool {
	return sameInt(left.HomeScore, right.HomeScore) && sameInt(left.AwayScore, right.AwayScore)
}

func sameInt(left, right *int) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func parseMatch(block, phase string, round, year int, loc *time.Location) (Fixture, error) {
	id := jogoRE.FindStringSubmatch(block)
	if id == nil {
		return Fixture{}, fmt.Errorf("match in %s round %d has no id", phase, round)
	}
	imgs := imgRE.FindAllString(block, -1)
	if len(imgs) < 2 {
		return Fixture{}, fmt.Errorf("match %s is missing a badge", id[1])
	}
	homeID, homeName := club(imgs[0])
	awayID, awayName := club(imgs[1])
	shorts := shortRE.FindAllStringSubmatch(block, -1)
	homeShort, awayShort := homeName, awayName
	if len(shorts) >= 1 {
		homeShort = clean(shorts[0][1])
	}
	if len(shorts) >= 2 {
		awayShort = clean(shorts[1][1])
	}
	if homeName == "" {
		homeName = homeShort
	}
	if awayName == "" {
		awayName = awayShort
	}
	if homeID == "" {
		homeID = nameID(homeName)
	}
	if awayID == "" {
		awayID = nameID(awayName)
	}
	if homeID == "" || awayID == "" {
		return Fixture{}, errUnnamed
	}
	if homeID == awayID {
		return Fixture{}, fmt.Errorf("match %s lists the same club twice", id[1])
	}
	var homeScore, awayScore *int
	if found := scoreRE.FindStringSubmatch(block); found != nil {
		left, right, err := pairScores(found[1], found[2])
		if err != nil {
			return Fixture{}, fmt.Errorf("match %s: %w", id[1], err)
		}
		homeScore, awayScore = left, right
	}
	whenText := ""
	if start := strings.Index(strings.ToLower(block), "cronolog-sm"); start >= 0 {
		piece := block[start:]
		if open := strings.Index(piece, ">"); open >= 0 {
			piece = piece[open+1:]
			if close := strings.Index(piece, "<"); close >= 0 {
				whenText = clean(piece[:close])
			}
		}
	}
	kickoff, venue, err := kickoffAndVenue(whenText, year, loc)
	if err != nil {
		return Fixture{}, fmt.Errorf("match %s: %w", id[1], err)
	}
	status := statusOf(block, homeScore, awayScore, kickoff)
	return Fixture{
		HomeName:  homeName,
		HomeShort: homeShort,
		AwayName:  awayName,
		AwayShort: awayShort,
		Match: domain.MatchInput{
			ExternalID:     id[1],
			RoundNumber:    round,
			Phase:          phase,
			HomeExternalID: homeID,
			AwayExternalID: awayID,
			Kickoff:        kickoff,
			Status:         status,
			HomeScore:      homeScore,
			AwayScore:      awayScore,
			Venue:          venue,
		},
	}, nil
}

func club(tag string) (id, name string) {
	attrs := map[string]string{}
	for _, found := range attrVal.FindAllStringSubmatch(tag, -1) {
		attrs[strings.ToLower(found[1])] = html.UnescapeString(found[2])
	}
	id = badgeID(attrs["src"])
	name = clean(attrs["alt"])
	if name == "" {
		name = clean(attrs["data-bs-title"])
	}
	if id == "" {
		id = nameID(name)
	}
	return id, name
}

// nameID is the identity of a club the page published without a numeric badge.
func nameID(name string) string {
	folded := fold(name)
	var b strings.Builder
	b.WriteString("name:")
	dash := false
	for _, r := range folded {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteByte(byte(r) + ('a' - 'A'))
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

func pairScores(left, right string) (*int, *int, error) {
	left = clean(left)
	right = clean(right)
	if left == "" && right == "" || left == "-" && right == "-" {
		return nil, nil, nil
	}
	if left == "" || right == "" || left == "-" || right == "-" {
		return nil, nil, fmt.Errorf("score %q x %q is incomplete", left, right)
	}
	home, err := publishedInt(left)
	if err != nil {
		return nil, nil, err
	}
	away, err := publishedInt(right)
	if err != nil {
		return nil, nil, err
	}
	return &home, &away, nil
}

func kickoffAndVenue(text string, year int, loc *time.Location) (*time.Time, string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, "", nil
	}
	found := whenRE.FindStringSubmatch(text)
	if found == nil {
		if strings.Contains(fold(text), "DEFINIR") {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("clock %q is not a date", text)
	}
	day, _ := strconv.Atoi(found[1])
	month, _ := strconv.Atoi(found[2])
	hour, _ := strconv.Atoi(found[3])
	minute, _ := strconv.Atoi(found[4])
	venue := clean(found[5])
	kick := time.Date(year, time.Month(month), day, hour, minute, 0, 0, loc)
	if kick.Day() != day || kick.Month() != time.Month(month) {
		return nil, "", fmt.Errorf("date %02d/%02d/%d is not a real day", day, month, year)
	}
	utc := kick.UTC()
	return &utc, venue, nil
}

func statusOf(block string, home, away *int, kickoff *time.Time) domain.MatchStatus {
	folded := fold(block)
	switch {
	case strings.Contains(folded, "CANCEL"):
		return domain.StatusCancelled
	case strings.Contains(folded, "SUSPENS"):
		return domain.StatusSuspended
	case home != nil && away != nil:
		return domain.StatusFinished
	case (strings.Contains(folded, "ADIAD") || strings.Contains(folded, "REMARC")) && kickoff == nil:
		return domain.StatusPostponed
	default:
		return domain.StatusScheduled
	}
}

// Club is the published spelling of a badge.
type Club struct {
	Name  string
	Short string
}

// FeedScripts reads the match and classification script names printed on the
// season page. Only ver2 and ver3 of the two catalog feeds are accepted.
func FeedScripts(page string) (matches, standings string, err error) {
	found := map[string]string{}
	for _, item := range feedRE.FindAllStringSubmatch(page, -1) {
		name := strings.ToLower(item[1])
		kind := "match"
		if strings.HasPrefix(name, "classificacao_") {
			kind = "standings"
		}
		if previous, ok := found[kind]; ok && previous != name {
			return "", "", fmt.Errorf("season page names more than one %s feed", kind)
		}
		found[kind] = name
	}
	if found["match"] == "" || found["standings"] == "" {
		return "", "", errors.New("season page does not name its match and classification feeds")
	}
	return found["match"], found["standings"], nil
}

func badgeID(text string) string {
	found := badgeRE.FindStringSubmatch(text)
	if found == nil {
		return ""
	}
	if found[1] != "" {
		return found[1]
	}
	return found[2]
}

func rowGroup(row string) (string, bool) {
	found := colspanRE.FindStringSubmatch(row)
	if found == nil {
		return "", false
	}
	name := clean(found[1])
	if name == "" || isStatHeader(name) || fold(name) == "CLASSIFICACAO" {
		return "", false
	}
	return name, true
}

// ParseStandings reads every classification table in one phase feed.
func ParseStandings(page string) ([]domain.StandingInput, map[string]Club, error) {
	var rows []domain.StandingInput
	clubs := map[string]Club{}
	seen := map[string]struct{}{}
	for _, table := range strings.Split(page, "<table")[1:] {
		group := ""
		for _, row := range rowRE.FindAllStringSubmatch(table, -1) {
			if label, ok := rowGroup(row[1]); ok {
				group = label
			}
			cells := cellRE.FindAllStringSubmatch(row[1], -1)
			if len(cells) < 14 {
				continue
			}
			position, err := publishedInt(cells[0][1])
			if err != nil || position <= 0 {
				continue
			}
			name, short := namesFrom(cells[2][1])
			badge := badgeID(cells[1][1])
			if badge == "" {
				badge = nameID(name)
			}
			if badge == "" || name == "" {
				return nil, nil, fmt.Errorf("standings row %d is missing a club", position)
			}
			if _, ok := seen[badge]; ok {
				return nil, nil, fmt.Errorf("standings list %s twice", name)
			}
			seen[badge] = struct{}{}
			values := make([]int, 11)
			for i := 0; i < 11; i++ {
				values[i], err = publishedInt(cells[i+3][1])
				if err != nil {
					return nil, nil, fmt.Errorf("standings row %s column %d: %w", name, i+3, err)
				}
			}
			if short == "" {
				short = name
			}
			clubs[badge] = Club{Name: name, Short: short}
			rows = append(rows, domain.StandingInput{
				ExternalTeamID: badge,
				GroupName:      group,
				Position:       position,
				Played:         values[1],
				Wins:           values[2],
				Draws:          values[3],
				Losses:         values[4],
				GoalsFor:       values[5],
				GoalsAgainst:   values[6],
				GoalDifference: values[7],
				Points:         values[0],
				RedCards:       values[8],
				YellowCards:    values[9],
				WinPercentage:  values[10],
			})
		}
	}
	return rows, clubs, nil
}

func namesFrom(cell string) (string, string) {
	spans := spanRE.FindAllStringSubmatch(cell, -1)
	if len(spans) >= 2 {
		return clean(spans[0][1]), clean(spans[1][1])
	}
	if len(spans) == 1 {
		name := clean(spans[0][1])
		return name, name
	}
	name := clean(cell)
	return name, name
}

func isStatHeader(text string) bool {
	switch fold(text) {
	case "PTS", "J", "V", "E", "D", "GP", "GC", "SG", "CV", "CA", "%", "#":
		return true
	default:
		return false
	}
}

func publishedInt(text string) (int, error) {
	text = clean(text)
	text = strings.TrimRight(text, "*")
	text = strings.TrimSuffix(text, "º")
	text = strings.TrimSuffix(text, "°")
	text = strings.TrimSuffix(text, "%")
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, fmt.Errorf("empty number")
	}
	return strconv.Atoi(text)
}

func clean(text string) string {
	text = tagRE.ReplaceAllString(text, " ")
	text = html.UnescapeString(text)
	return strings.Join(strings.Fields(text), " ")
}

func fold(text string) string {
	text = strings.NewReplacer(
		"Á", "A", "À", "A", "Ã", "A", "Â", "A",
		"É", "E", "Ê", "E",
		"Í", "I",
		"Ó", "O", "Ô", "O", "Õ", "O",
		"Ú", "U",
		"Ç", "C",
		"º", " ", "ª", " ",
	).Replace(strings.ToUpper(text))
	return strings.Join(strings.Fields(text), " ")
}

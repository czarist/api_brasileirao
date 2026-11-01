package uikit

import (
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

// FeedQuery is the fixed form the season page posts to its own feeds.
type FeedQuery struct {
	CompetitionID string
	CategoryID    string
	Championship  string
	Year          string
}

// Fixture is one match plus the names printed beside the badges.
type Fixture struct {
	Match     domain.MatchInput
	HomeName  string
	HomeShort string
	AwayName  string
	AwayShort string
}

// Club is the published spelling of a badge.
type Club struct {
	Name  string
	Short string
}

var (
	optionRE   = regexp.MustCompile(`(?i)<option\b([^>]*)>([0-9]{4})</option>`)
	hrefTitle  = regexp.MustCompile(`(?is)href="([^"]+)"[^>]*title="([^"]+)"`)
	titleHref  = regexp.MustCompile(`(?is)title="([^"]+)"[^>]*href="([^"]+)"`)
	cardID     = regexp.MustCompile(`(?i)(?:tabela\.php\?ID=|tabela/)([0-9]+)`)
	yearRE     = regexp.MustCompile(`(20[0-9]{2})\s*$`)
	anchorRE   = regexp.MustCompile(`(?is)<a\b[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	phaseQuery = regexp.MustCompile(`(?i)(?:\?|&|&amp;)ID=([0-9]+)(?:&|&amp;)fase=([0-9]+)`)
	phasePath  = regexp.MustCompile(`(?i)/tabela/([0-9]+)/([0-9]+)`)
	paramRE    = regexp.MustCompile(`(?i)"(idcampeonato|categoria|campeonato|exercicio)"\s*:\s*([0-9]+)`)
	roundRE    = regexp.MustCompile(`(?is)<h5\b[^>]*uk-card-title[^>]*>(.*?)</h5>`)
	matchRE    = regexp.MustCompile(`(?is)<a\b[^>]*href="([^"]*jogo[^"]*)"[^>]*>`)
	jogoQuery  = regexp.MustCompile(`(?i)jogo=([0-9]+)`)
	jogoPath   = regexp.MustCompile(`(?i)/jogo/[0-9]+/([0-9]+)`)
	clockRE    = regexp.MustCompile(`(?is)<p\b[^>]*>(.*?)</p>`)
	whenRE     = regexp.MustCompile(`(?i)(\d{1,2})/(\d{1,2})(?:/(20\d{2}))?\s*-\s*(\d{1,2})[:h](\d{2})(?:\s*-\s*(.*))?$`)
	scoreRE    = regexp.MustCompile(`(?is)<h4\b[^>]*>(.*?)</h4>`)
	pairRE     = regexp.MustCompile(`(?i)^\s*([0-9]+|-)\s*x\s*([0-9]+|-)\s*$`)
	tooltipRE  = regexp.MustCompile(`(?i)uk-tooltip="([^"]*)"`)
	imgRE      = regexp.MustCompile(`(?i)<img\b[^>]*src="([^"]*)"`)
	shortRE    = regexp.MustCompile(`(?is)<span\b[^>]*uk-text-bold[^>]*>([^<]*)</span>`)
	badgeRE    = regexp.MustCompile(`(?i)(?:escudo_clubes/([0-9]+)\.|/clubes/([0-9]+)/escudo\.)`)
	rowRE      = regexp.MustCompile(`(?is)<tr\b[^>]*>(.*?)</tr>`)
	cellRE     = regexp.MustCompile(`(?is)<td\b[^>]*>(.*?)</td>`)
	headRE     = regexp.MustCompile(`(?is)<th\b([^>]*)>(.*?)</th>`)
	colspanRE  = regexp.MustCompile(`(?i)colspan="?([0-9]+)`)
	groupRE    = regexp.MustCompile(`(?is)<td\b[^>]*colspan="?\d+"?[^>]*>(.*?)</td>`)
	tagRE      = regexp.MustCompile(`(?is)<[^>]+>`)
	roundNo    = regexp.MustCompile(`([0-9]+)`)
	errUnnamed = errors.New("match has no published club")
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
	add := func(href, title string) error {
		id, ok := competitionID(href)
		if !ok {
			return nil
		}
		title = clean(html.UnescapeString(title))
		if title == "" {
			return nil
		}
		division, ok := site.Classify(title)
		if !ok {
			return nil
		}
		year, convErr := titleYear(title)
		if convErr != nil {
			return convErr
		}
		if year != selected {
			return fmt.Errorf("catalog card %s %q belongs to %d on the %d page", id, title, year, selected)
		}
		key := division.Code + "\x00" + id
		if _, ok := seen[key]; ok {
			return nil
		}
		if _, ok := seen[division.Code]; ok {
			return fmt.Errorf("catalog year %d lists %s twice", selected, division.Code)
		}
		seen[key] = struct{}{}
		seen[division.Code] = struct{}{}
		links = append(links, Link{ID: id, Title: title, Year: year, Code: division.Code})
		return nil
	}
	for _, found := range hrefTitle.FindAllStringSubmatch(page, -1) {
		if err := add(found[1], found[2]); err != nil {
			return 0, nil, nil, err
		}
	}
	for _, found := range titleHref.FindAllStringSubmatch(page, -1) {
		if err := add(found[2], found[1]); err != nil {
			return 0, nil, nil, err
		}
	}
	return selected, links, years, nil
}

func competitionID(href string) (string, bool) {
	found := cardID.FindStringSubmatch(href)
	if found == nil {
		return "", false
	}
	return found[1], true
}

func titleYear(title string) (int, error) {
	found := yearRE.FindStringSubmatch(strings.TrimSpace(title))
	if found == nil {
		return 0, fmt.Errorf("catalog card %q has no year", title)
	}
	return strconv.Atoi(found[1])
}

// ParsePhases reads the stage tabs that belong to competitionID.
func ParsePhases(page, competitionID string) ([]Phase, error) {
	var phases []Phase
	seen := map[string]struct{}{}
	for _, found := range anchorRE.FindAllStringSubmatch(page, -1) {
		id, fase, ok := phaseIDs(found[1])
		if !ok || id != competitionID {
			continue
		}
		name := clean(found[2])
		if name == "" {
			continue
		}
		if _, dup := seen[fase]; dup {
			continue
		}
		seen[fase] = struct{}{}
		phases = append(phases, Phase{ID: fase, Name: name})
	}
	if len(phases) == 0 {
		return nil, fmt.Errorf("season page has no phases")
	}
	return phases, nil
}

func phaseIDs(href string) (competition, fase string, ok bool) {
	if found := phaseQuery.FindStringSubmatch(href); found != nil {
		return found[1], found[2], true
	}
	if found := phasePath.FindStringSubmatch(href); found != nil {
		return found[1], found[2], true
	}
	return "", "", false
}

// ParseFeedQuery reads the ids the public page posts. They must agree with
// the requested competition and catalog year.
func ParseFeedQuery(page, competitionID string, year int) (FeedQuery, error) {
	got := map[string]string{}
	for _, found := range paramRE.FindAllStringSubmatch(page, -1) {
		key := strings.ToLower(found[1])
		if previous, ok := got[key]; ok && previous != found[2] {
			return FeedQuery{}, fmt.Errorf("season page publishes two %s values", key)
		}
		got[key] = found[2]
	}
	query := FeedQuery{
		CompetitionID: got["idcampeonato"],
		CategoryID:    got["categoria"],
		Championship:  got["campeonato"],
		Year:          got["exercicio"],
	}
	if query.CompetitionID == "" || query.CategoryID == "" || query.Championship == "" || query.Year == "" {
		return FeedQuery{}, fmt.Errorf("season page is missing its feed ids")
	}
	if query.CompetitionID != competitionID {
		return FeedQuery{}, fmt.Errorf("season page feed belongs to %s, not %s", query.CompetitionID, competitionID)
	}
	if query.Year != strconv.Itoa(year) {
		return FeedQuery{}, fmt.Errorf("season page feed says %s, not %d", query.Year, year)
	}
	return query, nil
}

// ParseMatches reads one phase feed. year is the catalog year.
func ParseMatches(feed, phase string, year int, loc *time.Location) ([]Fixture, error) {
	parts := roundRE.Split(feed, -1)
	titles := roundRE.FindAllStringSubmatch(feed, -1)
	if len(titles) == 0 {
		return nil, nil
	}
	if len(parts) != len(titles)+1 {
		return nil, fmt.Errorf("round titles do not split the feed")
	}
	var matches []Fixture
	for i, title := range titles {
		round := 1
		if found := roundNo.FindStringSubmatch(fold(title[1])); found != nil {
			number, err := strconv.Atoi(found[1])
			if err != nil {
				return nil, err
			}
			round = number
		}
		section := parts[i+1]
		anchors := matchRE.FindAllStringSubmatchIndex(section, -1)
		for n, anchor := range anchors {
			start := 0
			if n > 0 {
				start = anchors[n-1][1]
			}
			block := section[start:anchor[1]]
			href := section[anchor[2]:anchor[3]]
			match, err := parseMatch(block, href, phase, round, year, loc)
			if errors.Is(err, errUnnamed) {
				continue
			}
			if err != nil {
				return nil, err
			}
			matches = append(matches, match)
		}
	}
	return matches, nil
}

func parseMatch(block, href, phase string, round, year int, loc *time.Location) (Fixture, error) {
	id := matchID(href)
	if id == "" {
		return Fixture{}, fmt.Errorf("match in %s round %d has no id", phase, round)
	}
	tips := tooltipRE.FindAllStringSubmatch(block, -1)
	imgs := imgRE.FindAllStringSubmatch(block, -1)
	shorts := shortRE.FindAllStringSubmatch(block, -1)
	if len(tips) < 2 && len(shorts) < 2 {
		return Fixture{}, errUnnamed
	}
	homeName, awayName := sideText(tips, 0), sideText(tips, 1)
	homeShort, awayShort := sideText(shorts, 0), sideText(shorts, 1)
	if homeName == "" {
		homeName = homeShort
	}
	if awayName == "" {
		awayName = awayShort
	}
	homeID := badgeID(sideText(imgs, 0))
	awayID := badgeID(sideText(imgs, 1))
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
		return Fixture{}, fmt.Errorf("match %s lists the same club twice", id)
	}
	homeScore, awayScore, err := scores(block)
	if err != nil {
		return Fixture{}, fmt.Errorf("match %s: %w", id, err)
	}
	whenText := ""
	for _, found := range clockRE.FindAllStringSubmatch(block, -1) {
		text := clean(found[1])
		if text != "" {
			whenText = text
			break
		}
	}
	kickoff, venue, err := kickoffAndVenue(whenText, year, loc)
	if err != nil {
		return Fixture{}, fmt.Errorf("match %s: %w", id, err)
	}
	if homeShort == "" {
		homeShort = homeName
	}
	if awayShort == "" {
		awayShort = awayName
	}
	return Fixture{
		HomeName: homeName, HomeShort: homeShort, AwayName: awayName, AwayShort: awayShort,
		Match: domain.MatchInput{
			ExternalID: id, RoundNumber: round, Phase: phase,
			HomeExternalID: homeID, AwayExternalID: awayID,
			Kickoff: kickoff, Status: statusOf(block, homeScore, awayScore, kickoff),
			HomeScore: homeScore, AwayScore: awayScore, Venue: venue,
		},
	}, nil
}

func sideText(found [][]string, index int) string {
	if index >= len(found) {
		return ""
	}
	return clean(html.UnescapeString(found[index][1]))
}

func matchID(href string) string {
	if found := jogoQuery.FindStringSubmatch(href); found != nil {
		return found[1]
	}
	if found := jogoPath.FindStringSubmatch(href); found != nil {
		return found[1]
	}
	return ""
}

func scores(block string) (*int, *int, error) {
	found := scoreRE.FindStringSubmatch(block)
	if found == nil {
		return nil, nil, nil
	}
	text := clean(found[1])
	if text == "" || text == "-" || text == "X" {
		return nil, nil, nil
	}
	pair := pairRE.FindStringSubmatch(text)
	if pair == nil {
		return nil, nil, fmt.Errorf("score %q is not a pair", text)
	}
	if pair[1] == "-" && pair[2] == "-" {
		return nil, nil, nil
	}
	if pair[1] == "-" || pair[2] == "-" {
		return nil, nil, fmt.Errorf("score %q is incomplete", text)
	}
	home, err := strconv.Atoi(pair[1])
	if err != nil {
		return nil, nil, err
	}
	away, err := strconv.Atoi(pair[2])
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
	if found[3] != "" {
		published, _ := strconv.Atoi(found[3])
		if published != year {
			return nil, "", fmt.Errorf("clock year %d is not %d", published, year)
		}
	}
	hour, _ := strconv.Atoi(found[4])
	minute, _ := strconv.Atoi(found[5])
	venue := clean(found[6])
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

func badgeID(src string) string {
	found := badgeRE.FindStringSubmatch(src)
	if found == nil {
		return ""
	}
	if found[1] != "" {
		return found[1]
	}
	return found[2]
}

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

// ParseStandings reads one classification table. An unpublished phase is an
// empty table. Card columns follow the header, because this catalog prints
// yellow cards before red cards.
func ParseStandings(page string) ([]domain.StandingInput, map[string]Club, error) {
	if strings.Contains(fold(page), "INDISPONIVEL") && !strings.Contains(strings.ToLower(page), "<table") {
		return nil, map[string]Club{}, nil
	}
	var rows []domain.StandingInput
	clubs := map[string]Club{}
	seen := map[string]struct{}{}
	for _, table := range strings.Split(page, "<table")[1:] {
		columns, ok := headerColumns(table)
		if !ok {
			continue
		}
		group := ""
		for _, row := range rowRE.FindAllStringSubmatch(table, -1) {
			if label, grouped := rowGroup(row[1]); grouped {
				group = label
				continue
			}
			cells := cellRE.FindAllStringSubmatch(row[1], -1)
			if len(cells) < 14 {
				continue
			}
			position, err := publishedInt(cells[0][1])
			if err != nil || position <= 0 {
				continue
			}
			name := clean(cells[2][1])
			badge := badgeID(cells[1][1])
			if badge == "" {
				badge = nameID(name)
			}
			if badge == "" || name == "" {
				return nil, nil, fmt.Errorf("standings row %d is missing a club", position)
			}
			if _, dup := seen[badge]; dup {
				return nil, nil, fmt.Errorf("standings list %s twice", name)
			}
			seen[badge] = struct{}{}
			value := func(label string) (int, error) {
				index, ok := columns[label]
				if !ok || index >= len(cells) {
					return 0, fmt.Errorf("column %s is missing", label)
				}
				return publishedInt(cells[index][1])
			}
			points, err := value("PTS")
			if err != nil {
				return nil, nil, fmt.Errorf("standings row %s: %w", name, err)
			}
			played, err := value("J")
			if err != nil {
				return nil, nil, fmt.Errorf("standings row %s: %w", name, err)
			}
			wins, err := value("V")
			if err != nil {
				return nil, nil, fmt.Errorf("standings row %s: %w", name, err)
			}
			draws, err := value("E")
			if err != nil {
				return nil, nil, fmt.Errorf("standings row %s: %w", name, err)
			}
			losses, err := value("D")
			if err != nil {
				return nil, nil, fmt.Errorf("standings row %s: %w", name, err)
			}
			goalsFor, err := value("GP")
			if err != nil {
				return nil, nil, fmt.Errorf("standings row %s: %w", name, err)
			}
			goalsAgainst, err := value("GC")
			if err != nil {
				return nil, nil, fmt.Errorf("standings row %s: %w", name, err)
			}
			goalDifference, err := value("SG")
			if err != nil {
				return nil, nil, fmt.Errorf("standings row %s: %w", name, err)
			}
			yellow, err := value("CA")
			if err != nil {
				return nil, nil, fmt.Errorf("standings row %s: %w", name, err)
			}
			red, err := value("CV")
			if err != nil {
				return nil, nil, fmt.Errorf("standings row %s: %w", name, err)
			}
			percent, err := value("%")
			if err != nil {
				return nil, nil, fmt.Errorf("standings row %s: %w", name, err)
			}
			clubs[badge] = Club{Name: name, Short: name}
			rows = append(rows, domain.StandingInput{
				ExternalTeamID: badge, GroupName: group, Position: position,
				Played: played, Wins: wins, Draws: draws, Losses: losses,
				GoalsFor: goalsFor, GoalsAgainst: goalsAgainst, GoalDifference: goalDifference,
				Points: points, YellowCards: yellow, RedCards: red, WinPercentage: percent,
			})
		}
	}
	return rows, clubs, nil
}

func headerColumns(table string) (map[string]int, bool) {
	head := table
	if end := strings.Index(strings.ToLower(table), "</thead>"); end >= 0 {
		head = table[:end]
	} else if body := strings.Index(strings.ToLower(table), "<tbody"); body >= 0 {
		head = table[:body]
	}
	columns := map[string]int{}
	cursor := 0
	for _, found := range headRE.FindAllStringSubmatch(head, -1) {
		span := 1
		if match := colspanRE.FindStringSubmatch(found[1]); match != nil {
			span, _ = strconv.Atoi(match[1])
			if span <= 0 {
				span = 1
			}
		}
		label := fold(clean(found[2]))
		if label != "" && label != "CLASSIFICACAO" {
			columns[label] = cursor
		}
		cursor += span
	}
	for _, label := range []string{"PTS", "J", "V", "E", "D", "GP", "GC", "SG", "CA", "CV", "%"} {
		if _, ok := columns[label]; !ok {
			return nil, false
		}
	}
	return columns, true
}

func rowGroup(row string) (string, bool) {
	found := groupRE.FindStringSubmatch(row)
	if found == nil {
		return "", false
	}
	label := clean(found[1])
	folded := fold(label)
	if label == "" || folded == "CLASSIFICACAO" || isStatHeader(folded) {
		return "", false
	}
	return label, true
}

func isStatHeader(text string) bool {
	switch text {
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

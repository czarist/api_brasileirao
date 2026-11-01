package fafal

import (
	"errors"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
	"github.com/czarist/api_brasileirao/internal/scraper/uikit"
)

// Link is one adult competition card.
type Link struct {
	ID    string
	Title string
	Year  int
	Code  string
}

// Phase is one published stage.
type Phase struct {
	ID   string
	Name string
}

// Fixture is one match plus the names printed beside the badges.
type Fixture struct {
	Match     domain.MatchInput
	HomeName  string
	HomeShort string
	AwayName  string
	AwayShort string
}

var (
	yearLinkRE   = regexp.MustCompile(`/novo/competicoes/(20[0-9]{2})`)
	anoRE        = regexp.MustCompile(`var ano = "([0-9]{4})"`)
	anchorCardRE = regexp.MustCompile(`(?is)<a\b[^>]*href="/novo/tabela\?ID=([0-9]+)"[^>]*>(.*?)</a>`)
	imageFileRE  = regexp.MustCompile(`/novo/imagens/competicoes/([a-z0-9._-]+\.png)`)
	imageA       = regexp.MustCompile(`^alagoano-(20[0-9]{2})\.png$`)
	imageB       = regexp.MustCompile(`^alagoano-serie-b-(20[0-9]{2})\.png$|^alagoano-(20[0-9]{2})-serie-b\.png$`)
	textDivision = regexp.MustCompile(`^ALAGOANO SERIE ([AB])(?: / ([0-9]{4})| ([0-9]{4}))?$`)
	penaltyRE    = regexp.MustCompile(`(?i)^\(([0-9]+)\s*x\s*([0-9]+)\)$`)
	faseRE       = regexp.MustCompile(`(?is)href="(?:/novo/tabela)?\?ID=([0-9]+)&(?:amp;)?fase=([0-9]+)"[^>]*>(.*?)</a>`)
	ogTitleRE    = regexp.MustCompile(`(?i)<meta\b[^>]*property="og:title"[^>]*content="([^"]+)"|<meta\b[^>]*content="([^"]+)"[^>]*property="og:title"`)
	roundRE      = regexp.MustCompile(`(?i)id="bloco-rodada-([0-9]+)"`)
	matchRE      = regexp.MustCompile(`(?is)<a\b[^>]*href="([^"]*jogo=[0-9]+[^"]*)"[^>]*>`)
	jogoRE       = regexp.MustCompile(`(?i)jogo=([0-9]+)`)
	headingRE    = regexp.MustCompile(`(?is)<h3\b[^>]*>(.*?)</h3>`)
	scorePair    = regexp.MustCompile(`(?i)^\s*([0-9]+|-)\s*x\s*([0-9]+|-)\s*$`)
	clockRE      = regexp.MustCompile(`(?is)<p\b[^>]*>(.*?)</p>`)
	whenRE       = regexp.MustCompile(`(?i)(\d{1,2})/(\d{1,2})/(20\d{2})\s+(\d{1,2})h(\d{2})(?:\s+(.*))?$`)
	titleRE      = regexp.MustCompile(`(?i)\btitle="([^"]*)"`)
	imgRE        = regexp.MustCompile(`(?i)<img\b[^>]*src="([^"]*)"`)
	shortRE      = regexp.MustCompile(`(?is)<span\b[^>]*uk-text-bold[^>]*>([^<]*)</span>`)
	badgeRE      = regexp.MustCompile(`(?i)escudo_clubes/([0-9]+)\.`)
	tagRE        = regexp.MustCompile(`(?is)<[^>]+>`)
	errUnnamed   = errors.New("match has no published club")
)

// ParseYears reads the year buttons.
func ParseYears(page string) ([]int, error) {
	seen := map[int]struct{}{}
	var years []int
	for _, found := range yearLinkRE.FindAllStringSubmatch(page, -1) {
		year, err := strconv.Atoi(found[1])
		if err != nil {
			return nil, err
		}
		if _, ok := seen[year]; ok {
			continue
		}
		seen[year] = struct{}{}
		years = append(years, year)
	}
	if len(years) == 0 {
		return nil, fmt.Errorf("catalog page has no years")
	}
	return years, nil
}

// PageYear reads the year the catalog script says it rendered.
func PageYear(page string) (int, error) {
	found := anoRE.FindStringSubmatch(page)
	if found == nil {
		return 0, fmt.Errorf("catalog page has no year")
	}
	return strconv.Atoi(found[1])
}

// ParseCatalog reads adult cards. year is the year this page was requested for.
func ParseCatalog(page string, year int) ([]Link, error) {
	published, err := PageYear(page)
	if err != nil {
		return nil, err
	}
	if published != year {
		return nil, fmt.Errorf("catalog page says %d, not %d", published, year)
	}
	seen := map[string]struct{}{}
	used := map[string]string{}
	var links []Link
	for _, found := range anchorCardRE.FindAllStringSubmatch(page, -1) {
		id := found[1]
		if _, ok := seen[id]; ok {
			continue
		}
		code, err := classifyCard(found[2], year)
		if err != nil {
			return nil, err
		}
		if code == "" {
			continue
		}
		if previous, ok := used[code]; ok && previous != id {
			return nil, fmt.Errorf("catalog lists %s twice in %d", code, year)
		}
		seen[id] = struct{}{}
		used[code] = id
		name, _ := Lookup(code)
		links = append(links, Link{ID: id, Title: fmt.Sprintf("%s / %d", name, year), Year: year, Code: code})
	}
	return links, nil
}

func classifyCard(inner string, pageYear int) (string, error) {
	if image := imageFileRE.FindStringSubmatch(inner); image != nil {
		file := strings.ToLower(image[1])
		code, imageYear, ok := classifyImage(file)
		if !ok {
			return "", nil
		}
		if imageYear != pageYear {
			return "", fmt.Errorf("catalog image %s belongs to %d on the %d page", file, imageYear, pageYear)
		}
		return code, nil
	}
	return classifyText(inner, pageYear), nil
}

func classifyImage(file string) (code string, year int, ok bool) {
	folded := strings.ToLower(file)
	for _, word := range []string{"sub", "femin", "copa", "base", "infantil", "master", "amador"} {
		if strings.Contains(folded, word) {
			return "", 0, false
		}
	}
	if found := imageA.FindStringSubmatch(folded); found != nil {
		year, err := strconv.Atoi(found[1])
		if err != nil {
			return "", 0, false
		}
		return codeA, year, true
	}
	if found := imageB.FindStringSubmatch(folded); found != nil {
		raw := found[1]
		if raw == "" {
			raw = found[2]
		}
		year, err := strconv.Atoi(raw)
		if err != nil {
			return "", 0, false
		}
		return codeB, year, true
	}
	return "", 0, false
}

func classifyText(inner string, pageYear int) string {
	text := fold(clean(inner))
	for _, word := range []string{"SUB", "FEMIN", "COPA", "BASE", "INFANTIL", "MASTER", "AMADOR"} {
		if strings.Contains(text, word) {
			return ""
		}
	}
	found := textDivision.FindStringSubmatch(text)
	if found == nil {
		return ""
	}
	published := found[2]
	if published == "" {
		published = found[3]
	}
	if published != "" && published != strconv.Itoa(pageYear) {
		return ""
	}
	if found[1] == "B" {
		return codeB
	}
	return codeA
}

// ParsePhases reads the stage tabs for competitionID.
func ParsePhases(page, competitionID string) ([]Phase, error) {
	var phases []Phase
	seen := map[string]struct{}{}
	for _, found := range faseRE.FindAllStringSubmatch(page, -1) {
		if found[1] != competitionID {
			continue
		}
		name := clean(found[3])
		if name == "" {
			continue
		}
		if _, ok := seen[found[2]]; ok {
			continue
		}
		seen[found[2]] = struct{}{}
		phases = append(phases, Phase{ID: found[2], Name: name})
	}
	if len(phases) == 0 {
		return nil, fmt.Errorf("season page has no phases")
	}
	return phases, nil
}

// SeasonTitle reads the published season name. It must name this division and year.
func SeasonTitle(page string, year int, code string) (string, error) {
	found := ogTitleRE.FindStringSubmatch(page)
	if found == nil {
		return "", fmt.Errorf("season page has no title")
	}
	title := found[1]
	if title == "" {
		title = found[2]
	}
	title = clean(html.UnescapeString(title))
	folded := fold(title)
	want := "SERIE A"
	if code == codeB {
		want = "SERIE B"
	}
	if !strings.Contains(folded, "ALAGOANO") || !strings.Contains(folded, want) {
		return "", fmt.Errorf("season title %q is not %s", title, want)
	}
	if !strings.HasSuffix(folded, strconv.Itoa(year)) {
		return "", fmt.Errorf("season title %q is not %d", title, year)
	}
	return title, nil
}

// ParseMatches reads one phase page. The clock is Maceió local time.
func ParseMatches(page, phase string, year int, loc *time.Location) ([]Fixture, error) {
	marks := roundRE.FindAllStringSubmatchIndex(page, -1)
	if len(marks) == 0 {
		return nil, nil
	}
	var matches []Fixture
	for i, mark := range marks {
		round, err := strconv.Atoi(page[mark[2]:mark[3]])
		if err != nil {
			return nil, err
		}
		end := len(page)
		if i+1 < len(marks) {
			end = marks[i+1][0]
		}
		section := page[mark[1]:end]
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
	id := ""
	if found := jogoRE.FindStringSubmatch(href); found != nil {
		id = found[1]
	}
	if id == "" {
		return Fixture{}, fmt.Errorf("match in %s round %d has no id", phase, round)
	}
	label := phase
	var homeScore, awayScore *int
	for _, heading := range headingRE.FindAllStringSubmatch(block, -1) {
		text := clean(heading[1])
		pair := scorePair.FindStringSubmatch(text)
		if pair == nil {
			if text != "" {
				label = text
			}
			continue
		}
		if pair[1] == "-" && pair[2] == "-" {
			continue
		}
		if pair[1] == "-" || pair[2] == "-" {
			return Fixture{}, fmt.Errorf("match %s score %q is incomplete", id, text)
		}
		home, err := strconv.Atoi(pair[1])
		if err != nil {
			return Fixture{}, err
		}
		away, err := strconv.Atoi(pair[2])
		if err != nil {
			return Fixture{}, err
		}
		homeScore, awayScore = &home, &away
	}
	tips := titleRE.FindAllStringSubmatch(block, -1)
	imgs := imgRE.FindAllStringSubmatch(block, -1)
	shorts := shortRE.FindAllStringSubmatch(block, -1)
	homeName, awayName := side(tips, 0), side(tips, 1)
	homeShort, awayShort := side(shorts, 0), side(shorts, 1)
	if homeName == "" {
		homeName = homeShort
	}
	if awayName == "" {
		awayName = awayShort
	}
	homeID, awayID := badgeID(side(imgs, 0)), badgeID(side(imgs, 1))
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
	if homeShort == "" {
		homeShort = homeName
	}
	if awayShort == "" {
		awayShort = awayName
	}
	kickoff, venue, homePenalties, awayPenalties, err := clockAndPenalties(block, year, loc)
	if err != nil {
		return Fixture{}, fmt.Errorf("match %s: %w", id, err)
	}
	return Fixture{
		HomeName: homeName, HomeShort: homeShort, AwayName: awayName, AwayShort: awayShort,
		Match: domain.MatchInput{
			ExternalID: id, RoundNumber: round, Phase: label,
			HomeExternalID: homeID, AwayExternalID: awayID,
			Kickoff: kickoff, Status: statusOf(block, homeScore, awayScore, kickoff),
			HomeScore: homeScore, AwayScore: awayScore,
			HomePenalties: homePenalties, AwayPenalties: awayPenalties, Venue: venue,
		},
	}, nil
}

func clockAndPenalties(block string, year int, loc *time.Location) (*time.Time, string, *int, *int, error) {
	var kickoff *time.Time
	var venue string
	var homePenalties, awayPenalties *int
	clockSeen := false
	for _, found := range clockRE.FindAllStringSubmatch(block, -1) {
		text := clean(found[1])
		if text == "" {
			continue
		}
		if pair := penaltyRE.FindStringSubmatch(text); pair != nil {
			home, err := strconv.Atoi(pair[1])
			if err != nil {
				return nil, "", nil, nil, err
			}
			away, err := strconv.Atoi(pair[2])
			if err != nil {
				return nil, "", nil, nil, err
			}
			homePenalties, awayPenalties = &home, &away
			continue
		}
		if clockSeen {
			continue
		}
		clockSeen = true
		when, where, err := kickoffAndVenue(text, year, loc)
		if err != nil {
			return nil, "", nil, nil, err
		}
		kickoff, venue = when, where
	}
	return kickoff, venue, homePenalties, awayPenalties, nil
}

func side(found [][]string, index int) string {
	if index >= len(found) {
		return ""
	}
	return clean(html.UnescapeString(found[index][1]))
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
	published, _ := strconv.Atoi(found[3])
	if published != year {
		return nil, "", fmt.Errorf("clock year %d is not %d", published, year)
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
	return found[1]
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

// ParseStandings reads the classification table. Yellow cards are printed
// before red cards, the same markup as the UIKit catalogs.
func ParseStandings(page string) ([]domain.StandingInput, map[string]uikit.Club, error) {
	return uikit.ParseStandings(page)
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

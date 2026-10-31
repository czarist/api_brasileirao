package cbf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
	"golang.org/x/net/html"
)

var teamHref = regexp.MustCompile(`/times/.+/(\d+)$`)

// ParseStandings reads the official table. Embedded classification groups win
// when they contain at least as many rows as the HTML table. The club id in the
// HTML table is the /times/ link that wraps the team name, not a later link
// such as the next opponent.
func ParseStandings(body []byte) ([]domain.StandingInput, map[string]string, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, nil, fmt.Errorf("standings page is empty")
	}
	embedded, embeddedNames, embeddedErr := parseEmbedded(body)
	if embeddedErr != nil {
		return nil, nil, embeddedErr
	}
	htmlRows, htmlNames, htmlErr := parseHTMLStandings(body)
	if len(embedded) > 0 && (htmlErr != nil || len(embedded) >= len(htmlRows)) {
		return embedded, embeddedNames, nil
	}
	if htmlErr != nil {
		return nil, nil, htmlErr
	}
	return htmlRows, htmlNames, nil
}

func parseHTMLStandings(body []byte) ([]domain.StandingInput, map[string]string, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, nil, fmt.Errorf("standings page is empty")
	}
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("parse standings html: %w", err)
	}
	var rows []domain.StandingInput
	names := make(map[string]string)
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if err != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "tr" {
			row, name, ok, rowErr := standingRow(n)
			if rowErr != nil {
				err = rowErr
				return
			}
			if ok {
				rows = append(rows, row)
				names[row.ExternalTeamID] = name
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if err != nil {
		return nil, nil, err
	}
	if len(rows) == 0 {
		if blocked(body) {
			return nil, nil, fmt.Errorf("standings page is a block page")
		}
		return nil, nil, fmt.Errorf("standings table is empty")
	}
	return rows, names, nil
}

func standingRow(tr *html.Node) (domain.StandingInput, string, bool, error) {
	cells := direct(tr, "td")
	if len(cells) == 0 {
		return domain.StandingInput{}, "", false, nil
	}
	teamID, teamName := teamFromCell(cells[0])
	if teamID == "" {
		return domain.StandingInput{}, "", false, nil
	}
	if len(cells) < 13 {
		return domain.StandingInput{}, "", false, fmt.Errorf("standings row for team %s is incomplete", teamID)
	}
	position, ok := positionFromCell(cells[0])
	if !ok {
		return domain.StandingInput{}, "", false, fmt.Errorf("standings row for team %s has no position", teamID)
	}
	nums := make([]int, 11)
	for i := 0; i < 11; i++ {
		value, err := cellInt(cells[i+1])
		if err != nil {
			return domain.StandingInput{}, "", false, fmt.Errorf("standings row for team %s column %d: %w", teamID, i+1, err)
		}
		nums[i] = value
	}
	row := domain.StandingInput{
		ExternalTeamID: teamID,
		Position:       position,
		Points:         nums[0],
		Played:         nums[1],
		Wins:           nums[2],
		Draws:          nums[3],
		Losses:         nums[4],
		GoalsFor:       nums[5],
		GoalsAgainst:   nums[6],
		GoalDifference: nums[7],
		YellowCards:    nums[8],
		RedCards:       nums[9],
		WinPercentage:  nums[10],
	}
	return row, strings.TrimSpace(teamName), true, nil
}

func teamFromCell(cell *html.Node) (string, string) {
	var id, name string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if id != "" || n == nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" {
			if found, ok := teamIDFromHref(attr(n, "href")); ok {
				if label := namedStrong(n); label != "" {
					id = found
					name = label
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(cell)
	return id, name
}

func positionFromCell(cell *html.Node) (int, bool) {
	var value int
	var found bool
	var walk func(*html.Node, bool)
	walk = func(n *html.Node, insideAnchor bool) {
		if found || n == nil {
			return
		}
		anchor := insideAnchor || (n.Type == html.ElementNode && n.Data == "a")
		if !anchor && n.Type == html.ElementNode && n.Data == "strong" {
			raw := strings.TrimSpace(textContent(n))
			if n, err := strconv.Atoi(raw); err == nil && n > 0 {
				value = n
				found = true
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, anchor)
		}
	}
	walk(cell, false)
	return value, found
}

func namedStrong(anchor *html.Node) string {
	var name string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if name != "" || n == nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "strong" {
			raw := strings.TrimSpace(textContent(n))
			if raw != "" && !isAllDigits(raw) {
				name = raw
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(anchor)
	return name
}

func teamIDFromHref(href string) (string, bool) {
	href = strings.TrimSpace(href)
	if i := strings.Index(href, "?"); i >= 0 {
		href = href[:i]
	}
	m := teamHref.FindStringSubmatch(href)
	if m == nil {
		return "", false
	}
	return m[1], true
}

func cellInt(cell *html.Node) (int, error) {
	raw := strings.TrimSpace(textContent(cell))
	raw = strings.TrimSuffix(raw, "%")
	raw = strings.ReplaceAll(raw, " ", "")
	raw = strings.ReplaceAll(raw, "\u2212", "-")
	if raw == "" {
		return 0, fmt.Errorf("empty cell")
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("value %q", raw)
	}
	return value, nil
}

func direct(n *html.Node, tag string) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == tag {
			out = append(out, c)
		}
	}
	return out
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func textContent(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

func isAllDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func blocked(body []byte) bool {
	sample := body
	if len(sample) > 4096 {
		sample = sample[:4096]
	}
	lower := strings.ToLower(string(sample))
	return strings.Contains(lower, "cf-browser-verification") || strings.Contains(lower, "captcha")
}

type rawSide struct {
	Name     string     `json:"nome"`
	State    string     `json:"uf"`
	BadgeURL string     `json:"url_escudo"`
	Goals    flexString `json:"gols"`
	Penalty  flexString `json:"panaltis"`
}

type rawMatch struct {
	ID     flexString `json:"ref_jogo"`
	Number flexString `json:"num_jogo"`
	Round  flexString `json:"rodada"`
	Venue  string     `json:"estadio"`
	City   string     `json:"cidade"`
	State  string     `json:"uf"`
	Date   string     `json:"data"`
	Time   flexString `json:"hora"`
	Home   rawSide    `json:"mandante"`
	Away   rawSide    `json:"visitante"`
}

type rawPhase struct {
	Matches []rawMatch `json:"jogos"`
}

type rawIMT struct {
	MatchID    flexString `json:"id_jogo"`
	Kind       string     `json:"tipo"`
	Initial    string     `json:"data_inicial"`
	Final      string     `json:"data_final"`
	Requested  string     `json:"data_solicitacao"`
	VenueFinal *string    `json:"nome_estadio_final"`
	Reason     string     `json:"motivo"`
}

// flexString accepts a JSON string or number. The source mixes both.
type flexString string

func (f *flexString) UnmarshalJSON(body []byte) error {
	body = bytes.TrimSpace(body)
	if len(body) == 0 || string(body) == "null" {
		*f = ""
		return nil
	}
	if body[0] == '"' {
		var value string
		if err := json.Unmarshal(body, &value); err != nil {
			return err
		}
		*f = flexString(value)
		return nil
	}
	*f = flexString(body)
	return nil
}

type scheduleChange struct {
	when  time.Time
	start *time.Time
	note  string
	venue string
}

// ParseMatches reads the detailed fixture document. An undefined date
// (01/01/1900) and an empty score stay nil.
func ParseMatches(body []byte, loc *time.Location) ([]domain.MatchInput, []domain.TeamInput, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, nil, fmt.Errorf("match document is empty")
	}
	var phases map[string]rawPhase
	if err := json.Unmarshal(body, &phases); err != nil {
		return nil, nil, fmt.Errorf("parse match document: %w", err)
	}
	if len(phases) == 0 {
		return nil, nil, fmt.Errorf("match document has no phases")
	}
	phasesInOrder := make([]string, 0, len(phases))
	for phase := range phases {
		phasesInOrder = append(phasesInOrder, phase)
	}
	sort.Strings(phasesInOrder)
	var matches []domain.MatchInput
	clubs := map[string]*domain.TeamInput{}
	for _, phase := range phasesInOrder {
		block := phases[phase]
		phase = strings.TrimSpace(phase)
		if phase == "" {
			return nil, nil, fmt.Errorf("match document has an empty phase name")
		}
		for _, raw := range block.Matches {
			match, home, away, err := convertMatch(phase, raw, loc)
			if err != nil {
				return nil, nil, err
			}
			matches = append(matches, match)
			addClub(clubs, home)
			addClub(clubs, away)
		}
	}
	if len(matches) == 0 {
		return nil, nil, fmt.Errorf("match document has no matches")
	}
	teams := make([]domain.TeamInput, 0, len(clubs))
	for _, club := range clubs {
		teams = append(teams, *club)
	}
	return matches, teams, nil
}

func convertMatch(phase string, raw rawMatch, loc *time.Location) (domain.MatchInput, domain.TeamInput, domain.TeamInput, error) {
	id := strings.TrimSpace(string(raw.ID))
	if id == "" {
		return domain.MatchInput{}, domain.TeamInput{}, domain.TeamInput{}, fmt.Errorf("match is missing ref_jogo")
	}
	round, err := strconv.Atoi(strings.TrimSpace(string(raw.Round)))
	if err != nil || round <= 0 {
		return domain.MatchInput{}, domain.TeamInput{}, domain.TeamInput{}, fmt.Errorf("match %s has round %q", id, raw.Round)
	}
	number, _ := strconv.Atoi(strings.TrimSpace(string(raw.Number)))
	homeScore, err := parseScore(string(raw.Home.Goals))
	if err != nil {
		return domain.MatchInput{}, domain.TeamInput{}, domain.TeamInput{}, fmt.Errorf("match %s home score: %w", id, err)
	}
	awayScore, err := parseScore(string(raw.Away.Goals))
	if err != nil {
		return domain.MatchInput{}, domain.TeamInput{}, domain.TeamInput{}, fmt.Errorf("match %s away score: %w", id, err)
	}
	homePens, err := parseScore(string(raw.Home.Penalty))
	if err != nil {
		return domain.MatchInput{}, domain.TeamInput{}, domain.TeamInput{}, fmt.Errorf("match %s home penalties: %w", id, err)
	}
	awayPens, err := parseScore(string(raw.Away.Penalty))
	if err != nil {
		return domain.MatchInput{}, domain.TeamInput{}, domain.TeamInput{}, fmt.Errorf("match %s away penalties: %w", id, err)
	}
	home, err := clubFromSide(raw.Home)
	if err != nil {
		return domain.MatchInput{}, domain.TeamInput{}, domain.TeamInput{}, fmt.Errorf("match %s home: %w", id, err)
	}
	away, err := clubFromSide(raw.Away)
	if err != nil {
		return domain.MatchInput{}, domain.TeamInput{}, domain.TeamInput{}, fmt.Errorf("match %s away: %w", id, err)
	}
	kickoff, err := kickoffFromParts(raw.Date, string(raw.Time), loc)
	if err != nil {
		return domain.MatchInput{}, domain.TeamInput{}, domain.TeamInput{}, fmt.Errorf("match %s kickoff: %w", id, err)
	}
	match := domain.MatchInput{
		ExternalID:     id,
		MatchNumber:    number,
		RoundNumber:    round,
		Phase:          phase,
		HomeExternalID: home.ExternalID,
		AwayExternalID: away.ExternalID,
		Kickoff:        kickoff,
		Status:         statusFor(homeScore, awayScore, kickoff, ""),
		HomeScore:      homeScore,
		AwayScore:      awayScore,
		HomePenalties:  homePens,
		AwayPenalties:  awayPens,
		Venue:          strings.TrimSpace(raw.Venue),
		City:           strings.TrimSpace(raw.City),
		State:          strings.TrimSpace(raw.State),
	}
	return match, home, away, nil
}

func clubFromSide(side rawSide) (domain.TeamInput, error) {
	id, err := clubID(side.BadgeURL)
	if err != nil {
		return domain.TeamInput{}, err
	}
	name := strings.TrimSpace(side.Name)
	if name == "" {
		return domain.TeamInput{}, fmt.Errorf("club %s has no name", id)
	}
	return domain.TeamInput{
		ExternalID: id,
		Name:       name,
		ShortName:  ShortName(name),
		State:      strings.TrimSpace(side.State),
	}, nil
}

var clubIDPattern = regexp.MustCompile(`/clubes/(\d+)/`)

func clubID(badgeURL string) (string, error) {
	m := clubIDPattern.FindStringSubmatch(badgeURL)
	if m == nil {
		return "", fmt.Errorf("badge url %q has no club id", badgeURL)
	}
	return m[1], nil
}

func parseScore(raw string) (*int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return nil, fmt.Errorf("invalid score %q", raw)
	}
	return &value, nil
}

func kickoffFromParts(date, clock string, loc *time.Location) (*time.Time, error) {
	date = strings.TrimSpace(date)
	clock = strings.TrimSpace(clock)
	if date == "" || strings.HasPrefix(date, "01/01/1900") {
		return nil, nil
	}
	if clock == "" || clock == "::00" || strings.HasPrefix(clock, ":") {
		return nil, nil
	}
	parsed, err := time.ParseInLocation("02/01/2006 15:04", date+" "+clock, loc)
	if err != nil {
		return nil, err
	}
	if parsed.Year() < 1990 {
		return nil, nil
	}
	utc := parsed.UTC()
	return &utc, nil
}

// ParseIMT indexes schedule changes by match id. Null and an empty array are
// no changes. The latest data_solicitacao wins when a match appears twice.
func ParseIMT(body []byte, loc *time.Location) (map[string]scheduleChange, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("schedule feed is empty")
	}
	if bytes.Equal(trimmed, []byte("null")) {
		return map[string]scheduleChange{}, nil
	}
	if trimmed[0] != '[' {
		return nil, fmt.Errorf("schedule feed is not a list")
	}
	var rows []rawIMT
	if err := json.Unmarshal(trimmed, &rows); err != nil {
		return nil, fmt.Errorf("parse schedule feed: %w", err)
	}
	out := make(map[string]scheduleChange, len(rows))
	for _, row := range rows {
		id := strings.TrimSpace(string(row.MatchID))
		if id == "" {
			continue
		}
		change := scheduleChange{
			note:  strings.TrimSpace(row.Kind + " " + row.Reason),
			venue: cleanVenue(row.VenueFinal),
		}
		if when, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(row.Requested), loc); err == nil {
			change.when = when
		}
		if start, err := parseIMTTime(row.Final, loc); err == nil {
			change.start = start
		}
		current, ok := out[id]
		if !ok || change.when.After(current.when) {
			out[id] = change
		}
	}
	return out, nil
}

func parseIMTTime(raw string, loc *time.Location) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "1900-01-01") || strings.Contains(raw, "::") {
		return nil, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", raw, loc)
	if err != nil {
		return nil, err
	}
	if parsed.Year() < 1990 {
		return nil, nil
	}
	utc := parsed.UTC()
	return &utc, nil
}

func cleanVenue(value *string) string {
	if value == nil {
		return ""
	}
	venue := strings.TrimSpace(*value)
	folded := strings.ToLower(venue)
	switch folded {
	case "", "-", "- a definir", "a definir":
		return ""
	default:
		return venue
	}
}

func statusFor(home, away *int, kickoff *time.Time, note string) domain.MatchStatus {
	if home != nil && away != nil {
		return domain.StatusFinished
	}
	folded := fold(note)
	switch {
	case strings.Contains(folded, "cancel"):
		return domain.StatusCancelled
	case strings.Contains(folded, "suspens"):
		return domain.StatusSuspended
	case kickoff == nil && (strings.Contains(folded, "remarca") || strings.Contains(folded, "adiad")):
		return domain.StatusPostponed
	default:
		return domain.StatusScheduled
	}
}

func addClub(clubs map[string]*domain.TeamInput, club domain.TeamInput) {
	current, ok := clubs[club.ExternalID]
	if !ok {
		clubs[club.ExternalID] = &club
		return
	}
	if current.State == "" {
		current.State = club.State
	}
	rememberAlias(current, club.Name)
}

func rememberAlias(club *domain.TeamInput, name string) {
	name = strings.TrimSpace(name)
	if name == "" || fold(name) == fold(club.Name) {
		return
	}
	for _, alias := range club.Aliases {
		if fold(alias) == fold(name) {
			return
		}
	}
	club.Aliases = append(club.Aliases, name)
}

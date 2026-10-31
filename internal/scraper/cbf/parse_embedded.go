package cbf

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/czarist/api_brasileirao/internal/domain"
)

type embeddedRow struct {
	TeamID         string `json:"cod_time"`
	Name           string `json:"time"`
	Position       string `json:"posicao"`
	Points         string `json:"pontos"`
	Played         string `json:"jogos"`
	Wins           string `json:"vitorias"`
	Draws          string `json:"empates"`
	Losses         string `json:"derrotas"`
	GoalsFor       string `json:"gols_pro"`
	GoalsAgainst   string `json:"gols_contra"`
	GoalDifference string `json:"gols_saldo"`
	YellowCards    string `json:"cartoes_amarelo"`
	RedCards       string `json:"cartoes_vermelho"`
	WinPercentage  string `json:"aproveitamento"`
}

// parseEmbedded reads the classification groups Next.js embeds in the page.
// A page without those groups returns no rows. Malformed group JSON is an error.
func parseEmbedded(body []byte) ([]domain.StandingInput, map[string]string, error) {
	text := string(body)
	rows, names, err := parseEmbeddedText(text)
	if err == nil && len(rows) > 0 {
		return rows, names, nil
	}
	plain := strings.ReplaceAll(text, `\"`, `"`)
	if plain == text {
		return rows, names, err
	}
	plainRows, plainNames, plainErr := parseEmbeddedText(plain)
	if plainErr == nil && len(plainRows) > 0 {
		return plainRows, plainNames, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return nil, nil, plainErr
}

func parseEmbeddedText(text string) ([]domain.StandingInput, map[string]string, error) {
	var rows []domain.StandingInput
	names := map[string]string{}
	order := 0
	for search := text; ; {
		titleAt := strings.Index(search, `"title":"`)
		if titleAt < 0 {
			break
		}
		search = search[titleAt+len(`"title":"`):]
		end := strings.IndexByte(search, '"')
		if end < 0 {
			break
		}
		title := decodeJSONString(search[:end])
		search = search[end+1:]
		if !strings.Contains(strings.ToUpper(title), "GRUPO") {
			continue
		}
		dataAt := strings.Index(search, `"data":[`)
		nextTitle := strings.Index(search, `"title":"`)
		if dataAt < 0 || dataAt > 400 || (nextTitle >= 0 && nextTitle < dataAt) {
			continue
		}
		raw, next, err := jsonArray(search[dataAt+len(`"data":`):])
		if err != nil {
			return nil, nil, fmt.Errorf("standings group %s: %w", title, err)
		}
		var decoded []embeddedRow
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return nil, nil, fmt.Errorf("standings group %s: %w", title, err)
		}
		if len(decoded) == 0 {
			return nil, nil, fmt.Errorf("standings group %s is empty", title)
		}
		for _, item := range decoded {
			row, err := item.standing(title, order)
			if err != nil {
				return nil, nil, err
			}
			if _, ok := names[row.ExternalTeamID]; ok {
				return nil, nil, fmt.Errorf("standings team %s is repeated", row.ExternalTeamID)
			}
			names[row.ExternalTeamID] = item.Name
			rows = append(rows, row)
		}
		order++
		search = next
	}
	return rows, names, nil
}

func (item embeddedRow) standing(group string, order int) (domain.StandingInput, error) {
	if !digitsOnly.MatchString(item.TeamID) {
		return domain.StandingInput{}, fmt.Errorf("standings team id %q is not numeric", item.TeamID)
	}
	position, err := atoiField(item.Position)
	if err != nil || position <= 0 {
		return domain.StandingInput{}, fmt.Errorf("standings position for team %s: %v", item.TeamID, err)
	}
	values := []string{
		item.Points, item.Played, item.Wins, item.Draws, item.Losses,
		item.GoalsFor, item.GoalsAgainst, item.GoalDifference,
		item.YellowCards, item.RedCards, item.WinPercentage,
	}
	nums := make([]int, len(values))
	for i, value := range values {
		nums[i], err = atoiField(value)
		if err != nil {
			return domain.StandingInput{}, fmt.Errorf("standings team %s column %d: %w", item.TeamID, i+1, err)
		}
	}
	return domain.StandingInput{
		ExternalTeamID: item.TeamID,
		GroupName:      group,
		GroupOrder:     order,
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
	}, nil
}

func atoiField(raw string) (int, error) {
	raw = strings.TrimSpace(strings.TrimSuffix(raw, "%"))
	raw = strings.NewReplacer("\u2212", "-", "−", "-").Replace(raw)
	if raw == "" {
		return 0, fmt.Errorf("empty number")
	}
	return strconv.Atoi(raw)
}

func jsonArray(text string) ([]byte, string, error) {
	if !strings.HasPrefix(text, "[") {
		return nil, "", fmt.Errorf("rows are not an array")
	}
	depth := 0
	inString := false
	escaped := false
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				raw := []byte(text[:i+1])
				if !json.Valid(raw) {
					return nil, "", fmt.Errorf("rows are not valid json")
				}
				return raw, text[i+1:], nil
			}
		}
	}
	return nil, "", fmt.Errorf("rows array is truncated")
}

func decodeJSONString(raw string) string {
	var value string
	if err := json.Unmarshal([]byte(`"`+raw+`"`), &value); err != nil || value == "" {
		return raw
	}
	return value
}

// firstPointsPhase returns the id of the first points phase on a year page.
// A knockout page lists that phase before the classification phase.
func firstPointsPhase(body []byte) (string, bool) {
	text := strings.ReplaceAll(string(body), `\"`, `"`)
	const marker = `"fase_tipo":"pontuacao"`
	for search := text; ; {
		at := strings.Index(search, marker)
		if at < 0 {
			return "", false
		}
		head := search[:at]
		idAt := strings.LastIndex(head, `"fase_id":"`)
		if idAt >= 0 {
			rest := head[idAt+len(`"fase_id":"`):]
			end := strings.IndexByte(rest, '"')
			if end > 0 && digitsOnly.MatchString(rest[:end]) {
				return rest[:end], true
			}
		}
		search = search[at+len(marker):]
	}
}

package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/czarist/api_brasileirao/docs"
	"github.com/czarist/api_brasileirao/internal/domain"
)

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"name":    "api-brasileirao",
		"docs":    "/docs",
		"openapi": "/openapi.yaml",
		"health":  "/health/live",
		"api":     "/api/v1/competitions",
	})
}

func (h *Handler) live(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	if err := h.reader.Ping(r.Context()); err != nil {
		h.options.Log.Error("readiness failed", "request_id", requestIDFrom(r), "error", err.Error())
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) openAPI(w http.ResponseWriter, r *http.Request) {
	body, err := docs.FS.ReadFile("openapi.yaml")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error", requestIDFrom(r))
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *Handler) docs(w http.ResponseWriter, r *http.Request) {
	body, err := docs.FS.ReadFile("swagger.html")
	if err != nil {
		http.Error(w, "docs unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *Handler) asset(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	var contentType string
	switch name {
	case "swagger-ui-bundle.js":
		contentType = "text/javascript; charset=utf-8"
	case "swagger-ui.css":
		contentType = "text/css; charset=utf-8"
	default:
		h.unknown(w, r)
		return
	}
	body, err := docs.FS.ReadFile("assets/" + name)
	if err != nil {
		h.unknown(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *Handler) competitions(w http.ResponseWriter, r *http.Request) {
	items, meta, err := h.reader.ListCompetitions(r.Context())
	if err != nil {
		h.writeStoredError(w, r, err, "competition not found")
		return
	}
	data := make([]competitionDTO, 0, len(items))
	for _, item := range items {
		data = append(data, competitionDTO{ID: item.ID, Code: item.Code, Name: item.Name})
	}
	writeJSON(w, http.StatusOK, listDTO[competitionDTO]{Data: data, Source: newSourceDTO(meta)})
}

func (h *Handler) competition(w http.ResponseWriter, r *http.Request) {
	code, err := pathCode(r)
	if err != nil {
		h.writeStoredError(w, r, err, "")
		return
	}
	item, meta, err := h.reader.GetCompetition(r.Context(), code)
	if err != nil {
		h.writeStoredError(w, r, err, "competition not found")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Data   competitionDTO `json:"data"`
		Source sourceDTO      `json:"source"`
	}{Data: competitionDTO{ID: item.ID, Code: item.Code, Name: item.Name}, Source: newSourceDTO(meta)})
}

func (h *Handler) seasons(w http.ResponseWriter, r *http.Request) {
	code, err := pathCode(r)
	if err != nil {
		h.writeStoredError(w, r, err, "")
		return
	}
	items, meta, err := h.reader.ListSeasons(r.Context(), code)
	if err != nil {
		h.writeStoredError(w, r, err, "competition not found")
		return
	}
	data := make([]seasonDTO, 0, len(items))
	for _, item := range items {
		data = append(data, seasonDTO{Year: item.Year, Name: item.Name, ExternalID: item.ExternalID})
	}
	writeJSON(w, http.StatusOK, listDTO[seasonDTO]{Data: data, Source: newSourceDTO(meta)})
}

func (h *Handler) season(w http.ResponseWriter, r *http.Request) {
	code, year, err := pathSeason(r)
	if err != nil {
		h.writeStoredError(w, r, err, "")
		return
	}
	item, meta, err := h.reader.GetSeason(r.Context(), code, year)
	if err != nil {
		h.writeStoredError(w, r, err, "season not found")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Data   seasonDTO `json:"data"`
		Source sourceDTO `json:"source"`
	}{Data: seasonDTO{Year: item.Year, Name: item.Name, ExternalID: item.ExternalID}, Source: newSourceDTO(meta)})
}

func (h *Handler) teams(w http.ResponseWriter, r *http.Request) {
	code, year, err := pathSeason(r)
	if err != nil {
		h.writeStoredError(w, r, err, "")
		return
	}
	items, meta, err := h.reader.ListTeams(r.Context(), code, year)
	if err != nil {
		h.writeStoredError(w, r, err, "season not found")
		return
	}
	data := make([]teamDTO, 0, len(items))
	for _, item := range items {
		data = append(data, newTeamDTO(item))
	}
	writeJSON(w, http.StatusOK, listDTO[teamDTO]{Data: data, Source: newSourceDTO(meta)})
}

func (h *Handler) standings(w http.ResponseWriter, r *http.Request) {
	code, year, err := pathSeason(r)
	if err != nil {
		h.writeStoredError(w, r, err, "")
		return
	}
	items, meta, err := h.reader.ListStandings(r.Context(), code, year)
	if err != nil {
		h.writeStoredError(w, r, err, "season not found")
		return
	}
	data := make([]standingDTO, 0, len(items))
	for _, item := range items {
		data = append(data, standingDTO{
			Position: item.Position, GroupName: item.GroupName, Team: newTeamDTO(item.Team), Played: item.Played, Wins: item.Wins,
			Draws: item.Draws, Losses: item.Losses, GoalsFor: item.GoalsFor, GoalsAgainst: item.GoalsAgainst,
			GoalDifference: item.GoalDifference, Points: item.Points, YellowCards: item.YellowCards,
			RedCards: item.RedCards, WinPercentage: item.WinPercentage,
		})
	}
	writeJSON(w, http.StatusOK, listDTO[standingDTO]{Data: data, Source: newSourceDTO(meta)})
}

func (h *Handler) rounds(w http.ResponseWriter, r *http.Request) {
	code, year, err := pathSeason(r)
	if err != nil {
		h.writeStoredError(w, r, err, "")
		return
	}
	items, meta, err := h.reader.ListRounds(r.Context(), code, year)
	if err != nil {
		h.writeStoredError(w, r, err, "season not found")
		return
	}
	data := make([]roundDTO, 0, len(items))
	for _, item := range items {
		data = append(data, roundDTO{Number: item.Number, Phase: item.Phase, Name: item.Name, MatchCount: item.MatchCount})
	}
	writeJSON(w, http.StatusOK, listDTO[roundDTO]{Data: data, Source: newSourceDTO(meta)})
}

func (h *Handler) matches(w http.ResponseWriter, r *http.Request) {
	code, year, err := pathSeason(r)
	if err != nil {
		h.writeStoredError(w, r, err, "")
		return
	}
	filter, err := parseFilter(r)
	if err != nil {
		h.writeStoredError(w, r, err, "")
		return
	}
	page, err := h.reader.ListMatches(r.Context(), code, year, filter)
	if err != nil {
		h.writeStoredError(w, r, err, "season not found")
		return
	}
	data := make([]matchDTO, 0, len(page.Items))
	for _, item := range page.Items {
		data = append(data, newMatchDTO(item))
	}
	writeJSON(w, http.StatusOK, matchListDTO{
		Data: data, Page: page.Page, PageSize: page.PageSize, Total: page.Total, Source: newSourceDTO(page.Source),
	})
}

func (h *Handler) match(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		h.writeStoredError(w, r, &domain.ValidationError{Message: "match id must be a positive integer"}, "")
		return
	}
	item, meta, err := h.reader.GetMatch(r.Context(), id)
	if err != nil {
		h.writeStoredError(w, r, err, "match not found")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Data   matchDTO  `json:"data"`
		Source sourceDTO `json:"source"`
	}{Data: newMatchDTO(item), Source: newSourceDTO(meta)})
}

func (h *Handler) unknown(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "resource not found", requestIDFrom(r))
}

func pathCode(r *http.Request) (string, error) {
	code := r.PathValue("code")
	for _, r := range code {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return "", &domain.ValidationError{Message: "competition code is invalid"}
		}
	}
	if code == "" {
		return "", &domain.ValidationError{Message: "competition code is invalid"}
	}
	return code, nil
}

func pathSeason(r *http.Request) (string, int, error) {
	code, err := pathCode(r)
	if err != nil {
		return "", 0, err
	}
	year, err := strconv.Atoi(r.PathValue("year"))
	if err != nil || year < 1990 || year > 2100 {
		return "", 0, &domain.ValidationError{Message: "year must be an integer from 1990 to 2100"}
	}
	return code, year, nil
}

func parseFilter(r *http.Request) (domain.MatchFilter, error) {
	q := r.URL.Query()
	filter := domain.MatchFilter{Page: 1, PageSize: 20}
	if raw := q.Get("page"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return filter, &domain.ValidationError{Message: "page must be a positive integer"}
		}
		filter.Page = n
	}
	if raw := q.Get("page_size"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			return filter, &domain.ValidationError{Message: "page_size must be from 1 to 100"}
		}
		filter.PageSize = n
	}
	if raw := q.Get("round"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return filter, &domain.ValidationError{Message: "round must be a positive integer"}
		}
		filter.Round = &n
	}
	if raw := q.Get("team_id"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 1 {
			return filter, &domain.ValidationError{Message: "team_id must be a positive integer"}
		}
		filter.TeamID = &n
	}
	if raw := q.Get("status"); raw != "" {
		status := domain.MatchStatus(raw)
		if !status.Valid() {
			return filter, &domain.ValidationError{Message: "status is invalid"}
		}
		filter.Status = &status
	}
	var err error
	if filter.From, err = parseTime(q.Get("from"), "from"); err != nil {
		return filter, err
	}
	if filter.To, err = parseTime(q.Get("to"), "to"); err != nil {
		return filter, err
	}
	if filter.From != nil && filter.To != nil && filter.To.Before(*filter.From) {
		return filter, &domain.ValidationError{Message: "to must be equal to or after from"}
	}
	return filter, nil
}

func parseTime(raw, name string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, &domain.ValidationError{Message: name + " must be an RFC3339 timestamp"}
	}
	utc := parsed.UTC()
	return &utc, nil
}

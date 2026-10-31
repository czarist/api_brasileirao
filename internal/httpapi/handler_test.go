package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
)

type fakeReader struct {
	pingErr error
	meta    domain.SourceMeta
	match   domain.Match
	matches domain.Page[domain.Match]
}

func (f fakeReader) Ping(ctx context.Context) error { return f.pingErr }
func (f fakeReader) ListCompetitions(ctx context.Context) ([]domain.Competition, domain.SourceMeta, error) {
	return []domain.Competition{{ID: 1, Code: "brasileirao-serie-a", Name: "Campeonato Brasileiro Série A"}}, f.meta, nil
}
func (f fakeReader) GetCompetition(ctx context.Context, code string) (domain.Competition, domain.SourceMeta, error) {
	if code != "brasileirao-serie-a" {
		return domain.Competition{}, domain.SourceMeta{}, domain.ErrNotFound
	}
	return domain.Competition{ID: 1, Code: code, Name: "Campeonato Brasileiro Série A"}, f.meta, nil
}
func (f fakeReader) ListSeasons(ctx context.Context, code string) ([]domain.Season, domain.SourceMeta, error) {
	if _, _, err := f.GetCompetition(ctx, code); err != nil {
		return nil, domain.SourceMeta{}, err
	}
	return []domain.Season{{Year: 2026, Name: "Campeonato Brasileiro Série A 2026", ExternalID: "1260611"}}, f.meta, nil
}
func (f fakeReader) GetSeason(ctx context.Context, code string, year int) (domain.Season, domain.SourceMeta, error) {
	if code != "brasileirao-serie-a" || year != 2026 {
		return domain.Season{}, domain.SourceMeta{}, domain.ErrNotFound
	}
	return domain.Season{Year: 2026, Name: "Campeonato Brasileiro Série A 2026", ExternalID: "1260611"}, f.meta, nil
}
func (f fakeReader) ListTeams(ctx context.Context, code string, year int) ([]domain.Team, domain.SourceMeta, error) {
	if _, _, err := f.GetSeason(ctx, code, year); err != nil {
		return nil, domain.SourceMeta{}, err
	}
	return []domain.Team{{ID: 7, Name: "Flamengo", ShortName: "FLA", State: "RJ"}}, f.meta, nil
}
func (f fakeReader) ListStandings(ctx context.Context, code string, year int) ([]domain.Standing, domain.SourceMeta, error) {
	if _, _, err := f.GetSeason(ctx, code, year); err != nil {
		return nil, domain.SourceMeta{}, err
	}
	return []domain.Standing{{Position: 1, Points: 73, Team: domain.Team{ID: 7, Name: "Flamengo", ShortName: "FLA"}}}, f.meta, nil
}
func (f fakeReader) ListRounds(ctx context.Context, code string, year int) ([]domain.Round, domain.SourceMeta, error) {
	if _, _, err := f.GetSeason(ctx, code, year); err != nil {
		return nil, domain.SourceMeta{}, err
	}
	return []domain.Round{{Number: 1, Phase: "Fase Única", Name: "Round 1", MatchCount: 10}}, f.meta, nil
}
func (f fakeReader) ListMatches(ctx context.Context, code string, year int, filter domain.MatchFilter) (domain.Page[domain.Match], error) {
	if _, _, err := f.GetSeason(ctx, code, year); err != nil {
		return domain.Page[domain.Match]{}, err
	}
	page := f.matches
	page.Page = filter.Page
	page.PageSize = filter.PageSize
	page.Source = f.meta
	if page.Items == nil {
		page.Items = []domain.Match{}
	}
	return page, nil
}
func (f fakeReader) GetMatch(ctx context.Context, id int64) (domain.Match, domain.SourceMeta, error) {
	if id != f.match.ID {
		return domain.Match{}, domain.SourceMeta{}, domain.ErrNotFound
	}
	return f.match, f.meta, nil
}

func TestHealthAndDocsStayUpWhenTheDatabaseIsDown(t *testing.T) {
	h := New(fakeReader{pingErr: errors.New("dial tcp: refused")}, Options{CORSOrigins: []string{"*"}, RateBurst: 5})
	live := get(t, h, "/health/live")
	if live.Code != 200 {
		t.Fatalf("live = %d", live.Code)
	}
	ready := get(t, h, "/health/ready")
	if ready.Code != 503 {
		t.Fatalf("ready = %d", ready.Code)
	}
	if get(t, h, "/docs").Code != 200 || get(t, h, "/openapi.yaml").Code != 200 {
		t.Fatal("docs were not served")
	}
	asset := get(t, h, "/docs/assets/swagger-ui.css")
	if asset.Code != 200 || asset.Body.Len() < 1000 {
		t.Fatalf("css = %d bytes %d", asset.Code, asset.Body.Len())
	}
	if get(t, h, "/docs/assets/secret.js").Code != 404 {
		t.Fatal("unknown asset was served")
	}
}

func TestErrorsPaginationAndStaleFlag(t *testing.T) {
	collected := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	meta := domain.SourceMeta{Name: "cbf", CollectedAt: &collected, LastSuccessAt: &collected, Stale: true, CollectionStatus: "ok"}
	match := domain.Match{ID: 4, Status: domain.StatusScheduled, RoundNumber: 21, Phase: "Fase Única", RoundName: "Round 21", Home: domain.Team{ID: 1, Name: "Chapecoense", ShortName: "CHA"}, Away: domain.Team{ID: 2, Name: "Vasco da Gama Saf", ShortName: "VAS"}, CollectedAt: collected, UpdatedAt: collected}
	h := New(fakeReader{meta: meta, match: match, matches: domain.Page[domain.Match]{Items: []domain.Match{match}, Total: 1}}, Options{CORSOrigins: []string{"https://example.com"}, RateBurst: 20})

	bad := get(t, h, "/api/v1/matches/nope")
	if bad.Code != 400 || jsonField(t, bad, "code") != "invalid" || jsonField(t, bad, "request_id") == "" {
		t.Fatalf("bad id = %d %s", bad.Code, bad.Body.String())
	}
	missing := get(t, h, "/api/v1/matches/9")
	if missing.Code != 404 {
		t.Fatalf("missing = %d", missing.Code)
	}
	status := get(t, h, "/api/v1/competitions/brasileirao-serie-a/seasons/2026/matches?status=paused")
	if status.Code != 400 {
		t.Fatalf("status = %d %s", status.Code, status.Body.String())
	}
	page := get(t, h, "/api/v1/competitions/brasileirao-serie-a/seasons/2026/matches?page=2&page_size=1")
	if page.Code != 200 || jsonField(t, page, "page") != "2" || jsonField(t, page, "stale") != "true" {
		t.Fatalf("page = %s", page.Body.String())
	}
	if page.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("cors reflected without an origin")
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/competitions", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Fatalf("cors = %s", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	standings := get(t, h, "/api/v1/competitions/brasileirao-serie-a/seasons/2026/standings")
	if standings.Code != 200 || jsonField(t, standings, "points") != "73" {
		t.Fatalf("standings = %s", standings.Body.String())
	}
}

func TestRateLimit(t *testing.T) {
	h := New(fakeReader{}, Options{CORSOrigins: []string{"*"}, RateRPS: 0.01, RateBurst: 1})
	first := get(t, h, "/api/v1/competitions")
	second := get(t, h, "/api/v1/competitions")
	if first.Code != 200 || second.Code != 429 || second.Header().Get("Retry-After") != "1" {
		t.Fatalf("first %d second %d retry %s", first.Code, second.Code, second.Header().Get("Retry-After"))
	}
	if get(t, h, "/health/live").Code != 200 {
		t.Fatal("health was rate limited")
	}
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func jsonField(t *testing.T, rec *httptest.ResponseRecorder, key string) string {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if value, ok := body[key]; ok {
		return stringify(value)
	}
	return findKey(body, key)
}

func findKey(value any, key string) string {
	switch typed := value.(type) {
	case map[string]any:
		if item, ok := typed[key]; ok {
			return stringify(item)
		}
		for _, item := range typed {
			if found := findKey(item, key); found != "" {
				return found
			}
		}
	case []any:
		for _, item := range typed {
			if found := findKey(item, key); found != "" {
				return found
			}
		}
	}
	return ""
}

func stringify(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconvFormat(typed)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		raw, _ := json.Marshal(typed)
		return string(raw)
	}
}

func strconvFormat(value float64) string {
	return jsonNumber(value)
}

func jsonNumber(value float64) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

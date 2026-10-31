package httpapi

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestOpenAPIDocument(t *testing.T) {
	body, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["openapi"] != "3.1.0" {
		t.Fatalf("openapi = %v", doc["openapi"])
	}
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatal("paths missing")
	}
	required := []string{
		"/",
		"/health/live",
		"/health/ready",
		"/openapi.yaml",
		"/docs",
		"/api/v1/competitions",
		"/api/v1/competitions/{code}",
		"/api/v1/competitions/{code}/seasons",
		"/api/v1/competitions/{code}/seasons/{year}",
		"/api/v1/competitions/{code}/seasons/{year}/teams",
		"/api/v1/competitions/{code}/seasons/{year}/standings",
		"/api/v1/competitions/{code}/seasons/{year}/rounds",
		"/api/v1/competitions/{code}/seasons/{year}/matches",
		"/api/v1/matches/{id}",
	}
	for _, path := range required {
		if _, ok := paths[path]; !ok {
			t.Errorf("missing path %s", path)
		}
	}
	raw := string(body)
	for _, needle := range []string{"home_score:", "kickoff_at:", "\"null\"", "request_id:", "page_size:", "goal_difference:"} {
		if !contains(raw, needle) {
			t.Errorf("spec is missing %s", needle)
		}
	}
}

func contains(body, needle string) bool {
	return len(body) >= len(needle) && (body == needle || len(needle) == 0 || indexOf(body, needle) >= 0)
}

func indexOf(body, needle string) int {
	for i := 0; i+len(needle) <= len(body); i++ {
		if body[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

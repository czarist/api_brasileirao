package cbf

import (
	"context"
	"fmt"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
)

// Source collects one Série A season from the configured origin.
type Source struct {
	client *Client
}

// NewSource binds the adapter to a client. Paths are fixed templates.
func NewSource(client *Client) *Source {
	return &Source{client: client}
}

// Fetch downloads the catalog, the fixture list, schedule changes, and the
// standings page. It does not write a database.
func (s *Source) Fetch(ctx context.Context, year int, collected time.Time) (domain.Snapshot, error) {
	if year < 1990 || year > 2100 {
		return domain.Snapshot{}, fmt.Errorf("season year %d is out of range", year)
	}
	catalog, err := s.client.GetJSON(ctx, "/api/cbf/campeonatos/anos")
	if err != nil {
		return domain.Snapshot{}, err
	}
	externalID, err := SeasonExternalID(catalog, year)
	if err != nil {
		return domain.Snapshot{}, err
	}
	matches, err := s.client.GetJSON(ctx, "/api/cbf/jogos/tabela-detalhada/campeonato/"+externalID)
	if err != nil {
		return domain.Snapshot{}, err
	}
	imt, err := s.client.GetJSON(ctx, "/api/cbf/campeonatos/"+externalID+"/imt")
	if err != nil {
		return domain.Snapshot{}, err
	}
	standings, err := s.client.GetHTML(ctx, fmt.Sprintf("/futebol-brasileiro/tabelas/campeonato-brasileiro/serie-a/%d", year))
	if err != nil {
		return domain.Snapshot{}, err
	}
	return Assemble(year, externalID, collected, matches, imt, standings)
}

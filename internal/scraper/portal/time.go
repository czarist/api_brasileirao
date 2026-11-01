package portal

import (
	"fmt"
	"time"
	_ "time/tzdata"
)

func saoPaulo() (*time.Location, error) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return nil, fmt.Errorf("load America/Sao_Paulo: %w", err)
	}
	return loc, nil
}

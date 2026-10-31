package cbf

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

// CurrentYear is the calendar year in America/Sao_Paulo.
func CurrentYear(now time.Time) (int, error) {
	loc, err := saoPaulo()
	if err != nil {
		return 0, err
	}
	return now.In(loc).Year(), nil
}

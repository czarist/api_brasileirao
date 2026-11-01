package uikit

import (
	"fmt"
	"time"
	_ "time/tzdata"
)

func loadLocation(name string) (*time.Location, error) {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", name, err)
	}
	return loc, nil
}

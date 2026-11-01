package fbf

import (
	"fmt"
	"time"
	_ "time/tzdata"
)

func bahia() (*time.Location, error) {
	loc, err := time.LoadLocation(location)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", location, err)
	}
	return loc, nil
}

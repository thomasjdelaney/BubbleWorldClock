package main

import (
	"time"

	"charm.land/bubbles/v2/help"
)

func testModel(cities []city) model {
	return model{
		cities: cities,
		now:    time.Date(2026, time.January, 2, 15, 4, 5, 0, time.UTC),
		width:  80,
		help:   help.New(),
		keys:   newKeyMap(),
	}
}

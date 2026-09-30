
# Bubble World Clock

A terminal world clock written in Go using [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), and [LipGloss](https://github.com/charmbracelet/lipgloss).

## Run

Requires Go 1.26.6 or newer.

From the project directory:

```sh
go run .
```

Press `q` or `ctrl+c` to quit. Press `?` to toggle expanded help. The clock view
responds to terminal resizing and keeps its layout within the available width.

## Manage Cities

Press `m` to open city management.

- **Add:** Press `a`, type to filter the offline catalog by city, region,
	country, or timezone, then press `enter`.
- **Select or remove:** Use the up/down arrows or `j`/`k` to select a city;
	press `d` to remove it.
- **Sort:** Press `o` to cycle city name A-Z, city name Z-A, UTC offset ascending,
	and UTC offset descending. Offset sorting uses the current offset, including
	daylight-saving changes. The selection stays with the same city when sorting.
- **Return:** Press `esc` to go back.

Cities and sort order are saved automatically. If saving fails, press `s` to
retry.

The catalog uses each city's IANA timezone. Settings are stored in the operating
system's user config directory, under `BubbleWorldClock/cities.json`. On first
run, an existing user settings file takes priority; otherwise a valid
`cities.json` in the current working directory is imported once, or the embedded
starter list is used. The running program never modifies the repository's
`cities.json`.

The bundled city catalog is derived from [GeoNames](https://www.geonames.org/)
`cities15000` and is licensed under CC BY 4.0. See [data/README.md](data/README.md)
for attribution, source details, and regeneration instructions.

## Development

Run the tests and static checks with:

```sh
go test ./...
go vet ./...
```

## Reference

- [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- [Bubbles](https://github.com/charmbracelet/bubbles)
- [LipGloss](https://github.com/charmbracelet/lipgloss)


# Bubble World Clock

A terminal world clock written in Go using [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), and [LipGloss](https://github.com/charmbracelet/lipgloss).

## Run

From the project directory:

```sh
go run .
```

Press `q` or `ctrl+c` to quit. Press `?` to toggle expanded help. The clock view
responds to terminal resizing and keeps its layout within the available width.

## Manage Cities

Press `m` to manage the clock's cities. Use `a` to open the searchable city
catalog, type to filter by city, region, country, or timezone, and press `enter`
to add the selected place. Use the up/down arrows or `j`/`k` to select a city in
the management view, `d` to remove it, and `esc` to go back. Selected cities are
saved as you make changes.

The city catalog works offline and uses each place's IANA timezone. User
settings are stored in the operating system's user config directory, under
`BubbleWorldClock/cities.json`. On first run, if no user settings exist, a valid
`cities.json` in the current working directory is imported once. Otherwise the
embedded starter list is used. The repository's `cities.json` is never modified
by the running program.

The bundled city catalog is derived from [GeoNames](https://www.geonames.org/)
`cities15000` and is licensed under CC BY 4.0. See [data/README.md](data/README.md)
for attribution, source details, and regeneration instructions.

## Reference

- [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- [Bubbles](https://github.com/charmbracelet/bubbles)
- [LipGloss](https://github.com/charmbracelet/lipgloss)


# Bubble World Clock

A terminal world clock written in Go with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Install

Download the archive for your operating system and CPU from the [latest release](https://github.com/thomasjdelaney/BubbleWorldClock/releases/latest), extract it, then run the `bubble-world-clock` executable (`bubble-world-clock.exe` on Windows). Linux releases also include `.deb` and `.rpm` packages.

To run from source, install Go 1.26.6 or newer, then run:

```sh
go run .
```

## Controls

- `q` or `Ctrl+C`: quit; `?`: show help; `m`: manage cities.
- In city management, press `a` to add a city, `d` to remove one, and `o` to change the sort order. Use the arrow keys or `j`/`k` to select a city.
- In the city picker, type to filter and press `Enter` to add a city; press `Esc` to return.
- Press `s` to retry saving if a save fails.

The city picker searches the bundled offline catalog by city, region, country, or timezone. City settings are saved in the operating system's user config directory under `BubbleWorldClock/cities.json`.

The catalog is derived from GeoNames `cities15000` and licensed under CC BY 4.0. See [data/README.md](data/README.md) for attribution and source details.

## Development

Run tests and static checks with:

```sh
go test ./...
go vet ./...
```

## Release (Maintainers)

Push a version tag to create a GitHub release. For example:

```sh
git tag -a v1.0.0 -m "v1.0.0"
git push origin v1.0.0
```

The release workflow publishes platform archives, Linux `.deb` and `.rpm` packages, and checksums.

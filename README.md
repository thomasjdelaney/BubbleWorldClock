
# Bubble World Clock

A terminal world clock written in Go using [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Run

From the project directory:

```sh
go run .
```

Press `q` or `ctrl+c` to quit. Press `?` to toggle expanded help. The clock view
responds to terminal resizing and keeps its layout within the available width.

## Configure Cities

Edit `cities.json` to choose which cities are displayed. Each city needs a display name and an [IANA time zone](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones):

```json
[
	{
		"name": "London",
		"timezone": "Europe/London"
	},
	{
		"name": "New York",
		"timezone": "America/New_York"
	}
]
```


## Reference

- [Bubble Tea](https://github.com/charmbracelet/bubbletea)

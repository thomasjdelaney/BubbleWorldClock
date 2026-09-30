# Bubble World Clock

A terminal world clock built with [Bubble Tea](https://github.com/charmbracelet/bubbletea). Manage a list of cities and see their local dates, times, and UTC offsets. The city picker uses a bundled offline catalog.

## Install

Installers download the latest stable release, verify its SHA-256 checksum, and install for the current user. They do not change saved city settings. Download the installer, inspect it, and run it locally; do not pipe a remote script directly into a shell.

### Linux and macOS

Requires `curl`, `tar`, and `sha256sum` or `shasum`.

```sh
curl -fL https://raw.githubusercontent.com/thomasjdelaney/BubbleWorldClock/master/scripts/install.sh -o install.sh
less install.sh
sh install.sh
```

To choose a release version or installation directory:

```sh
sh install.sh --version v1.2.3 --install-dir "$HOME/bin"
```

The default install directory is `~/.local/bin`. The installer adds it to `~/.profile` on Linux or `~/.zprofile` on macOS. Open a new terminal after installation. If you choose a custom directory, add it to PATH yourself.

### Windows

In PowerShell, download and inspect the script before running it:

```powershell
Invoke-WebRequest https://raw.githubusercontent.com/thomasjdelaney/BubbleWorldClock/master/scripts/install.ps1 -OutFile install.ps1
Get-Content .\install.ps1
.\install.ps1
```

If PowerShell blocks script execution, after inspecting the file allow scripts for this session only with `Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass`, then run `.\install.ps1` again.

To choose a release version or installation directory:

```powershell
.\install.ps1 -Version v1.2.3 -InstallDir "$env:LOCALAPPDATA\Programs\BubbleWorldClock"
```

The default directory is `%LOCALAPPDATA%\Programs\BubbleWorldClock`; the installer adds it to your user PATH. Open a new terminal after installation. Windows ARM64 is not currently supported.

Linux users can also install from the `.deb` or `.rpm` packages on the [releases page](https://github.com/thomasjdelaney/BubbleWorldClock/releases).

## Use

Run `bubble-world-clock` (or `bubble-world-clock.exe` on Windows).

- `q` or `Ctrl+C`: quit; `?`: show help; `m`: manage cities.
- In city management, `a` adds a city, `d` removes one, and `o` changes sort order. Use the arrow keys or `j`/`k` to move.
- In the city picker, type to filter, `Enter` to add a city, and `Esc` to go back.
- If saving fails, press `s` to retry.

Your city list is saved in the operating system's user config directory at `BubbleWorldClock/cities.json`. The catalog is derived from GeoNames `cities15000` under CC BY 4.0; see [data/README.md](data/README.md) for attribution.

## Develop

Install Go 1.26.6 or newer, then run:

```sh
go run .
go test ./...
go vet ./...
```

## Release (maintainers)

Push a version tag to publish platform archives, Linux `.deb` and `.rpm` packages, and checksums:

```sh
git tag -a v1.0.0 -m "v1.0.0"
git push origin v1.0.0
```

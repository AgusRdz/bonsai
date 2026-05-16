# Installation

## Requirements

- Git 2.28 or later
- A terminal with UTF-8 support (virtually all modern terminals qualify)
- macOS, Linux, or Windows

## One-line install

**macOS / Linux**

```sh
curl -fsSL https://raw.githubusercontent.com/AgusRdz/bonsai/main/install.sh | sh
```

The script downloads the latest release binary for your platform and places it in `~/.local/bin` (or `/usr/local/bin` if that is writable). It adds the directory to your `$PATH` if needed.

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/AgusRdz/bonsai/main/install.ps1 | iex
```

## Manual install

1. Download the binary for your platform from the [releases page](https://github.com/AgusRdz/bonsai/releases).
2. Make it executable: `chmod +x bonsai`
3. Move it somewhere on your `$PATH`: `mv bonsai ~/.local/bin/`

## Build from source

Requires Go 1.21 or later.

```sh
git clone https://github.com/AgusRdz/bonsai.git
cd bonsai
go build -ldflags="-s -w" -o bonsai .
mv bonsai ~/.local/bin/
```

## Update

```sh
bonsai update
```

Checks GitHub releases and replaces the current binary in-place.

## Uninstall

```sh
bonsai uninstall
```

Removes the binary. You can also delete the global config manually:

```sh
rm -rf ~/.config/bonsai/
```

## First run

The first time you run `bonsai` inside a repo it will automatically open the setup wizard if no global config exists. You can also run it explicitly at any time:

```sh
bonsai setup
```

# lazychezmoi

A terminal UI for [chezmoi](https://www.chezmoi.io/) inspired by lazygit/gitui.

## Features

- Browse changed dotfiles with a split-pane TUI
- Preview diffs inline (colorized unified diff)
- Apply individual files with confirmation
- Open source files in your `$EDITOR`
- Refresh the file list at any time

## Installation

```sh
go install lazychezmoi/cmd/lazychezmoi@latest
```

Or download a release binary from the [Releases](../../releases) page.

## Usage

```sh
lazychezmoi [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--source` | auto | chezmoi source directory |
| `--destination` | `$HOME` | chezmoi destination directory |
| `--exclude` | | Comma-separated types to exclude (e.g. `templates,scripts`) |
| `--chezmoi-bin` | `chezmoi` | Path to chezmoi binary |

### Keybindings

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `tab` | Switch pane (src ↔ target) |
| `shift+tab` | Switch pane (reverse) |
| `a` | Apply selected file (target pane only) |
| `e` | Open source file in `$EDITOR` |
| `r` | Refresh file list |
| `?` | Toggle help |
| `q` / `ctrl+c` | Quit |

## Requirements

- Go 1.22+
- chezmoi installed and configured

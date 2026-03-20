# lazychezmoi

A terminal UI for [chezmoi](https://www.chezmoi.io/) inspired by lazygit/gitui.

## Features

- Browse changed dotfiles with a split-pane TUI
- Preview diffs inline with background caching and refresh
- Apply one or many target files with confirmation
- Open the focused source or target file in your `$EDITOR`
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
| `tab` | Switch pane (`target` → `src` → diff) |
| `shift+tab` | Switch pane (reverse) |
| `space` | Toggle the current target in the apply queue |
| `a` | Apply queued targets, or the current target if nothing is queued |
| `e` | Open the focused `src` or `target` file in `$EDITOR` |
| `pgup` / `pgdn` / `g` / `G` | Scroll the focused diff |
| `r` | Refresh file list |
| `?` | Toggle help |
| `q` / `ctrl+c` | Quit |

## Requirements

- Go 1.22+
- chezmoi installed and configured

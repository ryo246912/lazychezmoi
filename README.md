# lazychezmoi

A terminal UI for [chezmoi](https://www.chezmoi.io/) inspired by lazygit/gitui.

## Features

- Browse changed dotfiles with a split-pane, tree-based TUI
- Toggle between managed changes and unmanaged target-only entries
- Filter the file tree with `/`, click or wheel-scroll to focus a row, and expand directories with `Enter`
- Preview diffs inline with debounced refresh and background caching
- Apply one or many target files from `working tree`, `staged`, or `HEAD`
- Run `chezmoi add` for unmanaged targets or import target-side changes from managed targets back into source state
- Delete unmanaged targets from the target tree
- Open a lazygit-style `:` shell command prompt with persistent history and `LAZYCHEZMOI_*` environment variables
- Open the focused source or target file in your `$EDITOR`
- Show loading spinners for apply, mode switches, and snapshot preparation
- Refresh the file list at any time

## Installation

```sh
go install github.com/ryo246912/lazychezmoi/tools/lazychezmoi/cmd/lazychezmoi@latest
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

### Modes

- `managed`: files already tracked by chezmoi where the target tree currently differs from source state
- `all`: managed diffs plus target-only paths that are not yet tracked in chezmoi source state

### Keybindings

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `enter` | Expand or collapse the selected directory |
| `/` | Start filter input for the file tree |
| `h` / `l` | Focus `src` / `target` pane |
| `tab` | Toggle diff focus |
| `m` | Toggle `managed` / `all` mode |
| `1` / `2` / `3` | Select apply source: `working tree` / `staged` / `HEAD` |
| `space` | Toggle the current target in the apply queue (`managed` mode) |
| `a` | Apply queued targets, or the current target if nothing is queued (`managed` mode) |
| `i` | Run `chezmoi add` for the selected target: update source from target (`managed`) or start tracking the target (`all`) |
| `d` | Delete the current unmanaged target after confirmation (`all` mode) |
| `:` | Open the shell command prompt for the selected entry |
| `e` | Open the focused `src` or `target` file in `$EDITOR` |
| Mouse click / wheel | Focus the clicked pane; clicking or scrolling a `src` / `target` row also selects it |
| `pgup` / `pgdn` / `g` / `G` | Scroll the focused diff |
| `r` | Refresh file list |
| `?` | Toggle help |
| `q` / `ctrl+c` | Quit |

### Shell Command Context

Custom shell commands receive these environment variables:

- `LAZYCHEZMOI_TARGET_PATH`
- `LAZYCHEZMOI_SOURCE_PATH`
- `LAZYCHEZMOI_ENTRY_MODE`
- `LAZYCHEZMOI_TARGET_KIND`
- `LAZYCHEZMOI_APPLY_SOURCE`
- `LAZYCHEZMOI_LIST_MODE`

Inside the shell command prompt:

- `enter`: run the current command immediately
- `down` / `up`: browse command history
- `esc`: close the prompt without running a command

## Requirements

- Go 1.22+
- chezmoi installed and configured

## Release Flow

This project uses [tagpr](https://github.com/Songmu/tagpr) for automated releases.

1. When changes are pushed to the `main` branch, tagpr automatically creates or updates a pull request for the next release.
2. The pull request includes a version bump and a changelog update.
3. Merging the pull request will:
    - Create a git tag for the new version.
    - Create a GitHub Release.
    - Trigger the `release-lazychezmoi` workflow to build and publish artifacts.

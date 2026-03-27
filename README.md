# circleci-watch

A live-updating terminal UI for monitoring CircleCI pipelines — like `k9s` but for CI.

![status: running]

## Features

- Auto-detects project from `git remote origin`
- Shows the last 10 pipelines across all branches
- Auto-expands running and failed pipelines to show jobs
- Running jobs show the currently-executing step
- Failed jobs show the error output inline, word-wrapped to your terminal
- Polls every 5 seconds and updates in place

## Install

```bash
git clone <this repo>
cd circleci-watch
go build -o circleci-watch .
```

Or build and move to your PATH:
```bash
go build -o /usr/local/bin/circleci-watch .
```

## Usage

```bash
export CIRCLECI_TOKEN=your_token_here
circleci-watch
```

Run from any directory with a `git remote origin` pointing to GitHub or Bitbucket — the project is detected automatically.

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--project` | auto-detect | CircleCI project slug, e.g. `gh/myorg/myrepo` |
| `--branch` | all branches | Filter to a specific branch |
| `--refresh` | `5s` | Polling interval |
| `--limit` | `10` | Number of recent pipelines to show |
| `--token` | `$CIRCLECI_TOKEN` | API token |
| `--debug` | off | Write raw API responses to a file, e.g. `/tmp/cw.log` |

### Keybindings

| Key | Action |
|-----|--------|
| `q` / `ctrl+c` | Quit |
| `r` | Force refresh |
| `↑` / `k` | Scroll up |
| `↓` / `j` | Scroll down |
| `PgUp` / `PgDn` | Scroll by page |

## Auth

Create a CircleCI personal API token at **User Settings → Personal API Tokens** and set it as `CIRCLECI_TOKEN`.

## How it works

Uses the CircleCI v2 API for pipelines, workflows, and jobs. Falls back to the v1.1 API to retrieve step output when the v2 job details response omits `output_url` (which it does for most jobs).

# circleci-watch — Copilot Instructions

## What this is

A Go CLI that renders a live-updating TUI for CircleCI pipelines. Think `k9s` for CI.

## Stack

- **Language**: Go 1.26
- **TUI**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) (Elm-style model/update/view)
- **Styling**: [Lip Gloss](https://github.com/charmbracelet/lipgloss)
- **CLI flags**: [Cobra](https://github.com/spf13/cobra)
- **API**: CircleCI v2 (primary) + v1.1 (fallback for step output)

## Project layout

```
cmd/root.go           CLI entrypoint — flags, wires everything together
internal/api/client.go  CircleCI HTTP client (v2 + v1.1), debug logging, ANSI stripping
internal/git/detect.go  Parses git remote URL → CircleCI project slug
internal/ui/model.go    Bubble Tea model: state, Update(), polling logic
internal/ui/view.go     View() renderer — pipeline tree
internal/ui/styles.go   Lip Gloss style definitions and status icons
internal/ui/helpers.go  calcJobDuration, wordWrap
main.go               Calls cmd.Execute()
```

## Key behaviours

- **Polling**: pipelines fetched every `--refresh` (default 5s). Active pipelines re-fetch workflows + jobs every cycle so status stays live.
- **Expansion**: pipelines auto-expand when any workflow is `running`/`failing`/`failed`. Succeeded pipelines stay collapsed.
- **Step output**: two-phase fetch — (1) job details for step names + `output_url`, (2) GET `output_url` for log text. v2 often returns empty steps, so v1.1 is tried as fallback when no `output_url` is found.
- **ANSI stripping**: CircleCI log output contains terminal escape codes; these are stripped before display.
- **Word wrap**: failure output is wrapped to terminal width at word boundaries.

## Adding features

- New API calls go in `internal/api/client.go` — use `c.get(url, &out)` for authed v2 calls, `c.getWithAuth(url, false, &out)` for pre-signed URLs
- New UI state goes on `pipelineState` / `workflowState` / `jobState` structs in `model.go`
- New messages follow the `tea.Msg` pattern: define a type, return a `tea.Cmd` from a method on `Model`, handle in `Update()`
- Styles: add to `styles.go`, use in `view.go`

## Debugging

```bash
./circleci-watch --debug /tmp/cw.log
tail -f /tmp/cw.log
```

All API requests (URL, status, full response body) are written to the log file.

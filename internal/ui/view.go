package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.width == 0 {
		return "Initialising…"
	}

	header := m.renderHeader()
	statusBar := m.renderStatusBar()

	contentHeight := m.height - lipgloss.Height(header) - lipgloss.Height(statusBar)
	if contentHeight < 1 {
		contentHeight = 1
	}

	content := m.renderContent(contentHeight)

	return header + "\n" + content + "\n" + statusBar
}

// renderHeader renders the top bar.
func (m Model) renderHeader() string {
	left := fmt.Sprintf(" circleci-watch  ·  %s", m.projectSlug)
	right := ""
	if !m.lastRefresh.IsZero() {
		right = fmt.Sprintf("↻ %s ago  ", humanDuration(time.Since(m.lastRefresh)))
	}
	if m.loading {
		right = "fetching…  "
	}

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	line := left + strings.Repeat(" ", gap) + right
	return styleHeader.Width(m.width).Render(line)
}

// renderStatusBar renders the bottom keybindings bar.
func (m Model) renderStatusBar() string {
	keys := "  q quit  ·  r refresh  ·  ↑↓/jk scroll  ·  PgUp/PgDn page"
	if m.err != nil {
		keys = styleFailed.Render(fmt.Sprintf("  ✖ error: %s", m.err))
	}
	return styleStatusBar.Width(m.width).Render(keys)
}

// renderContent renders the scrollable pipeline tree.
func (m Model) renderContent(height int) string {
	var lines []string

	if len(m.pipelines) == 0 && !m.loading {
		lines = append(lines, styleGrey.Render("  No pipelines found."))
	}

	for _, ps := range m.pipelines {
		lines = append(lines, m.renderPipeline(ps)...)
	}

	// clamp scroll
	maxOffset := len(lines) - height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset > maxOffset {
		// We can't mutate m here, just clamp for display
	}
	offset := m.offset
	if offset > maxOffset {
		offset = maxOffset
	}

	// slice window
	visible := lines
	if offset < len(lines) {
		visible = lines[offset:]
	}
	if len(visible) > height {
		visible = visible[:height]
	}

	// pad to fill height
	for len(visible) < height {
		visible = append(visible, "")
	}

	return strings.Join(visible, "\n")
}

func (m Model) renderPipeline(ps pipelineState) []string {
	var lines []string

	status := pipelineOverallStatus(ps)
	icon := statusIcon(status)
	st := statusStyle(status)

	branch := ps.pipeline.VCS.Branch
	if branch == "" {
		branch = ps.pipeline.VCS.Tag
	}
	if branch == "" {
		branch = "unknown"
	}

	age := humanDuration(time.Since(ps.pipeline.CreatedAt))
	trigger := ps.pipeline.Trigger.Type
	actor := ps.pipeline.Trigger.Actor.Login

	pipelineLabel := fmt.Sprintf(
		"%s  Pipeline #%d  %s  %s  %s",
		icon,
		ps.pipeline.Number,
		styleBlue.Render(branch),
		styleGrey.Render(age+" ago"),
		styleGrey.Render("("+trigger+")"),
	)
	if actor != "" {
		pipelineLabel += styleGrey.Render(" by "+actor)
	}
	statusLabel := st.Render("[" + status + "]")
	gap := m.width - lipgloss.Width(pipelineLabel) - lipgloss.Width(statusLabel) - 2
	if gap < 1 {
		gap = 1
	}
	row := stylePipelineRow.Width(m.width).Render(
		pipelineLabel + strings.Repeat(" ", gap) + statusLabel,
	)
	lines = append(lines, row)

	if !ps.expanded {
		return lines
	}

	if ps.loading {
		lines = append(lines, styleGrey.Render("    loading workflows…"))
		return lines
	}

	for _, wf := range ps.workflows {
		lines = append(lines, m.renderWorkflow(wf)...)
	}

	lines = append(lines, "") // spacing between pipelines
	return lines
}

func (m Model) renderWorkflow(wf workflowState) []string {
	var lines []string

	icon := statusIcon(wf.workflow.Status)
	st := statusStyle(wf.workflow.Status)

	dur := ""
	if !wf.workflow.StoppedAt.IsZero() {
		dur = " · " + humanDuration(wf.workflow.StoppedAt.Sub(wf.workflow.CreatedAt))
	}

	wfLine := fmt.Sprintf("    %s  %s  %s%s",
		icon,
		styleWhite.Render(wf.workflow.Name),
		st.Render(wf.workflow.Status),
		styleGrey.Render(dur),
	)
	lines = append(lines, wfLine)

	if wf.loading {
		lines = append(lines, styleGrey.Render("        loading jobs…"))
		return lines
	}

	for _, js := range wf.jobs {
		lines = append(lines, m.renderJob(js)...)
	}

	return lines
}

func (m Model) renderJob(js jobState) []string {
	var lines []string

	icon := statusIcon(js.job.Status)
	st := statusStyle(js.job.Status)

	dur := calcJobDuration(js.job)
	jobLine := fmt.Sprintf("        %s  %-40s  %s  %s",
		icon,
		styleWhite.Render(js.job.Name),
		st.Render(js.job.Status),
		styleGrey.Render(dur),
	)
	lines = append(lines, jobLine)

	// Render step details for running and failed jobs
	isFailed := js.job.Status == "failed" || js.job.Status == "infrastructure_fail" || js.job.Status == "timedout"
	isRunning := js.job.Status == "running"

	if isRunning {
		if js.stepsLoading && len(js.steps) == 0 {
			lines = append(lines, styleGrey.Render("            fetching steps…"))
		} else if len(js.steps) > 0 {
			if step := runningStepName(js.steps); step != "" {
				lines = append(lines, styleRunning.Render("            ▶ "+step))
			}
		}
	} else if isFailed {
		if js.outputLoading || (js.stepsFetched && !js.outputFetched) {
			lines = append(lines, styleGrey.Render("            fetching failure details…"))
		} else if js.stepsFetched {
			stepLabel := js.failedStepName
			if stepLabel == "" {
				stepLabel = "unknown step"
			}
			lines = append(lines, styleFailed.Render("            ╭─ "+stepLabel))
			const indent = "            │ "
			const indentWidth = 14 // visual width of indent prefix
			wrapWidth := m.width - indentWidth
			if wrapWidth < 40 {
				wrapWidth = 40
			}
			var outputText string
			if js.failureOutput != "" {
				outputText = truncateOutput(js.failureOutput, 15)
			} else {
				outputText = "(no output — infrastructure failure or job was cancelled)"
			}
			for _, l := range strings.Split(outputText, "\n") {
				for _, wrapped := range wordWrap(l, wrapWidth) {
					lines = append(lines, styleFailureBlock.Render(indent+wrapped))
				}
			}
		}
	}

	return lines
}

// --- Time helpers ---

func humanDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		if s == 0 {
			return fmt.Sprintf("%dm", m)
		}
		return fmt.Sprintf("%dm%ds", m, s)
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", h, m)
}



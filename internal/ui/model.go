package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/damien/circleci-watch/internal/api"
	"github.com/damien/circleci-watch/internal/notify"
)

// --- Messages ---

type tickMsg time.Time
type pipelinesMsg struct {
	pipelines []api.Pipeline
	err       error
}
type workflowsMsg struct {
	pipelineID string
	workflows  []api.Workflow
	err        error
}
type jobsMsg struct {
	workflowID string
	jobs       []api.Job
	err        error
}
type stepsMsg struct {
	jobKey string
	steps  []api.Step
	err    error
}
type stepOutputMsg struct {
	jobKey string
	output string
	err    error
}

// --- State ---

type pipelineState struct {
	pipeline       api.Pipeline
	workflows      []workflowState
	loading        bool
	expanded       bool
	notifiedStatus string // last status we fired a desktop notification for
}

type workflowState struct {
	workflow api.Workflow
	jobs     []jobState
	loading  bool
}

type jobState struct {
	job            api.Job
	steps          []api.Step
	stepsLoading   bool
	stepsFetched   bool
	failedStepName string
	failureOutput  string
	outputFetched  bool
	outputLoading  bool
}

// --- Model ---

type Model struct {
	client      *api.Client
	projectSlug string
	branch      string
	limit       int
	refresh     time.Duration
	notify      bool

	pipelines   []pipelineState
	lastRefresh time.Time
	err         error
	loading     bool

	width  int
	height int
	offset int

	fetchingWorkflows map[string]bool
	fetchingJobs      map[string]bool
	fetchingSteps     map[string]bool
	fetchingOutput    map[string]bool
}

func NewModel(client *api.Client, projectSlug, branch string, limit int, refresh time.Duration, notify bool) Model {
	return Model{
		client:            client,
		projectSlug:       projectSlug,
		branch:            branch,
		limit:             limit,
		refresh:           refresh,
		notify:            notify,
		loading:           true,
		fetchingWorkflows: make(map[string]bool),
		fetchingJobs:      make(map[string]bool),
		fetchingSteps:     make(map[string]bool),
		fetchingOutput:    make(map[string]bool),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.fetchPipelines(),
		tick(m.refresh),
	)
}

// --- Commands ---

func tick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) fetchPipelines() tea.Cmd {
	return func() tea.Msg {
		pipelines, err := m.client.GetPipelines(m.projectSlug, m.branch, m.limit)
		return pipelinesMsg{pipelines: pipelines, err: err}
	}
}

func (m Model) fetchWorkflows(pipelineID string) tea.Cmd {
	return func() tea.Msg {
		workflows, err := m.client.GetWorkflows(pipelineID)
		return workflowsMsg{pipelineID: pipelineID, workflows: workflows, err: err}
	}
}

func (m Model) fetchJobs(workflowID string) tea.Cmd {
	return func() tea.Msg {
		jobs, err := m.client.GetJobs(workflowID)
		return jobsMsg{workflowID: workflowID, jobs: jobs, err: err}
	}
}

func (m Model) fetchSteps(jobKey string, jobNumber int) tea.Cmd {
	return func() tea.Msg {
		// Try v2 first; fall back to v1.1 if v2 returns no output_url on any action
		steps, err := m.client.GetJobSteps(m.projectSlug, jobNumber)
		if err == nil && !anyOutputURL(steps) {
			if v1Steps, v1Err := m.client.GetJobStepsV1(m.projectSlug, jobNumber); v1Err == nil {
				steps = v1Steps
			}
		}
		return stepsMsg{jobKey: jobKey, steps: steps, err: err}
	}
}

func (m Model) fetchStepOutput(jobKey, outputURL string) tea.Cmd {
	return func() tea.Msg {
		output, err := m.client.GetStepOutput(outputURL)
		return stepOutputMsg{jobKey: jobKey, output: output, err: err}
	}
}

// --- Update ---

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			m.loading = true
			return m, m.fetchPipelines()
		case "up", "k":
			if m.offset > 0 {
				m.offset--
			}
		case "down", "j":
			m.offset++
		case "pgup":
			m.offset -= m.contentHeight()
			if m.offset < 0 {
				m.offset = 0
			}
		case "pgdown":
			m.offset += m.contentHeight()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		return m, tea.Batch(m.fetchPipelines(), tick(m.refresh))

	case pipelinesMsg:
		m.loading = false
		m.lastRefresh = time.Now()
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.mergePipelines(msg.pipelines)
		return m, m.kickOffFetches()

	case workflowsMsg:
		if msg.err == nil {
			m.mergeWorkflows(msg.pipelineID, msg.workflows)
		}
		delete(m.fetchingWorkflows, msg.pipelineID)
		notifyCmds := m.checkNotifications(msg.pipelineID)
		return m, tea.Batch(append(notifyCmds, m.kickOffFetches())...)

	case jobsMsg:
		if msg.err == nil {
			m.mergeJobs(msg.workflowID, msg.jobs)
		}
		delete(m.fetchingJobs, msg.workflowID)
		return m, m.kickOffFetches()

	case stepsMsg:
		if msg.err == nil {
			m.mergeSteps(msg.jobKey, msg.steps)
		}
		delete(m.fetchingSteps, msg.jobKey)
		return m, m.kickOffFetches()

	case stepOutputMsg:
		if msg.err == nil {
			m.mergeStepOutput(msg.jobKey, msg.output)
		} else {
			m.mergeStepOutput(msg.jobKey, fmt.Sprintf("(error fetching output: %v)", msg.err))
		}
		delete(m.fetchingOutput, msg.jobKey)
	}

	return m, nil
}

func (m Model) contentHeight() int {
	if m.height > 4 {
		return m.height - 4
	}
	return 1
}

// --- Merge helpers ---

func (m *Model) mergePipelines(fresh []api.Pipeline) {
	existing := make(map[string]*pipelineState, len(m.pipelines))
	for i := range m.pipelines {
		existing[m.pipelines[i].pipeline.ID] = &m.pipelines[i]
	}

	next := make([]pipelineState, 0, len(fresh))
	for _, p := range fresh {
		if ex, ok := existing[p.ID]; ok {
			ex.pipeline = p
			// re-expand running/failed pipelines automatically
			if isActiveStatus(p.State) {
				ex.expanded = true
			}
			next = append(next, *ex)
		} else {
			expanded := isActiveStatus(p.State)
			next = append(next, pipelineState{
				pipeline: p,
				loading:  true,
				expanded: expanded,
			})
		}
	}
	m.pipelines = next
}

func (m *Model) mergeWorkflows(pipelineID string, workflows []api.Workflow) {
	for i := range m.pipelines {
		if m.pipelines[i].pipeline.ID != pipelineID {
			continue
		}
		ps := &m.pipelines[i]
		ps.loading = false

		existing := make(map[string]workflowState)
		for _, w := range ps.workflows {
			existing[w.workflow.ID] = w
		}

		next := make([]workflowState, 0, len(workflows))
		for _, wf := range workflows {
			if ex, ok := existing[wf.ID]; ok {
				ex.workflow = wf
				next = append(next, ex)
			} else {
				next = append(next, workflowState{workflow: wf, loading: true})
			}
		}
		ps.workflows = next

		// Auto-expand pipeline based on workflow statuses (pipeline.State is always "created";
		// the real active state is only visible once workflows are loaded).
		for _, wf := range ps.workflows {
			if isActiveWorkflowStatus(wf.workflow.Status) {
				ps.expanded = true
				break
			}
		}
		return
	}
}

func (m *Model) mergeJobs(workflowID string, jobs []api.Job) {
	for i := range m.pipelines {
		for j := range m.pipelines[i].workflows {
			wf := &m.pipelines[i].workflows[j]
			if wf.workflow.ID != workflowID {
				continue
			}
			wf.loading = false

			existing := make(map[string]jobState)
			for _, js := range wf.jobs {
				existing[js.job.ID] = js
			}

			next := make([]jobState, 0, len(jobs))
			for _, job := range jobs {
				if ex, ok := existing[job.ID]; ok {
					prevStatus := ex.job.Status
					ex.job = job
					// Running job: invalidate cached steps so we re-fetch the current step
					if job.Status == "running" && prevStatus == "running" {
						ex.stepsFetched = false
					}
					next = append(next, ex)
				} else {
					next = append(next, jobState{job: job})
				}
			}
			wf.jobs = next
			return
		}
	}
}

func (m *Model) mergeSteps(jobKey string, steps []api.Step) {
	for i := range m.pipelines {
		for j := range m.pipelines[i].workflows {
			for k := range m.pipelines[i].workflows[j].jobs {
				js := &m.pipelines[i].workflows[j].jobs[k]
				if jobStateKey(js.job) == jobKey {
					js.steps = steps
					js.stepsLoading = false
					if js.job.Status != "running" {
						js.stepsFetched = true
						js.failedStepName, _ = failedStepInfo(steps)
					}
					return
				}
			}
		}
	}
}

func (m *Model) mergeStepOutput(jobKey, output string) {
	for i := range m.pipelines {
		for j := range m.pipelines[i].workflows {
			for k := range m.pipelines[i].workflows[j].jobs {
				js := &m.pipelines[i].workflows[j].jobs[k]
				if jobStateKey(js.job) == jobKey {
					js.failureOutput = output
					js.outputFetched = true
					js.outputLoading = false
					return
				}
			}
		}
	}
}

// checkNotifications inspects the pipeline with the given ID and fires a desktop
// notification if it just transitioned to a terminal status we haven't notified for yet.
func (m *Model) checkNotifications(pipelineID string) []tea.Cmd {
	if !m.notify {
		return nil
	}
	for i := range m.pipelines {
		ps := &m.pipelines[i]
		if ps.pipeline.ID != pipelineID {
			continue
		}
		status := pipelineOverallStatus(*ps)
		if !isTerminalOverallStatus(status) || status == ps.notifiedStatus {
			return nil
		}
		ps.notifiedStatus = status

		num := ps.pipeline.Number
		branch := ps.pipeline.VCS.Branch
		if branch == "" {
			branch = ps.pipeline.VCS.Tag
		}

		var title, body string
		switch status {
		case "success":
			title = "✔ Pipeline succeeded"
			body = fmt.Sprintf("#%d %s", num, branch)
		default:
			title = "✖ Pipeline failed"
			body = fmt.Sprintf("#%d %s — %s", num, branch, status)
		}

		return []tea.Cmd{func() tea.Msg {
			notify.Send(title, body)
			return nil
		}}
	}
	return nil
}

func (m *Model) kickOffFetches() tea.Cmd {
	var cmds []tea.Cmd

	for i := range m.pipelines {
		ps := &m.pipelines[i]

		// Re-fetch workflows on initial load and for any non-terminal status.
		// Only skip re-fetching when we have a definitive final status.
		overallStatus := pipelineOverallStatus(*ps)
		needsWorkflowRefresh := ps.loading || !isTerminalOverallStatus(overallStatus)
		if needsWorkflowRefresh && !m.fetchingWorkflows[ps.pipeline.ID] {
			m.fetchingWorkflows[ps.pipeline.ID] = true
			cmds = append(cmds, m.fetchWorkflows(ps.pipeline.ID))
		}

		if !ps.expanded {
			continue
		}

		for j := range ps.workflows {
			wf := &ps.workflows[j]

			// Re-fetch jobs on initial load and every cycle while workflow is active
			needsJobRefresh := wf.loading || wf.workflow.Status == "running" || wf.workflow.Status == "failing"
			if needsJobRefresh && !m.fetchingJobs[wf.workflow.ID] {
				m.fetchingJobs[wf.workflow.ID] = true
				cmds = append(cmds, m.fetchJobs(wf.workflow.ID))
			}

			for k := range wf.jobs {
				js := &wf.jobs[k]
				key := jobStateKey(js.job)

				isFailed := js.job.Status == "failed" || js.job.Status == "infrastructure_fail" || js.job.Status == "timedout"
				isRunning := js.job.Status == "running"

				// Phase 1: fetch steps for running jobs (every cycle) and failed jobs (once)
				needsSteps := isRunning || (isFailed && !js.stepsFetched)
				if needsSteps && !js.stepsLoading && !m.fetchingSteps[key] && js.job.JobNumber != nil {
					m.fetchingSteps[key] = true
					js.stepsLoading = true
					cmds = append(cmds, m.fetchSteps(key, *js.job.JobNumber))
				}

				// Phase 2: once steps are loaded for a failed job, fetch the log output URL (once)
				if isFailed && js.stepsFetched && !js.outputFetched && !js.outputLoading && !m.fetchingOutput[key] {
					if url := failedActionOutputURL(js.steps); url != "" {
						m.fetchingOutput[key] = true
						js.outputLoading = true
						cmds = append(cmds, m.fetchStepOutput(key, url))
					} else {
						js.outputFetched = true // no output URL, nothing to fetch
					}
				}
			}
		}
	}

	return tea.Batch(cmds...)
}

func isActiveStatus(state string) bool {
	switch state {
	case "running", "failing", "failed", "error", "errored":
		return true
	}
	return false
}

func isActiveWorkflowStatus(status string) bool {
	switch status {
	case "running", "failing", "failed", "error":
		return true
	}
	return false
}

// isTerminalOverallStatus returns true when a pipeline has reached a definitive
// end state and its workflows no longer need polling.
func isTerminalOverallStatus(status string) bool {
	switch status {
	case "success", "failed", "error", "canceled", "unauthorized":
		return true
	}
	return false
}

func jobStateKey(j api.Job) string {
	if j.JobNumber != nil {
		return fmt.Sprintf("%s-%d", j.ID, *j.JobNumber)
	}
	return j.ID
}

// pipelineOverallStatus derives a display status from workflows.
func pipelineOverallStatus(ps pipelineState) string {
	if ps.loading {
		return "loading"
	}
	if len(ps.workflows) == 0 {
		return ps.pipeline.State
	}
	// Worst-status wins
	priority := map[string]int{
		"failed": 5, "error": 5, "infrastructure_fail": 5, "timedout": 5,
		"failing": 4,
		"running": 3,
		"on_hold": 2,
		"success": 1,
		"not_run": 0,
	}
	best := ""
	bestP := -1
	for _, wf := range ps.workflows {
		if p, ok := priority[wf.workflow.Status]; ok && p > bestP {
			bestP = p
			best = wf.workflow.Status
		}
	}
	if best == "" {
		return ps.pipeline.State
	}
	return best
}

// runningStepName returns the name of the currently executing step/action.
func runningStepName(steps []api.Step) string {
	for _, step := range steps {
		for _, action := range step.Actions {
			if action.Status == "running" {
				return action.Name
			}
		}
	}
	// Fall back to the last step that has started
	for i := len(steps) - 1; i >= 0; i-- {
		for j := len(steps[i].Actions) - 1; j >= 0; j-- {
			a := steps[i].Actions[j]
			if a.Status != "not_run" {
				return a.Name
			}
		}
	}
	return ""
}

// failedStepInfo returns the name and output_url of the action responsible for the failure.
// Uses multiple passes from specific to broad so we always find something when output exists.
func failedStepInfo(steps []api.Step) (name string, outputURL string) {
	// Pass 1: explicit failure status with an output URL
	for _, step := range steps {
		for _, action := range step.Actions {
			if isFailedActionStatus(action.Status) && action.OutputURL != "" {
				return action.Name, action.OutputURL
			}
		}
	}
	// Pass 2: non-zero exit code with an output URL
	for _, step := range steps {
		for _, action := range step.Actions {
			if action.ExitCode != nil && *action.ExitCode != 0 && action.OutputURL != "" {
				return action.Name, action.OutputURL
			}
		}
	}
	// Pass 3: explicit failure status, no output URL (at least get the step name)
	for _, step := range steps {
		for _, action := range step.Actions {
			if isFailedActionStatus(action.Status) {
				return action.Name, action.OutputURL
			}
		}
	}
	// Pass 4: has_output=true on any non-success action — catches non-standard statuses
	for i := len(steps) - 1; i >= 0; i-- {
		for j := len(steps[i].Actions) - 1; j >= 0; j-- {
			a := steps[i].Actions[j]
			if a.HasOutput && a.OutputURL != "" && a.Status != "success" {
				return a.Name, a.OutputURL
			}
		}
	}
	// Pass 5: last action with any output URL — last resort
	for i := len(steps) - 1; i >= 0; i-- {
		for j := len(steps[i].Actions) - 1; j >= 0; j-- {
			a := steps[i].Actions[j]
			if a.OutputURL != "" {
				return a.Name, a.OutputURL
			}
		}
	}
	return "", ""
}

func isFailedActionStatus(s string) bool {
	switch s {
	case "failed", "infrastructure_fail", "timedout", "error":
		return true
	}
	return false
}

// anyOutputURL returns true if any action in the steps has a non-empty output_url.
func anyOutputURL(steps []api.Step) bool {
	for _, step := range steps {
		for _, action := range step.Actions {
			if action.OutputURL != "" {
				return true
			}
		}
	}
	return false
}

// failedActionOutputURL returns just the output_url for use in kickOffFetches.
func failedActionOutputURL(steps []api.Step) string {
	_, url := failedStepInfo(steps)
	return url
}

func truncateOutput(s string, maxLines int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > maxLines {
		lines = append([]string{fmt.Sprintf("... (%d lines omitted)", len(lines)-maxLines)}, lines[len(lines)-maxLines:]...)
	}
	return strings.Join(lines, "\n")
}

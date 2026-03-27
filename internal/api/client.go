package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	baseURL   = "https://circleci.com/api/v2"
	baseURLV1 = "https://circleci.com/api/v1.1"
)

type Client struct {
	token      string
	httpClient *http.Client
	debugLog   *log.Logger
}

func NewClient(token string, debugLogPath string) *Client {
	c := &Client{
		token: token,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
	if debugLogPath != "" {
		f, err := os.OpenFile(debugLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err == nil {
			c.debugLog = log.New(f, "", log.Ltime)
		}
	}
	return c
}

func (c *Client) get(url string, out any) error {
	return c.getWithAuth(url, true, out)
}

func (c *Client) getWithAuth(url string, addAuth bool, out any) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	if addAuth {
		req.Header.Set("Circle-Token", c.token)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if c.debugLog != nil {
		c.debugLog.Printf("GET %s  status=%d\n%s\n", url, resp.StatusCode, string(body))
	}

	if resp.StatusCode == 401 {
		return fmt.Errorf("unauthorized: check your CIRCLECI_TOKEN")
	}
	if resp.StatusCode == 404 {
		return fmt.Errorf("not found (404): %s", url)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return json.NewDecoder(bytes.NewReader(body)).Decode(out)
}

// --- Types ---

type Pipeline struct {
	ID          string    `json:"id"`
	Number      int       `json:"number"`
	State       string    `json:"state"` // created, errored, setup-pending, setup, pending
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Trigger     Trigger   `json:"trigger"`
	VCS         VCS       `json:"vcs"`
}

type Trigger struct {
	Type       string    `json:"type"`
	ReceivedAt time.Time `json:"received_at"`
	Actor      Actor     `json:"actor"`
}

type Actor struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

type VCS struct {
	Branch        string `json:"branch"`
	Tag           string `json:"tag"`
	OriginRepoURL string `json:"origin_repository_url"`
}

type Workflow struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"` // success, running, not_run, failed, error, failing, on_hold, canceled, unauthorized
	CreatedAt  time.Time `json:"created_at"`
	StoppedAt  time.Time `json:"stopped_at"`
	PipelineID string    `json:"pipeline_id"`
}

type Job struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Status      string     `json:"status"` // success, running, not_run, failed, error, queued, not_running, infrastructure_fail, timedout, on_hold, terminated-unknown, blocked, canceled, unauthorized
	StartedAt   *time.Time `json:"started_at"`
	StoppedAt   *time.Time `json:"stopped_at"`
	JobNumber   *int       `json:"job_number"`
	Type        string     `json:"type"`
	Dependencies []string  `json:"dependencies"`
}

type Step struct {
	Name    string   `json:"name"`
	Actions []Action `json:"actions"`
}

type Action struct {
	Name      string  `json:"name"`
	Status    string  `json:"status"`
	ExitCode  *int    `json:"exit_code"`
	Output    *string `json:"output"`
	OutputURL string  `json:"output_url"`
	HasOutput bool    `json:"has_output"`
	RunTimeMS int     `json:"run_time_millis"`
}

type paginatedResponse[T any] struct {
	Items         []T    `json:"items"`
	NextPageToken string `json:"next_page_token"`
}

// --- API Methods ---

func (c *Client) GetPipelines(projectSlug string, branch string, limit int) ([]Pipeline, error) {
	u := fmt.Sprintf("%s/project/%s/pipeline", baseURL, projectSlug)
	if branch != "" {
		u += "?branch=" + branch
	}

	var result paginatedResponse[Pipeline]
	if err := c.get(u, &result); err != nil {
		return nil, err
	}

	if limit > 0 && len(result.Items) > limit {
		return result.Items[:limit], nil
	}
	return result.Items, nil
}

func (c *Client) GetWorkflows(pipelineID string) ([]Workflow, error) {
	var result paginatedResponse[Workflow]
	if err := c.get(fmt.Sprintf("%s/pipeline/%s/workflow", baseURL, pipelineID), &result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (c *Client) GetJobs(workflowID string) ([]Job, error) {
	var result paginatedResponse[Job]
	if err := c.get(fmt.Sprintf("%s/workflow/%s/job", baseURL, workflowID), &result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

// GetJobSteps fetches step details (with output_url per action) via the v2 API.
func (c *Client) GetJobSteps(projectSlug string, jobNumber int) ([]Step, error) {
	type jobDetail struct {
		Steps []Step `json:"steps"`
	}
	var detail jobDetail
	u := fmt.Sprintf("%s/project/%s/job/%d", baseURL, projectSlug, jobNumber)
	if err := c.get(u, &detail); err != nil {
		return nil, err
	}
	return detail.Steps, nil
}

type logChunk struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// GetStepOutput fetches actual log text from an action's output_url.
// The output_url is a pre-signed URL (no auth required) returning JSON log chunks.
// ANSI escape codes are stripped so the text renders cleanly in the TUI.
func (c *Client) GetStepOutput(outputURL string) (string, error) {
	var chunks []logChunk
	if err := c.getWithAuth(outputURL, false, &chunks); err != nil {
		return "", fmt.Errorf("fetching step output: %w", err)
	}
	var sb strings.Builder
	for _, chunk := range chunks {
		sb.WriteString(chunk.Message)
	}
	return strings.TrimRight(stripANSI(sb.String()), "\n\r"), nil
}

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07|\x1b[^[]`)

// stripANSI removes terminal escape sequences from s.
func stripANSI(s string) string {
	s = ansiEscape.ReplaceAllString(s, "")
	// Normalise carriage returns left after stripping \r\n sequences
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

// GetJobStepsV1 fetches step output via the v1.1 API, which reliably includes
// output_url even when the v2 job details response returns empty output_url fields.
// projectSlug must be in "gh/org/repo" format; jobNumber is the build number.
func (c *Client) GetJobStepsV1(projectSlug string, jobNumber int) ([]Step, error) {
	vcs, org, repo, err := parseSlug(projectSlug)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/project/%s/%s/%s/%d", baseURLV1, vcs, org, repo, jobNumber)

	type v1Action struct {
		Name      string `json:"name"`
		Status    string `json:"status"`
		ExitCode  *int   `json:"exit_code"`
		OutputURL string `json:"output_url"`
		HasOutput bool   `json:"has_output"`
		Index     int    `json:"index"`
	}
	type v1Step struct {
		Name    string     `json:"name"`
		Actions []v1Action `json:"actions"`
	}
	type v1Build struct {
		Steps []v1Step `json:"steps"`
	}
	var build v1Build
	if err := c.get(u, &build); err != nil {
		return nil, err
	}

	steps := make([]Step, len(build.Steps))
	for i, s := range build.Steps {
		actions := make([]Action, len(s.Actions))
		for j, a := range s.Actions {
			ec := a.ExitCode
			actions[j] = Action{
				Name:      a.Name,
				Status:    a.Status,
				ExitCode:  ec,
				OutputURL: a.OutputURL,
				HasOutput: a.HasOutput,
			}
		}
		steps[i] = Step{Name: s.Name, Actions: actions}
	}
	return steps, nil
}

// parseSlug converts "gh/org/repo" → ("github", "org", "repo").
func parseSlug(slug string) (vcs, org, repo string, err error) {
	parts := strings.SplitN(slug, "/", 3)
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid project slug: %s", slug)
	}
	switch parts[0] {
	case "gh":
		vcs = "github"
	case "bb":
		vcs = "bitbucket"
	default:
		return "", "", "", fmt.Errorf("unsupported VCS in slug: %s", parts[0])
	}
	return vcs, parts[1], parts[2], nil
}

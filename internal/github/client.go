package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type RESTClient interface {
	Request(method string, path string, body io.Reader) (*http.Response, error)
}

type WorkflowRun struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	HTMLURL string `json:"html_url"`
}

type Job struct {
	ID        int64     `json:"id"`
	RunID     int64     `json:"run_id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Labels    []string  `json:"labels"`
	CreatedAt time.Time `json:"created_at"`
	HTMLURL   string    `json:"html_url"`
}

type QueuedJob struct {
	ID           int64     `json:"id"`
	RunID        int64     `json:"run_id"`
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	Labels       []string  `json:"labels"`
	WorkflowName string    `json:"workflow_name"`
	Repository   string    `json:"repository"`
	CreatedAt    time.Time `json:"created_at"`
	Wait         string    `json:"wait"`
	HTMLURL      string    `json:"html_url"`
	RunURL       string    `json:"run_url"`
}

type workflowRunsResponse struct {
	WorkflowRuns []WorkflowRun `json:"workflow_runs"`
}

type jobsResponse struct {
	Jobs []Job `json:"jobs"`
}

func FetchQueuedJobs(client RESTClient, owner, repo string, now time.Time) ([]QueuedJob, error) {
	seenRuns := map[int64]struct{}{}
	var result []QueuedJob
	for _, status := range []string{"queued", "waiting", "in_progress"} {
		runs, err := fetchRuns(client, owner, repo, status)
		if err != nil {
			return nil, err
		}
		for _, run := range runs {
			if _, ok := seenRuns[run.ID]; ok {
				continue
			}
			seenRuns[run.ID] = struct{}{}
			jobs, err := fetchJobs(client, owner, repo, run.ID)
			if err != nil {
				continue
			}
			for _, job := range jobs {
				if !IsQueuedStatus(job.Status) {
					continue
				}
				result = append(result, QueuedJob{
					ID:           job.ID,
					RunID:        job.RunID,
					Name:         job.Name,
					Status:       job.Status,
					Labels:       job.Labels,
					WorkflowName: run.Name,
					Repository:   owner + "/" + repo,
					CreatedAt:    job.CreatedAt,
					Wait:         FormatWait(job.CreatedAt, now),
					HTMLURL:      job.HTMLURL,
					RunURL:       run.HTMLURL,
				})
			}
		}
	}
	return result, nil
}

func fetchRuns(client RESTClient, owner, repo, status string) ([]WorkflowRun, error) {
	var all []WorkflowRun
	page := 1
	for {
		path := fmt.Sprintf(
			"repos/%s/%s/actions/runs?status=%s&per_page=100&page=%d",
			owner, repo, status, page,
		)
		resp, err := client.Request(http.MethodGet, path, nil)
		if err != nil {
			return nil, fmt.Errorf("list runs status=%s: %w", status, err)
		}
		var payload workflowRunsResponse
		decErr := json.NewDecoder(resp.Body).Decode(&payload)
		_ = resp.Body.Close()
		if decErr != nil {
			return nil, decErr
		}
		all = append(all, payload.WorkflowRuns...)
		if len(payload.WorkflowRuns) < 100 {
			break
		}
		page++
	}
	return all, nil
}

func fetchJobs(client RESTClient, owner, repo string, runID int64) ([]Job, error) {
	var all []Job
	page := 1
	for {
		path := fmt.Sprintf(
			"repos/%s/%s/actions/runs/%d/jobs?filter=latest&per_page=100&page=%d",
			owner, repo, runID, page,
		)
		resp, err := client.Request(http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}
		var payload jobsResponse
		decErr := json.NewDecoder(resp.Body).Decode(&payload)
		_ = resp.Body.Close()
		if decErr != nil {
			return nil, decErr
		}
		all = append(all, payload.Jobs...)
		if len(payload.Jobs) < 100 {
			break
		}
		page++
	}
	return all, nil
}

func IsQueuedStatus(status string) bool {
	switch status {
	case "queued", "waiting", "pending", "requested":
		return true
	default:
		return false
	}
}

func FormatWait(created, now time.Time) string {
	if created.IsZero() {
		return "-"
	}
	d := now.Sub(created).Round(time.Second)
	if d < 0 {
		d = 0
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func StatusIcon(status string) string {
	switch status {
	case "queued":
		return "🟡"
	case "waiting":
		return "🔵"
	case "pending", "requested":
		return "⚪"
	default:
		return "❓"
	}
}

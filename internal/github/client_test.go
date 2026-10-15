package github

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type stubClient struct {
	responses map[string]*http.Response
	calls     []string
}

func (s *stubClient) Request(method, path string, _ io.Reader) (*http.Response, error) {
	s.calls = append(s.calls, method+" "+path)
	if resp, ok := s.responses[path]; ok {
		return resp, nil
	}
	return jsonResponse(http.StatusOK, map[string]any{}), nil
}

func jsonResponse(status int, body any) *http.Response {
	raw, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader(raw)),
		Header:     make(http.Header),
	}
}

func TestIsQueuedStatus(t *testing.T) {
	cases := map[string]bool{
		"queued":      true,
		"waiting":     true,
		"pending":     true,
		"requested":   true,
		"in_progress": false,
		"completed":   false,
		"":            false,
	}
	for status, want := range cases {
		if got := IsQueuedStatus(status); got != want {
			t.Errorf("IsQueuedStatus(%q)=%v want %v", status, got, want)
		}
	}
}

func TestFormatWait(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		created time.Time
		want    string
	}{
		{"zero", time.Time{}, "-"},
		{"seconds", now.Add(-45 * time.Second), "00:45"},
		{"minutes", now.Add(-5*time.Minute - 30*time.Second), "05:30"},
		{"hours", now.Add(-2*time.Hour - 15*time.Minute - 45*time.Second), "02:15:45"},
		{"future", now.Add(10 * time.Second), "00:00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatWait(tt.created, now); got != tt.want {
				t.Errorf("got %s want %s", got, tt.want)
			}
		})
	}
}

func TestStatusIcon(t *testing.T) {
	tests := map[string]string{
		"queued":    "🟡",
		"waiting":   "🔵",
		"pending":   "⚪",
		"requested": "⚪",
		"unknown":   "❓",
	}
	for status, want := range tests {
		if got := StatusIcon(status); got != want {
			t.Errorf("StatusIcon(%q)=%s want %s", status, got, want)
		}
	}
}

func TestFetchQueuedJobs(t *testing.T) {
	created := time.Date(2026, 10, 15, 11, 50, 0, 0, time.UTC)
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	client := &stubClient{responses: map[string]*http.Response{
		"repos/xxx/repo123/actions/runs?status=queued&per_page=100&page=1": jsonResponse(http.StatusOK, map[string]any{
			"workflow_runs": []map[string]any{
				{"id": 11, "name": "CI", "html_url": "https://github.com/xxx/repo123/actions/runs/11"},
			},
		}),
		"repos/xxx/repo123/actions/runs?status=waiting&per_page=100&page=1": jsonResponse(http.StatusOK, map[string]any{
			"workflow_runs": []map[string]any{},
		}),
		"repos/xxx/repo123/actions/runs?status=in_progress&per_page=100&page=1": jsonResponse(http.StatusOK, map[string]any{
			"workflow_runs": []map[string]any{
				{"id": 22, "name": "Release", "html_url": "https://github.com/xxx/repo123/actions/runs/22"},
			},
		}),
		"repos/xxx/repo123/actions/runs/11/jobs?filter=latest&per_page=100&page=1": jsonResponse(http.StatusOK, map[string]any{
			"jobs": []map[string]any{
				{
					"id": 101, "run_id": 11, "name": "build", "status": "queued",
					"labels":     []string{"self-hosted", "linux"},
					"created_at": created.Format(time.RFC3339),
					"html_url":   "https://github.com/xxx/repo123/actions/runs/11/job/101",
				},
			},
		}),
		"repos/xxx/repo123/actions/runs/22/jobs?filter=latest&per_page=100&page=1": jsonResponse(http.StatusOK, map[string]any{
			"jobs": []map[string]any{
				{
					"id": 201, "run_id": 22, "name": "test", "status": "in_progress",
					"labels":     []string{"ubuntu-latest"},
					"created_at": created.Format(time.RFC3339),
					"html_url":   "https://github.com/xxx/repo123/actions/runs/22/job/201",
				},
				{
					"id": 202, "run_id": 22, "name": "publish", "status": "waiting",
					"labels":     []string{"self-hosted"},
					"created_at": created.Format(time.RFC3339),
					"html_url":   "https://github.com/xxx/repo123/actions/runs/22/job/202",
				},
			},
		}),
	}}

	jobs, err := FetchQueuedJobs(client, "xxx", "repo123", now)
	if err != nil {
		t.Fatalf("FetchQueuedJobs: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("got %d jobs, want 2: %+v", len(jobs), jobs)
	}
	if jobs[0].ID != 101 || jobs[0].Status != "queued" || jobs[0].WorkflowName != "CI" {
		t.Errorf("first job = %+v", jobs[0])
	}
	if jobs[0].Wait != "10:00" {
		t.Errorf("wait = %s want 10:00", jobs[0].Wait)
	}
	if jobs[1].ID != 202 || jobs[1].Status != "waiting" {
		t.Errorf("second job = %+v", jobs[1])
	}

	var sawLatestFilter bool
	for _, call := range client.calls {
		if strings.Contains(call, "filter=latest") {
			sawLatestFilter = true
		}
		if strings.Contains(call, "filter=all") {
			t.Errorf("queued-job list should use filter=latest, got %s", call)
		}
	}
	if !sawLatestFilter {
		t.Error("expected jobs request with filter=latest")
	}
}

func TestFetchQueuedJobsDedupesRuns(t *testing.T) {
	created := time.Date(2026, 10, 15, 11, 59, 0, 0, time.UTC)
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	run := map[string]any{"id": 7, "name": "CI", "html_url": "https://example/run/7"}
	client := &stubClient{responses: map[string]*http.Response{
		"repos/o/r/actions/runs?status=queued&per_page=100&page=1": jsonResponse(http.StatusOK, map[string]any{
			"workflow_runs": []map[string]any{run},
		}),
		"repos/o/r/actions/runs?status=waiting&per_page=100&page=1": jsonResponse(http.StatusOK, map[string]any{
			"workflow_runs": []map[string]any{run},
		}),
		"repos/o/r/actions/runs?status=in_progress&per_page=100&page=1": jsonResponse(http.StatusOK, map[string]any{
			"workflow_runs": []map[string]any{},
		}),
		"repos/o/r/actions/runs/7/jobs?filter=latest&per_page=100&page=1": jsonResponse(http.StatusOK, map[string]any{
			"jobs": []map[string]any{
				{
					"id": 1, "run_id": 7, "name": "build", "status": "queued",
					"created_at": created.Format(time.RFC3339),
				},
			},
		}),
	}}
	jobs, err := FetchQueuedJobs(client, "o", "r", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Fatalf("got %d jobs, want 1 after run dedupe", len(jobs))
	}
	jobFetches := 0
	for _, call := range client.calls {
		if strings.Contains(call, "/jobs?") {
			jobFetches++
		}
	}
	if jobFetches != 1 {
		t.Errorf("jobs endpoint called %d times, want 1", jobFetches)
	}
}

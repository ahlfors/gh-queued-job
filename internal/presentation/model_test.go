package presentation

import (
	"errors"
	"strings"
	"testing"
	"time"

	gh "github.com/ahlfors/gh-queued-job/internal/github"
	tea "github.com/charmbracelet/bubbletea"
)

func TestViewQuitting(t *testing.T) {
	m := &Model{quitting: true}
	if m.View() != "" {
		t.Fatal("expected empty view when quitting")
	}
}

func TestViewLoading(t *testing.T) {
	m := NewModel(func() ([]gh.QueuedJob, error) { return nil, nil }, "o", "r", "", 5)
	view := m.View()
	if !strings.Contains(view, "Loading") {
		t.Fatalf("expected Loading in view, got %q", view)
	}
	if !strings.Contains(view, "o/r") {
		t.Fatalf("expected repo in header, got %q", view)
	}
}

func TestViewEmpty(t *testing.T) {
	m := NewModel(func() ([]gh.QueuedJob, error) { return nil, nil }, "o", "r", "org", 5)
	m.loading = false
	m.lastUpdate = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	view := m.View()
	if !strings.Contains(view, "No queued / waiting jobs") {
		t.Fatalf("expected empty message, got %q", view)
	}
	if !strings.Contains(view, "Org: org") {
		t.Fatalf("expected org in header, got %q", view)
	}
}

func TestViewError(t *testing.T) {
	m := NewModel(func() ([]gh.QueuedJob, error) { return nil, nil }, "o", "r", "", 5)
	m.loading = false
	m.err = errors.New("boom")
	view := m.View()
	if !strings.Contains(view, "boom") {
		t.Fatalf("expected error in view, got %q", view)
	}
}

func TestUpdateQuit(t *testing.T) {
	m := NewModel(func() ([]gh.QueuedJob, error) { return nil, nil }, "o", "r", "", 5)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	got := next.(*Model)
	if !got.quitting {
		t.Fatal("expected quitting")
	}
	if cmd == nil {
		t.Fatal("expected quit cmd")
	}
}

func TestUpdateData(t *testing.T) {
	m := NewModel(func() ([]gh.QueuedJob, error) { return nil, nil }, "o", "r", "", 5)
	next, _ := m.Update(dataMsg{jobs: []gh.QueuedJob{{
		ID: 42, Name: "build", Status: "queued", Wait: "01:00",
		WorkflowName: "CI", Labels: []string{"self-hosted"},
	}}})
	got := next.(*Model)
	if got.loading {
		t.Fatal("expected loading=false")
	}
	if len(got.jobs) != 1 || got.jobs[0].ID != 42 {
		t.Fatalf("jobs = %+v", got.jobs)
	}
	view := got.View()
	if !strings.Contains(view, "queued: 1") {
		t.Fatalf("expected queued count, got %q", view)
	}
}

func TestGetCalculatedTableHeight(t *testing.T) {
	if h := getCalculatedTableHeight(24); h != 18 {
		t.Errorf("height=%d want 18", h)
	}
	if h := getCalculatedTableHeight(8); h != 5 {
		t.Errorf("min height=%d want 5", h)
	}
}

func TestGetCalculatedColumnWidths(t *testing.T) {
	cols := getCalculatedColumnWidths(40)
	if len(cols) != 5 {
		t.Fatalf("cols=%d", len(cols))
	}
	wide := getCalculatedColumnWidths(160)
	if wide[3].Width <= cols[3].Width {
		t.Errorf("labels width did not grow: min=%d wide=%d", cols[3].Width, wide[3].Width)
	}
}

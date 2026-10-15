package presentation

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	gh "github.com/ahlfors/gh-queued-job/internal/github"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	columnTitleJob      = "JOB"
	columnTitleStatus   = "STATUS"
	columnTitleWait     = "WAIT"
	columnTitleLabels   = "LABELS"
	columnTitleWorkflow = "WORKFLOW / JOB"

	minJobWidth      = 12
	statusWidth      = 12
	waitWidth        = 10
	minLabelsWidth   = 10
	minWorkflowWidth = 16
	borderPadding    = 10
	headerFooterHeight = 6
	ratioLabels      = 0.40
	ratioWorkflow    = 0.60
	defaultWidth     = 80
	defaultHeight    = 24
)

type Fetcher func() ([]gh.QueuedJob, error)

type dataMsg struct {
	jobs []gh.QueuedJob
	err  error
}

type Model struct {
	table          table.Model
	spinner        spinner.Model
	fetcher        Fetcher
	owner          string
	repo           string
	org            string
	jobs           []gh.QueuedJob
	lastUpdate     time.Time
	updateInterval time.Duration
	loading        bool
	quitting       bool
	width          int
	height         int
	err            error
	openURL        func(string) error
}

func NewModel(fetcher Fetcher, owner, repo, org string, intervalSeconds int) *Model {
	columns := []table.Column{
		{Title: columnTitleJob, Width: minJobWidth},
		{Title: columnTitleStatus, Width: statusWidth},
		{Title: columnTitleWait, Width: waitWidth},
		{Title: columnTitleLabels, Width: minLabelsWidth},
		{Title: columnTitleWorkflow, Width: minWorkflowWidth},
	}
	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	styles.Selected = styles.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(getCalculatedTableHeight(defaultHeight)),
		table.WithStyles(styles),
	)
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &Model{
		table:          t,
		spinner:        sp,
		fetcher:        fetcher,
		owner:          owner,
		repo:           repo,
		org:            org,
		updateInterval: time.Duration(intervalSeconds) * time.Second,
		loading:        true,
		width:          defaultWidth,
		height:         defaultHeight,
		openURL:        openBrowser,
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.fetchData(),
		tea.Tick(m.updateInterval, func(t time.Time) tea.Msg { return t }),
	)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetHeight(getCalculatedTableHeight(msg.Height))
		m.updateColumnWidths()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "r":
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.fetchData())
		case "enter", "return":
			return m, m.openSelected()
		}

	case time.Time:
		return m, tea.Batch(
			m.fetchData(),
			tea.Tick(m.updateInterval, func(t time.Time) tea.Msg { return t }),
		)

	case dataMsg:
		if msg.err == nil {
			m.jobs = msg.jobs
			m.lastUpdate = time.Now()
			m.err = nil
			m.updateRows()
		} else {
			m.err = msg.err
		}
		m.loading = false
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *Model) View() string {
	if m.quitting {
		return ""
	}
	header := fmt.Sprintf("GitHub Queued Jobs - Repository: %s/%s\n", m.owner, m.repo)
	if m.org != "" {
		header = fmt.Sprintf("GitHub Queued Jobs - Org: %s  Repo: %s/%s\n", m.org, m.owner, m.repo)
	}
	if m.loading && len(m.jobs) == 0 {
		return header + fmt.Sprintf("\n%s Loading...\n", m.spinner.View())
	}
	spin := ""
	if m.loading {
		spin = m.spinner.View() + " "
	}
	header += fmt.Sprintf(
		"%sLast Updated: %s | queued: %d | q quit, r refresh, enter open job\n\n",
		spin, m.lastUpdate.Format("15:04:05"), len(m.jobs),
	)
	if m.err != nil {
		return header + fmt.Sprintf("Error: %v\n", m.err)
	}
	if len(m.jobs) == 0 {
		return header + "No queued / waiting jobs.\n"
	}
	return header + m.table.View()
}

func (m *Model) fetchData() tea.Cmd {
	return func() tea.Msg {
		jobs, err := m.fetcher()
		return dataMsg{jobs: jobs, err: err}
	}
}

func (m *Model) updateRows() {
	rows := make([]table.Row, 0, len(m.jobs))
	for _, job := range m.jobs {
		labels := "-"
		if len(job.Labels) > 0 {
			labels = strings.Join(job.Labels, ",")
		}
		rows = append(rows, table.Row{
			fmt.Sprintf("%d", job.ID),
			fmt.Sprintf("%s %s", gh.StatusIcon(job.Status), job.Status),
			job.Wait,
			labels,
			fmt.Sprintf("%s / %s", job.WorkflowName, job.Name),
		})
	}
	m.table.SetRows(rows)
}

func (m *Model) updateColumnWidths() {
	columns := getCalculatedColumnWidths(m.width)
	m.table.SetColumns(columns)
}

func (m *Model) openSelected() tea.Cmd {
	return func() tea.Msg {
		idx := m.table.Cursor()
		if idx < 0 || idx >= len(m.jobs) {
			return nil
		}
		url := m.jobs[idx].HTMLURL
		if url == "" {
			url = m.jobs[idx].RunURL
		}
		if url == "" || m.openURL == nil {
			return nil
		}
		if err := m.openURL(url); err != nil {
			return dataMsg{err: err}
		}
		return nil
	}
}

func getCalculatedColumnWidths(terminalWidth int) []table.Column {
	availableWidth := terminalWidth - borderPadding
	totalMinWidth := minJobWidth + statusWidth + waitWidth + minLabelsWidth + minWorkflowWidth
	if availableWidth < totalMinWidth {
		return []table.Column{
			{Title: columnTitleJob, Width: minJobWidth},
			{Title: columnTitleStatus, Width: statusWidth},
			{Title: columnTitleWait, Width: waitWidth},
			{Title: columnTitleLabels, Width: minLabelsWidth},
			{Title: columnTitleWorkflow, Width: minWorkflowWidth},
		}
	}
	remaining := availableWidth - totalMinWidth
	labelsExtra := int(float64(remaining) * ratioLabels)
	workflowExtra := int(float64(remaining) * ratioWorkflow)
	return []table.Column{
		{Title: columnTitleJob, Width: minJobWidth},
		{Title: columnTitleStatus, Width: statusWidth},
		{Title: columnTitleWait, Width: waitWidth},
		{Title: columnTitleLabels, Width: minLabelsWidth + labelsExtra},
		{Title: columnTitleWorkflow, Width: minWorkflowWidth + workflowExtra},
	}
}

func getCalculatedTableHeight(terminalHeight int) int {
	height := terminalHeight - headerFooterHeight
	if height < 5 {
		return 5
	}
	return height
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return nil
	}
	return cmd.Start()
}

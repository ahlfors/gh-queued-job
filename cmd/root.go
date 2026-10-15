package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	ghapi "github.com/ahlfors/gh-queued-job/internal/github"
	"github.com/ahlfors/gh-queued-job/internal/presentation"
	"github.com/cli/go-gh/v2/pkg/api"
	ghrepo "github.com/cli/go-gh/v2/pkg/repository"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	orgFlag      string
	repoFlag     string
	intervalSecs int
	jsonOut      bool
	once         bool
)

func Execute(version string) {
	root := newRootCmd(version)
	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:     "gh-queued-job",
		Short:   "TUI for queued GitHub Actions jobs",
		Long:    "List queued / waiting GitHub Actions jobs. Default is a live TUI; --once / --json for scripts.",
		Version: version,
		RunE:    run,
	}
	root.Flags().StringVar(&orgFlag, "org", "", "Organization (CLI-compatible with runner-monitor; jobs still come from --repo)")
	root.Flags().StringVar(&repoFlag, "repo", "", "Repository (owner/repo)")
	root.Flags().IntVar(&intervalSecs, "interval", 5, "Refresh interval in seconds")
	root.Flags().BoolVar(&jsonOut, "json", false, "Print JSON and exit (implies --once)")
	root.Flags().BoolVar(&once, "once", false, "Print a table once and exit, no TUI")
	return root
}

func run(_ *cobra.Command, _ []string) error {
	owner, repo, err := resolveRepo(orgFlag, repoFlag)
	if err != nil {
		return err
	}
	client, err := api.DefaultRESTClient()
	if err != nil {
		return fmt.Errorf("failed to create REST client: %w\nPlease run 'gh auth login'", err)
	}

	fetch := func() ([]ghapi.QueuedJob, error) {
		return ghapi.FetchQueuedJobs(client, owner, repo, time.Now())
	}

	if jsonOut || once {
		jobs, err := fetch()
		if err != nil {
			return err
		}
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(jobs)
		}
		printTable(owner, repo, jobs)
		return nil
	}

	model := presentation.NewModel(fetch, owner, repo, orgFlag, intervalSecs)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func resolveRepo(org, repo string) (string, string, error) {
	if repo != "" {
		parts := strings.Split(repo, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", "", fmt.Errorf("invalid --repo, use owner/repo")
		}
		return parts[0], parts[1], nil
	}
	if org != "" {
		return "", "", fmt.Errorf("--org alone is not enough; use --repo owner/repo")
	}
	current, err := ghrepo.Current()
	if err != nil {
		return "", "", fmt.Errorf("not in a git repository and no --repo specified")
	}
	return current.Owner, current.Name, nil
}

func printTable(owner, repo string, jobs []ghapi.QueuedJob) {
	fmt.Printf("Queued jobs for %s/%s: %d\n", owner, repo, len(jobs))
	if len(jobs) == 0 {
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "JOB_ID\tNAME\tWORKFLOW\tSTATUS\tLABELS\tWAIT\tURL")
	for _, job := range jobs {
		labels := "-"
		if len(job.Labels) > 0 {
			labels = strings.Join(job.Labels, ",")
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			job.ID, job.Name, job.WorkflowName, job.Status, labels, job.Wait, job.HTMLURL)
	}
	_ = w.Flush()
}

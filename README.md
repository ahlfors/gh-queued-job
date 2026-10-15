# gh-queued-job

A GitHub CLI extension that lists queued / waiting GitHub Actions jobs with a Terminal User Interface (TUI), modeled on [gh-runner-monitor](https://github.com/ahlfors/gh-runner-monitor).

## Features

- Real-time TUI of jobs in `queued`, `waiting`, `pending`, `requested`
- Wait time from `created_at` (queued jobs often have empty `started_at`)
- `enter` opens the job page in the browser
- `--once` / `--json` for scripts and CI
- `--org` is CLI-compatible with runner-monitor; jobs always come from `--repo`

Rows are **jobs**, not runners. A queued job has no `runner_id` yet, so aggregating by runner would show `-`.

## Precondition

1. Install `gh`

macos:

```bash
brew install gh
```

ubuntu:

```bash
sudo apt install gh
```

2. Authenticate and grant `repo` / `actions:read` (private repos need `repo`)

```bash
gh auth login
```

## Installation

```bash
gh extension install ahlfors/gh-queued-job
```

From a local checkout:

```bash
gh extension install .
```

## Usage

Default is TUI. `--once` prints a table and exits. `--json` implies `--once`.

```bash
# current git repo
gh queued-job

# specific repository
gh queued-job --repo xxx/repo123

# same flags as runner-monitor (--org is accepted, jobs still come from --repo)
gh queued-job --org xxx --repo xxx/repo123

# refresh every 10 seconds
gh queued-job --interval 10

# one-shot table
gh queued-job --once --repo xxx/repo123

# JSON for scripts
gh queued-job --json --repo xxx/repo123
```

Open two terminals if you want both views:

```bash
gh runner-monitor --org xxx --repo xxx/repo123
gh queued-job     --org xxx --repo xxx/repo123
```

Do not merge runner idle/busy and job backlog into one wide table. They are different axes.

## Status Colors

- 🟡 **Yellow** - queued
- 🔵 **Blue** - waiting (environment protection)
- ⚪ **White** - pending / requested

## Keyboard Shortcuts

- `↑/↓` or `j/k` - Navigate jobs
- `r` - Manual refresh
- `enter` - Open selected job in the browser
- `q` or `Ctrl+C` - Quit

## How it works

1. List workflow runs with `status=queued|waiting|in_progress` (`filter` is not used on runs).
2. For each run, `GET /repos/{owner}/{repo}/actions/runs/{id}/jobs?filter=latest`.
3. Keep jobs whose status is `queued|waiting|pending|requested`.

`in_progress` runs are still fetched: a matrix can have sibling jobs still queued.

Refresh cost is one `/jobs` call per active run. `--interval 5` is fine for dozens of queued jobs; bump to 10 if you have hundreds.

`--org` alone is not enough. There is no org-wide "list queued jobs" REST endpoint that is cheaper than scanning every repo.

## Development

### Prerequisites

- Go 1.25 or higher (`go.mod` uses `toolchain default`, so your installed Go is used as-is)
- GitHub CLI (`gh`) installed and authenticated

### Building from source

```bash
git clone https://github.com/ahlfors/gh-queued-job.git
cd gh-queued-job
go build -o gh-queued-job
```

If you see `compile: version "go1.25.0" does not match go tool version "go1.27.1"`, the go.mod in an older zip pinned patch `1.25.0` and your `go` downloaded that toolchain. This tree uses `toolchain default`. Workaround on any checkout:

```bash
GOTOOLCHAIN=local go build -o gh-queued-job
```

### Testing locally

```bash
go build -o gh-queued-job
./gh-queued-job --help
./gh-queued-job --once --repo owner/repo

gh extension install .
gh queued-job --once --repo owner/repo
gh extension remove queued-job
```

### Running tests

```bash
go test ./...
go test -cover ./...
go test -v ./...
```

## License

MIT

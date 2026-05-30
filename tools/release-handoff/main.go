// Command release-handoff prints a release handoff document for the current
// checkout and can optionally execute the local release verification set.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const outputLimit = 4000

type config struct {
	outputPath string
	runChecks  bool
	timeout    time.Duration
}

type gitInfo struct {
	Branch      string
	Commit      string
	CommitTitle string
	Status      string
	Clean       bool
}

type checkSpec struct {
	Name string
	Args []string
}

type checkResult struct {
	Name     string
	Command  string
	Passed   bool
	Skipped  bool
	Duration time.Duration
	Output   string
	Error    string
}

type handoffData struct {
	GeneratedAt     time.Time
	Git             gitInfo
	LatestMigration string
	RunChecks       bool
	Checks          []checkResult
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "release_handoff=failed error=%q\n", err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()

	data, err := buildHandoff(ctx, cfg)
	if err != nil {
		return err
	}
	document := renderMarkdown(data)
	if cfg.outputPath == "" {
		fmt.Print(document)
		return nil
	}
	if err := os.WriteFile(cfg.outputPath, []byte(document), 0o644); err != nil {
		return fmt.Errorf("write handoff document: %w", err)
	}
	fmt.Printf("release_handoff=written path=%q\n", cfg.outputPath)
	return nil
}

func parseConfig(args []string) (config, error) {
	cfg := config{timeout: 20 * time.Minute}
	fs := flag.NewFlagSet("release-handoff", flag.ContinueOnError)
	fs.StringVar(&cfg.outputPath, "output", "", "write the handoff document to a file instead of stdout")
	fs.BoolVar(&cfg.runChecks, "run-checks", false, "run local release verification commands and embed results")
	fs.DurationVar(&cfg.timeout, "timeout", cfg.timeout, "overall timeout for metadata and checks")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	cfg.outputPath = strings.TrimSpace(cfg.outputPath)
	if cfg.timeout <= 0 {
		return config{}, errors.New("timeout must be positive")
	}
	return cfg, nil
}

func buildHandoff(ctx context.Context, cfg config) (handoffData, error) {
	info, err := collectGitInfo(ctx)
	if err != nil {
		return handoffData{}, err
	}
	data := handoffData{
		GeneratedAt:     time.Now().UTC(),
		Git:             info,
		LatestMigration: latestMigration("migrations/mysql"),
		RunChecks:       cfg.runChecks,
	}
	for _, spec := range releaseChecks() {
		if !cfg.runChecks {
			data.Checks = append(data.Checks, checkResult{
				Name:    spec.Name,
				Command: shellQuote(spec.Args),
				Skipped: true,
			})
			continue
		}
		data.Checks = append(data.Checks, runCheck(ctx, spec))
	}
	return data, nil
}

func collectGitInfo(ctx context.Context) (gitInfo, error) {
	branch, err := gitOutput(ctx, "branch", "--show-current")
	if err != nil {
		return gitInfo{}, err
	}
	commit, err := gitOutput(ctx, "rev-parse", "--short", "HEAD")
	if err != nil {
		return gitInfo{}, err
	}
	title, err := gitOutput(ctx, "log", "-1", "--pretty=%s")
	if err != nil {
		return gitInfo{}, err
	}
	status, err := gitOutput(ctx, "status", "--short")
	if err != nil {
		return gitInfo{}, err
	}
	status = strings.TrimSpace(status)
	return gitInfo{
		Branch:      strings.TrimSpace(branch),
		Commit:      strings.TrimSpace(commit),
		CommitTitle: strings.TrimSpace(title),
		Status:      status,
		Clean:       status == "",
	}, nil
}

func gitOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func releaseChecks() []checkSpec {
	return []checkSpec{
		{Name: "unit and integration tests", Args: []string{"go", "test", "./..."}},
		{Name: "go vet", Args: []string{"go", "vet", "./..."}},
		{Name: "command build", Args: []string{"go", "build", "./cmd/..."}},
		{Name: "portal and OpenAPI contracts", Args: []string{"go", "test", "./tools/portal-smoke", "./tools/release-handoff", "./tests/contract"}},
		{Name: "RC smoke syntax", Args: []string{"bash", "-n", "tests/rc/clean_env_smoke.sh"}},
		{Name: "release gate", Args: []string{"tests/failure/release_gate.sh"}},
	}
}

func runCheck(ctx context.Context, spec checkSpec) checkResult {
	started := time.Now()
	result := checkResult{Name: spec.Name, Command: shellQuote(spec.Args)}
	if len(spec.Args) == 0 {
		result.Error = "empty command"
		return result
	}
	cmd := exec.CommandContext(ctx, spec.Args[0], spec.Args[1:]...)
	out, err := cmd.CombinedOutput()
	result.Duration = time.Since(started)
	result.Output = truncate(redactSecrets(string(out)), outputLimit)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Passed = true
	return result
}

func latestMigration(dir string) string {
	var migrations []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".up.sql") {
			return nil
		}
		migrations = append(migrations, filepath.Base(path))
		return nil
	})
	sort.Strings(migrations)
	if len(migrations) == 0 {
		return "n/a"
	}
	return migrations[len(migrations)-1]
}

func renderMarkdown(data handoffData) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Release Handoff\n\n")
	fmt.Fprintf(&b, "- Generated at: `%s`\n", data.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "- Branch: `%s`\n", emptyAs(data.Git.Branch, "unknown"))
	fmt.Fprintf(&b, "- Commit: `%s` `%s`\n", emptyAs(data.Git.Commit, "unknown"), emptyAs(data.Git.CommitTitle, "unknown"))
	fmt.Fprintf(&b, "- Worktree clean: `%t`\n", data.Git.Clean)
	fmt.Fprintf(&b, "- Latest migration: `%s`\n", data.LatestMigration)
	if !data.Git.Clean {
		fmt.Fprintf(&b, "\n## Worktree Status\n\n```text\n%s\n```\n", data.Git.Status)
	}

	fmt.Fprintf(&b, "\n## Validation\n\n")
	fmt.Fprintf(&b, "| Check | Command | Result | Duration |\n")
	fmt.Fprintf(&b, "|---|---|---|---|\n")
	for _, check := range data.Checks {
		fmt.Fprintf(&b, "| %s | `%s` | %s | %s |\n", check.Name, check.Command, checkStatus(check), check.Duration.Round(time.Millisecond))
	}
	if data.RunChecks {
		for _, check := range data.Checks {
			if check.Passed || check.Output == "" {
				continue
			}
			fmt.Fprintf(&b, "\n### %s Output\n\n```text\n%s\n```\n", check.Name, check.Output)
		}
	} else {
		fmt.Fprintf(&b, "\nRun `go run ./tools/release-handoff -run-checks` before release to embed current validation evidence.\n")
	}

	fmt.Fprintf(&b, "\n## Customer Acceptance\n\n")
	fmt.Fprintf(&b, "- Run `tests/rc/clean_env_smoke.sh` in a disposable RC environment.\n")
	fmt.Fprintf(&b, "- Confirm `rc_smoke=portal_customer_acceptance` and `portal_smoke=passed` are present.\n")
	fmt.Fprintf(&b, "- For staging, run `go run ./tools/portal-smoke -gateway-url ${GATEWAY_URL} -api-key ${API_KEY} -model ${MODEL}`.\n")

	fmt.Fprintf(&b, "\n## Release Fields\n\n")
	fmt.Fprintf(&b, "| Field | Value |\n")
	fmt.Fprintf(&b, "|---|---|\n")
	fmt.Fprintf(&b, "| Release commit | `%s` |\n", emptyAs(data.Git.Commit, ""))
	fmt.Fprintf(&b, "| Branch / PR | `%s` |\n", emptyAs(data.Git.Branch, ""))
	fmt.Fprintf(&b, "| Image tag |  |\n")
	fmt.Fprintf(&b, "| Migration latest | `%s` |\n", data.LatestMigration)
	fmt.Fprintf(&b, "| Snapshot version |  |\n")
	fmt.Fprintf(&b, "| Redis key prefix |  |\n")
	fmt.Fprintf(&b, "| Release gate result |  |\n")
	fmt.Fprintf(&b, "| Portal smoke result |  |\n")
	fmt.Fprintf(&b, "| Rollback tested |  |\n")
	fmt.Fprintf(&b, "| Reconciliation result |  |\n")
	fmt.Fprintf(&b, "| Known risks |  |\n")
	fmt.Fprintf(&b, "| Owner / time |  |\n")

	fmt.Fprintf(&b, "\n## Rollback\n\n")
	fmt.Fprintf(&b, "Prefer snapshot rollback before code rollback:\n\n")
	fmt.Fprintf(&b, "```bash\n")
	fmt.Fprintf(&b, "curl -fsS -X POST -H \"X-Admin-Token: ${ADMIN_TOKEN}\" \\\n")
	fmt.Fprintf(&b, "  \"${CONFIGD_URL}/configd/snapshots/rollback\"\n")
	fmt.Fprintf(&b, "```\n")
	return b.String()
}

func checkStatus(result checkResult) string {
	switch {
	case result.Skipped:
		return "not run"
	case result.Passed:
		return "passed"
	default:
		return "failed: " + result.Error
	}
}

func shellQuote(args []string) string {
	var quoted []string
	for _, arg := range args {
		if arg == "" || strings.ContainsAny(arg, " \t\n'\"$`\\") {
			quoted = append(quoted, "'"+strings.ReplaceAll(arg, "'", "'\\''")+"'")
			continue
		}
		quoted = append(quoted, arg)
	}
	return strings.Join(quoted, " ")
}

func redactSecrets(value string) string {
	replacements := []struct {
		pattern     *regexp.Regexp
		replacement string
	}{
		{regexp.MustCompile(`sk-[A-Za-z0-9_-]{12,}`), "[REDACTED]"},
		{regexp.MustCompile(`AKIA[0-9A-Z]{16}`), "[REDACTED]"},
		{regexp.MustCompile(`ghp_[A-Za-z0-9]{20,}`), "[REDACTED]"},
		{regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{12,}`), "[REDACTED]"},
		{regexp.MustCompile(`(?i)(api[_-]?key|authorization|token|password|secret)(=|:)\s*[^ \n]+`), "$1$2 [REDACTED]"},
	}
	out := value
	for _, replacement := range replacements {
		out = replacement.pattern.ReplaceAllString(out, replacement.replacement)
	}
	return out
}

func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "\n... truncated ..."
}

func emptyAs(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func checkSummary(results []checkResult) string {
	var b bytes.Buffer
	for _, result := range results {
		fmt.Fprintf(&b, "%s=%s\n", result.Name, checkStatus(result))
	}
	return b.String()
}

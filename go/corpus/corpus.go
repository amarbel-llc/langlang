package corpus

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/clarete/langlang/go"
)

// CorpusDir is the -corpus=dir flag for differential corpus tests.
// RunFromFlag reads it, expands ~, and runs the corpus; callers pass
// Config with Dir left empty.
var CorpusDir = flag.String("corpus", "", "directory of source files for differential corpus test")

// CorpusCacheDir is the -corpus_cache=dir flag for caching downloaded
// or cloned resources (e.g. Rust AutoCorpus repos, JavaScript
// real-world files). When set, tests use this directory instead of a
// temp dir and do not remove it after the run.
var CorpusCacheDir = flag.String("corpus_cache", "", "directory to cache downloaded/cloned resources for corpus tests (optional)")

// CorpusCacheDirExpanded returns the -corpus_cache path with ~
// expanded, or ("", false) if -corpus_cache is unset. Call
// flag.Parse() before use.
func CorpusCacheDirExpanded() (string, bool) {
	s := *CorpusCacheDir
	if s == "" {
		return "", false
	}
	if strings.HasPrefix(s, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		s = filepath.Join(home, s[2:])
	}
	return s, true
}

// Repo describes a git repository to clone (e.g. for corpus or benchmark fixtures).
type Repo struct {
	Name   string // short name used for directory under cache
	URL    string // git clone URL
	Branch string // empty = default branch
}

// CloneRepoIfNeeded clones the repo into dest if dest does not already
// contain a .git directory. Used with -corpus_cache to avoid re-downloading
// on repeated runs. tb may be *testing.T or *testing.B.
func CloneRepoIfNeeded(tb testing.TB, repo Repo, dest string) {
	tb.Helper()
	if _, err := os.Stat(filepath.Join(dest, ".git")); err == nil {
		tb.Logf("using cached clone at %s", dest)
		return
	}
	CloneRepo(tb, repo, dest)
}

// CloneRepo runs git clone --depth=1 (and --branch if repo.Branch != "")
// into dest. Logs to stderr and calls tb.Fatalf on failure. tb may be
// *testing.T or *testing.B.
func CloneRepo(tb testing.TB, repo Repo, dest string) {
	tb.Helper()
	args := []string{"clone", "--depth=1"}
	if repo.Branch != "" {
		args = append(args, "--branch", repo.Branch)
	}
	args = append(args, repo.URL, dest)

	tb.Logf("cloning %s ...", repo.URL)
	cmd := exec.Command("git", args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		tb.Fatalf("git clone %s failed: %v", repo.URL, err)
	}
}

// RunFromFlag parses flags, reads -corpus=dir, expands ~, validates
// the directory, sets cfg.Dir, and calls Run. Skip message uses
// cfg.LangName.  Use this from TestXxxDifferentialCorpus so the flag
// is defined and handled in one place.
func RunFromFlag(t *testing.T, cfg Config) {
	t.Helper()
	flag.Parse()
	dir := *CorpusDir
	if dir == "" {
		t.Skipf("run with -corpus=dir to test %s corpus", cfg.LangName)
	}
	if strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("cannot expand ~: %v", err)
		}
		dir = filepath.Join(home, dir[2:])
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("corpus directory does not exist or is not a directory: %s", dir)
	}
	cfg.Dir = dir
	Run(t, cfg)
}

// AssertParsesAll runs matcher.Match(data) and fails the test unless
// parsing succeeds and consumes all bytes. On failure, logs location
// for ParsingErrors.  Use this for snippet tests and for each file in
// RunTestFiles.
func AssertParsesAll(t *testing.T, matcher langlang.Matcher, data []byte, name string) {
	t.Helper()
	tree, n, err := matcher.Match(data)
	if err != nil {
		if perr, ok := err.(langlang.ParsingError); ok && tree != nil {
			loc := tree.Location(perr.End)
			t.Logf("  error at %d:%d: %s", loc.Line, loc.Column, perr.Message)
		}
		t.Errorf("failed to parse %s: %v", name, err)
		return
	}
	if n != len(data) {
		t.Errorf("parser did not consume all input for %s: consumed %d of %d bytes", name, n, len(data))
	}
}

// RunTestFiles runs a subtest per file in dir whose extension matches
// ext (e.g. ".ts", ".java"), parsing each with matcher and calling
// AssertParsesAll.  Fails if the directory cannot be read or contains
// no matching files.
func RunTestFiles(t *testing.T, matcher langlang.Matcher, dir string, ext string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	var ran int
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ext {
			continue
		}
		ran++
		t.Run(entry.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				t.Fatalf("read file: %v", err)
			}
			AssertParsesAll(t, matcher, data, entry.Name())
		})
	}
	if ran == 0 {
		t.Fatalf("no %s files found in %s", ext, dir)
	}
}

// Config configures a differential corpus run. The test walks a
// directory of source files, parses each with the given matcher, and
// reports pass/fail.
type Config struct {
	// Dir is the corpus root (e.g. a repo path). ~ is expanded.
	Dir string
	// Extensions lists file extensions to parse (e.g. [".ts"],
	// [".js", ".mjs"]).
	Extensions []string
	// Matcher is the langlang parser to use.
	Matcher langlang.Matcher
	// SkipDirs are directory names to skip when walking
	// (e.g. "node_modules", ".git").
	SkipDirs map[string]bool
	// FailThreshold is the minimum pass rate (0–100). If > 0 and
	// pass rate is below, test fails.
	FailThreshold float64
	// PerFileTimeout limits parse time per file. Zero means no
	// timeout.
	PerFileTimeout time.Duration
	// LangName is used in the summary log (e.g. "TypeScript",
	// "Rust").
	LangName string
	// OnFailures, if set, is called with the failure list after
	// logging individual failures but before the summary
	// (e.g. for language-specific breakdowns).
	OnFailures func(t *testing.T, failures []Failure)
}

// Failure records one file that failed to parse.
type Failure struct {
	Rel    string
	Detail string
}

// Run walks the corpus directory, parses each matching file, logs
// failures and a summary, and fails the test if the pass rate is
// below FailThreshold.  It returns (pass, fail, skip, timeout) counts
// and the list of failures for callers that want extra reporting
// (e.g. Rust's directory breakdown).
func Run(t *testing.T, cfg Config) (pass, fail, skip, timeout int, failures []Failure) {
	t.Helper()

	extSet := make(map[string]bool)
	for _, e := range cfg.Extensions {
		extSet[e] = true
	}
	extOK := func(path string) bool {
		return extSet[filepath.Ext(path)]
	}

	err := filepath.WalkDir(cfg.Dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if cfg.SkipDirs != nil && cfg.SkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !extOK(path) {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			skip++
			return nil
		}
		if len(data) == 0 {
			skip++
			return nil
		}

		rel, _ := filepath.Rel(cfg.Dir, path)
		var tree langlang.Tree
		var n int
		var llErr error

		if cfg.PerFileTimeout > 0 {
			ctx, cancel := context.WithTimeout(context.Background(), cfg.PerFileTimeout)
			defer cancel()
			tree, n, llErr = matchWithTimeout(ctx, cfg.Matcher, data)
		} else {
			tree, n, llErr = cfg.Matcher.Match(data)
		}

		if llErr != nil || n != len(data) {
			fail++
			detail := ""
			if llErr != nil {
				if perr, ok := llErr.(langlang.ParsingError); ok && tree != nil {
					loc := tree.Location(perr.End)
					detail = fmt.Sprintf(" at %d:%d: %s", loc.Line, loc.Column, perr.Message)
				} else {
					if strings.Contains(llErr.Error(), "timeout") {
						timeout++
					}
					detail = fmt.Sprintf(": %v", llErr)
				}
			} else {
				detail = fmt.Sprintf(" (consumed %d/%d bytes)", n, len(data))
			}
			failures = append(failures, Failure{Rel: rel, Detail: detail})
		} else {
			pass++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk error: %v", err)
	}

	total := pass + fail
	if total == 0 {
		exts := strings.Join(cfg.Extensions, ", ")
		t.Fatalf("no %s files found in corpus", exts)
	}

	pct := float64(pass) / float64(total) * 100

	const maxShow = 200
	for i, f := range failures {
		if i >= maxShow {
			t.Logf("... and %d more failures", len(failures)-maxShow)
			break
		}
		t.Logf("FAIL %s%s", f.Rel, f.Detail)
	}
	if cfg.OnFailures != nil {
		cfg.OnFailures(t, failures)
	}

	t.Logf("")
	t.Logf("%s corpus [%s]: %d/%d pass (%.1f%%), %d fail, %d skipped, %d timeout",
		cfg.LangName, filepath.Base(cfg.Dir), pass, total, pct, fail, skip, timeout)

	if cfg.FailThreshold > 0 && pct < cfg.FailThreshold {
		t.Errorf("pass rate %.1f%% is below %.0f%% threshold", pct, cfg.FailThreshold)
	}
	return pass, fail, skip, timeout, failures
}

func matchWithTimeout(ctx context.Context, matcher langlang.Matcher, data []byte) (langlang.Tree, int, error) {
	type result struct {
		tree langlang.Tree
		n    int
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- result{nil, 0, fmt.Errorf("panic: %v", r)}
			}
		}()
		tree, n, err := matcher.Match(data)
		ch <- result{tree, n, err}
	}()
	select {
	case r := <-ch:
		return r.tree, r.n, r.err
	case <-ctx.Done():
		return nil, 0, fmt.Errorf("timeout after %s", ctx.Err())
	}
}

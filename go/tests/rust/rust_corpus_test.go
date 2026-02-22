package rust

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/clarete/langlang/go"
	"github.com/clarete/langlang/go/corpus"

	"github.com/stretchr/testify/require"
)

var corpusSkipDirs = map[string]bool{
	"target":       true,
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"test_data":    true,
}

// TestRustDifferentialCorpus walks a real-world Rust codebase and
// parses every .rs file with the Rust grammar. Since the corpus comes
// from a working project, every file is assumed to be valid Rust; any
// parse failure is a grammar gap.
//
//	go test -run TestRustDifferentialCorpus -v -timeout 30m \
//		 ./tests/rust/ -args -corpus=~/path/to/rust/repo
func TestRustDifferentialCorpus(t *testing.T) {
	cfg := langlang.NewConfig()
	cfg.SetBool("grammar.handle_spaces", false)
	matcher, err := langlang.MatcherFromFilePathWithConfig(grammarPath, cfg)
	require.NoError(t, err)
	corpus.RunFromFlag(t, corpus.Config{
		Extensions:     []string{".rs"},
		Matcher:        matcher,
		SkipDirs:       corpusSkipDirs,
		FailThreshold:  10.0,
		PerFileTimeout: 10 * time.Second,
		LangName:       "Rust",
		OnFailures:     rustFailureBreakdown,
	})
}

var defaultCorpusRepos = []corpus.Repo{
	{Name: "ripgrep", URL: "https://github.com/BurntSushi/ripgrep.git", Branch: ""},
	{Name: "serde", URL: "https://github.com/serde-rs/serde.git", Branch: ""},
	{Name: "tokio", URL: "https://github.com/tokio-rs/tokio.git", Branch: ""},
	{Name: "rust", URL: "https://github.com/rust-lang/rust.git", Branch: ""},
	{Name: "rust-analyzer", URL: "https://github.com/rust-lang/rust-analyzer.git", Branch: ""},
	{Name: "bevy", URL: "https://github.com/bevyengine/bevy.git", Branch: ""},
	{Name: "diesel", URL: "https://github.com/diesel-rs/diesel.git", Branch: ""},
	{Name: "syn", URL: "https://github.com/dtolnay/syn.git", Branch: ""},
}

// TestRustAutoCorpus clones well-known Rust repos and parses every
// .rs file.  Uses Edition 2024 grammar except for syn (Edition 2021;
// gen is identifier). Needs network. Use -corpus_cache=dir to cache
// cloned repos and avoid re-downloading.
//
//	go test -run TestRustAutoCorpus -v -timeout 60m ./tests/rust/
//	go test -run TestRustAutoCorpus -v -timeout 60m ./tests/rust/ -args -corpus_cache=~/cache/rust-corpus
func TestRustAutoCorpus(t *testing.T) {
	if os.Getenv("AUTO_CORPUS") != "true" {
		t.Skip()
	}
	flag.Parse()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	baseDir, useCache := corpus.CorpusCacheDirExpanded()
	if !useCache {
		var err error
		baseDir, err = os.MkdirTemp("", "langlang-rust-corpus-*")
		if err != nil {
			t.Fatalf("cannot create temp dir: %v", err)
		}
		defer os.RemoveAll(baseDir)
	} else {
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			t.Fatalf("cannot create cache dir %s: %v", baseDir, err)
		}
	}

	for _, repo := range defaultCorpusRepos {
		t.Run(repo.Name, func(t *testing.T) {
			dest := filepath.Join(baseDir, repo.Name)
			corpus.CloneRepoIfNeeded(t, repo, dest)
			grammar := grammarPathForRepo(repo.Name)
			cfg := langlang.NewConfig()
			cfg.SetBool("grammar.handle_spaces", false)
			matcher, err := langlang.MatcherFromFilePathWithConfig(grammar, cfg)
			require.NoError(t, err)
			corpus.Run(t, corpus.Config{
				Dir:            dest,
				Extensions:     []string{".rs"},
				Matcher:        matcher,
				SkipDirs:       corpusSkipDirs,
				FailThreshold:  0,
				PerFileTimeout: 10 * time.Second,
				LangName:       "Rust",
				OnFailures:     rustFailureBreakdown,
			})
		})
	}
}

// grammarPathForRepo returns the grammar to use for a corpus
// repo. syn uses Edition 2021 (gen is identifier).
func grammarPathForRepo(name string) string {
	if name == "syn" {
		return grammarPath2021
	}
	return grammarPath
}

// TestRustAutoCorpus2021 is like TestRustAutoCorpus but uses
// rust2021.peg for all repos (Edition 2021).
//
//	go test -run TestRustAutoCorpus2021 -v -timeout 60m ./tests/rust/
//	go test -run TestRustAutoCorpus2021 -v -timeout 60m ./tests/rust/ -args -corpus_cache=~/cache/rust-corpus
func TestRustAutoCorpus2021(t *testing.T) {
	if os.Getenv("AUTO_CORPUS") != "true" {
		t.Skip()
	}
	flag.Parse()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	baseDir, useCache := corpus.CorpusCacheDirExpanded()
	if !useCache {
		var err error
		baseDir, err = os.MkdirTemp("", "langlang-rust-corpus-*")
		if err != nil {
			t.Fatalf("cannot create temp dir: %v", err)
		}
		defer os.RemoveAll(baseDir)
	} else {
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			t.Fatalf("cannot create cache dir %s: %v", baseDir, err)
		}
	}

	for _, repo := range defaultCorpusRepos {
		t.Run(repo.Name, func(t *testing.T) {
			dest := filepath.Join(baseDir, repo.Name)
			corpus.CloneRepoIfNeeded(t, repo, dest)
			cfg := langlang.NewConfig()
			cfg.SetBool("grammar.handle_spaces", false)
			matcher, err := langlang.MatcherFromFilePathWithConfig(grammarPath2021, cfg)
			require.NoError(t, err)
			corpus.Run(t, corpus.Config{
				Dir:            dest,
				Extensions:     []string{".rs"},
				Matcher:        matcher,
				SkipDirs:       corpusSkipDirs,
				FailThreshold:  0,
				PerFileTimeout: 10 * time.Second,
				LangName:       "Rust",
				OnFailures:     rustFailureBreakdown,
			})
		})
	}
}

// rustFailureBreakdown is passed to corpus.Run as OnFailures to log
// directory and tests/ui breakdowns.
func rustFailureBreakdown(t *testing.T, failures []corpus.Failure) {
	if len(failures) == 0 {
		return
	}
	type dirEntry struct {
		dir   string
		count int
	}
	dirCounts := make(map[string]int)
	for _, f := range failures {
		parts := strings.SplitN(f.Rel, string(filepath.Separator), 3)
		key := parts[0]
		if len(parts) > 1 {
			key = parts[0] + "/" + parts[1]
		}
		dirCounts[key]++
	}
	var sorted []dirEntry
	for d, c := range dirCounts {
		sorted = append(sorted, dirEntry{d, c})
	}
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].count > sorted[i].count {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	t.Logf("")
	t.Logf("Failure breakdown by directory:")
	for _, e := range sorted {
		if e.count >= 3 {
			t.Logf("  %4d  %s", e.count, e.dir)
		}
	}

	uiCounts := make(map[string]int)
	for _, f := range failures {
		if !strings.HasPrefix(f.Rel, "tests/ui/") {
			continue
		}
		rest := f.Rel[len("tests/ui/"):]
		parts := strings.SplitN(rest, string(filepath.Separator), 2)
		uiCounts[parts[0]]++
	}
	var uiSorted []dirEntry
	for d, c := range uiCounts {
		uiSorted = append(uiSorted, dirEntry{d, c})
	}
	for i := 0; i < len(uiSorted); i++ {
		for j := i + 1; j < len(uiSorted); j++ {
			if uiSorted[j].count > uiSorted[i].count {
				uiSorted[i], uiSorted[j] = uiSorted[j], uiSorted[i]
			}
		}
	}
	if len(uiSorted) > 0 {
		t.Logf("")
		t.Logf("tests/ui breakdown (top 40):")
		for i, e := range uiSorted {
			if i >= 40 {
				break
			}
			t.Logf("  %4d  tests/ui/%s", e.count, e.dir)
		}
	}
}

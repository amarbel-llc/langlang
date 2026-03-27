package goviews

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func testdataDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..",
		"testdata", "go")
}

func TestParseTestdata(t *testing.T) {
	dir := testdataDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read testdata dir: %v", err)
	}

	parser := NewGoParser()
	parser.SetShowFails(false)

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			parser.SetInput(data)
			tree, err := parser.ParseSourceFile()
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			root, ok := tree.Root()
			if !ok {
				t.Fatal("no root node")
			}
			text := tree.Text(root)
			if text != string(data) {
				t.Errorf("root text length %d != input length %d", len(text), len(data))
			}
		})
	}
}

// goStdlibBenchFiles returns paths to representative Go stdlib source
// files for benchmarking. Skips if GOROOT is unavailable.
func goStdlibBenchFiles(t testing.TB) map[string][]byte {
	t.Helper()
	goroot := runtime.GOROOT()
	if goroot == "" {
		t.Skip("GOROOT not available")
	}

	files := map[string]string{
		"30kb":  filepath.Join(goroot, "src", "encoding", "json", "decode.go"),
		"500kb": filepath.Join(goroot, "src", "net", "http", "h2_bundle.go"),
	}

	result := make(map[string][]byte, len(files))
	for name, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Skipf("skip %s: %v", name, err)
		}
		result[name] = data
	}
	return result
}

func TestParseStdlib(t *testing.T) {
	inputs := goStdlibBenchFiles(t)

	parser := NewGoParser()
	parser.SetShowFails(false)

	for name, data := range inputs {
		t.Run(name, func(t *testing.T) {
			t.Logf("parsing %d bytes", len(data))
			parser.SetInput(data)
			tree, err := parser.ParseSourceFile()
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			root, ok := tree.Root()
			if !ok {
				t.Fatal("no root node")
			}
			text := tree.Text(root)
			if len(text) != len(data) {
				t.Errorf("root text length %d != input length %d", len(text), len(data))
			}
		})
	}
}

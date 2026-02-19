package javascript

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/clarete/langlang/go"
	"github.com/clarete/langlang/go/corpus"

	"github.com/stretchr/testify/require"
)

const (
	grammarES5Path    = "../../../grammars/javascript/es5.peg"
	grammarESNextPath = "../../../grammars/javascript/esnext.peg"

	es5TestdataDir    = "../../../testdata/javascript"
	es2015TestdataDir = "../../../testdata/javascript/es2015"
)

var realWorldES5Files = []string{
	"jquery-1.12.4.min.js",
	"underscore-1.13.6.min.js",
	"backbone-1.4.1.min.js",
	"mustache-2.3.2.min.js",
	"store-1.3.20.min.js",
	"d3-3.5.17.min.js",
	"es5-shim-4.6.7.min.js",
	"q-1.5.1.min.js",
}

var realWorldES2015Files = []string{
	"axios-1.6.8.min.js",
	"dayjs-1.11.10.min.js",
	"jquery-1.12.4.min.js",
	"moment-2.29.4.min.js",
	"mustache-2.3.2.min.js",
	"react-18.2.0.production.min.js",
	"react-dom-18.2.0.production.min.js",
	"ramda-0.30.1.min.js",
	"redux-4.2.1.min.js",
	"store-1.3.20.min.js",
	"underscore-1.13.6.min.js",
	"uuid-8.3.2.min.js",
	"backbone-1.4.1.min.js",
	"bluebird-3.7.2.min.js",
	"d3-3.5.17.min.js",
	"d3-7.8.5.min.js",
	"es5-shim-4.6.7.min.js",
	"lodash-4.17.21.min.js",
	"q-1.5.1.min.js",
	"core-js-bundle-3.37.1.min.js",
	"zone.js-0.14.8.umd.js",
	"rxjs-7.8.1.umd.min.js",
	"three-0.152.2.min.js",
	"vue-3.4.31.global.prod.js",
	"mobx-6.13.2.umd.production.min.js",
	"preact-10.26.4.min.js",
	"immutable-4.3.7.min.js",
	"echarts-5.5.0.min.js",
	"handlebars-4.7.8.min.js",
	"chartjs-4.4.1.umd.min.js",
	"fabricjs-5.3.1.min.js",
	"animejs-3.2.2.min.js",
	"protobufjs-7.4.0.min.js",
	"pdfjs-3.11.174.min.js",
	"highlightjs-11.10.0.min.js",
	"babel-standalone-7.25.6.min.js",
	"mermaid-11.4.0.min.js",
	"monaco-0.52.2.vs-loader.js",
	"sqljs-1.11.0.sql-wasm.js",
	"terser-5.31.1.bundle.min.js",
	"vue-compiler-dom-3.5.13.global.prod.js",
}

var realworldSources = map[string]string{
	"animejs-3.2.2.min.js":                   "https://cdnjs.cloudflare.com/ajax/libs/animejs/3.2.2/anime.min.js",
	"axios-1.6.8.min.js":                     "https://cdnjs.cloudflare.com/ajax/libs/axios/1.6.8/axios.min.js",
	"backbone-1.4.1.min.js":                  "https://cdnjs.cloudflare.com/ajax/libs/backbone.js/1.4.1/backbone-min.js",
	"babel-standalone-7.25.6.min.js":         "https://unpkg.com/@babel/standalone@7.25.6/babel.min.js",
	"bluebird-3.7.2.min.js":                  "https://cdnjs.cloudflare.com/ajax/libs/bluebird/3.7.2/bluebird.min.js",
	"chartjs-4.4.1.umd.min.js":               "https://cdnjs.cloudflare.com/ajax/libs/Chart.js/4.4.1/chart.umd.min.js",
	"core-js-2.6.12.min.js":                  "https://cdnjs.cloudflare.com/ajax/libs/core-js/2.6.12/core.min.js",
	"core-js-bundle-3.37.1.min.js":           "https://cdn.jsdelivr.net/npm/core-js-bundle@3.37.1/minified.js",
	"d3-3.5.17.min.js":                       "https://cdnjs.cloudflare.com/ajax/libs/d3/3.5.17/d3.min.js",
	"d3-7.8.5.min.js":                        "https://cdnjs.cloudflare.com/ajax/libs/d3/7.8.5/d3.min.js",
	"dayjs-1.11.10.min.js":                   "https://cdnjs.cloudflare.com/ajax/libs/dayjs/1.11.10/dayjs.min.js",
	"echarts-5.5.0.min.js":                   "https://cdnjs.cloudflare.com/ajax/libs/echarts/5.5.0/echarts.min.js",
	"es5-shim-4.6.7.min.js":                  "https://cdnjs.cloudflare.com/ajax/libs/es5-shim/4.6.7/es5-shim.min.js",
	"fabricjs-5.3.1.min.js":                  "https://cdnjs.cloudflare.com/ajax/libs/fabric.js/5.3.1/fabric.min.js",
	"handlebars-4.7.8.min.js":                "https://cdnjs.cloudflare.com/ajax/libs/handlebars.js/4.7.8/handlebars.min.js",
	"highlightjs-11.10.0.min.js":             "https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.10.0/highlight.min.js",
	"immutable-4.3.7.min.js":                 "https://unpkg.com/immutable@4.3.7/dist/immutable.min.js",
	"jquery-1.12.4.min.js":                   "https://code.jquery.com/jquery-1.12.4.min.js",
	"lodash-4.17.21.min.js":                  "https://cdnjs.cloudflare.com/ajax/libs/lodash.js/4.17.21/lodash.min.js",
	"mermaid-11.4.0.min.js":                  "https://unpkg.com/mermaid@11.4.0/dist/mermaid.min.js",
	"mobx-6.13.2.umd.production.min.js":      "https://unpkg.com/mobx@6.13.2/dist/mobx.umd.production.min.js",
	"moment-2.29.4.min.js":                   "https://cdnjs.cloudflare.com/ajax/libs/moment.js/2.29.4/moment.min.js",
	"monaco-0.52.2.vs-loader.js":             "https://unpkg.com/monaco-editor@0.52.2/min/vs/loader.js",
	"mustache-2.3.2.min.js":                  "https://cdnjs.cloudflare.com/ajax/libs/mustache.js/2.3.2/mustache.min.js",
	"pdfjs-3.11.174.min.js":                  "https://unpkg.com/pdfjs-dist@3.11.174/build/pdf.min.js",
	"preact-10.26.4.min.js":                  "https://unpkg.com/preact@10.26.4/dist/preact.min.js",
	"protobufjs-7.4.0.min.js":                "https://unpkg.com/protobufjs@7.4.0/dist/protobuf.min.js",
	"q-1.5.1.min.js":                         "https://cdnjs.cloudflare.com/ajax/libs/q.js/1.5.1/q.min.js",
	"ramda-0.30.1.min.js":                    "https://cdnjs.cloudflare.com/ajax/libs/ramda/0.30.1/ramda.min.js",
	"react-18.2.0.production.min.js":         "https://unpkg.com/react@18.2.0/umd/react.production.min.js",
	"react-dom-18.2.0.production.min.js":     "https://unpkg.com/react-dom@18.2.0/umd/react-dom.production.min.js",
	"redux-4.2.1.min.js":                     "https://cdnjs.cloudflare.com/ajax/libs/redux/4.2.1/redux.min.js",
	"rxjs-5.5.12.min.js":                     "https://unpkg.com/rxjs@5.5.12/bundles/Rx.min.js",
	"rxjs-7.8.1.umd.min.js":                  "https://unpkg.com/rxjs@7.8.1/dist/bundles/rxjs.umd.min.js",
	"sqljs-1.11.0.sql-wasm.js":               "https://unpkg.com/sql.js@1.11.0/dist/sql-wasm.js",
	"store-1.3.20.min.js":                    "https://cdnjs.cloudflare.com/ajax/libs/store.js/1.3.20/store.min.js",
	"terser-5.31.1.bundle.min.js":            "https://unpkg.com/terser@5.31.1/dist/bundle.min.js",
	"three-0.152.2.min.js":                   "https://unpkg.com/three@0.152.2/build/three.min.js",
	"underscore-1.13.6.min.js":               "https://cdnjs.cloudflare.com/ajax/libs/underscore.js/1.13.6/underscore-min.js",
	"uuid-8.3.2.min.js":                      "https://cdnjs.cloudflare.com/ajax/libs/uuid/8.3.2/uuid.min.js",
	"vue-3.4.31.global.prod.js":              "https://unpkg.com/vue@3.4.31/dist/vue.global.prod.js",
	"vue-compiler-dom-3.5.13.global.prod.js": "https://unpkg.com/@vue/compiler-dom@3.5.13/dist/compiler-dom.global.prod.js",
	"zone.js-0.14.8.umd.js":                  "https://unpkg.com/zone.js@0.14.8/bundles/zone.umd.js",
}

func TestJavaScriptES5TestFiles(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarES5Path)
	require.NoError(t, err)
	corpus.RunTestFiles(t, matcher, es5TestdataDir, ".js")
}

func TestJavaScriptES2015TestFiles(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarESNextPath)
	require.NoError(t, err)
	corpus.RunTestFiles(t, matcher, es2015TestdataDir, ".js")
}

func TestJavaScriptES2015BackwardCompatES5(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarESNextPath)
	require.NoError(t, err)
	corpus.RunTestFiles(t, matcher, es5TestdataDir, ".js")
}

func TestJavaScriptES5RealWorldLibraries(t *testing.T) {
	realWorldTestdataDir := download(realWorldES5Files)
	defer os.RemoveAll(realWorldTestdataDir)
	matcher, err := langlang.MatcherFromFilePath(grammarES5Path)
	require.NoError(t, err)
	runNamedFiles(t, matcher, realWorldTestdataDir, realWorldES5Files)
}

func TestJavaScriptES2015RealWorldLibraries(t *testing.T) {
	realWorldTestdataDir := download(realWorldES2015Files)
	defer os.RemoveAll(realWorldTestdataDir)
	matcher, err := langlang.MatcherFromFilePath(grammarESNextPath)
	require.NoError(t, err)
	runNamedFiles(t, matcher, realWorldTestdataDir, realWorldES2015Files)
}

func TestJavaScriptES2015TargetedSnippets(t *testing.T) {
	matcher, err := langlang.MatcherFromFilePath(grammarESNextPath)
	require.NoError(t, err)
	tests := []struct {
		name  string
		input string
	}{
		{"empty array literal", "[];\n"},
		{"assign empty array", "x=[];\n"},
		{"in with non-empty array", "u in[1];\n"},
		{"in with empty array minimal", "u in[];\n"},
		{"in with empty array spaced", "u in [];\n"},
		{"in with empty array", "u in[]&&Array(1)[u](function(){c=!1});\n"},
		{"generator function expression", "Float64Array.from(function*(t,n){yield t}(a,b));\n"},
		{"space regex literal", "/ /g;\n"},
		{"regex hash then star", "/# */;\n"},
		{"regex space star hash", "/ *#/;\n"},
		{"regex replace with global flag", "(t+=\"\").replace(/ /g,\"\").toLowerCase();\n"},
		{"regex ending with hash", "n.concat(/# */,n.either(a,t),/ *#/);\n"},
		{"optional chaining property", "e?.length>=2;\n"},
		{"nullish coalescing", "o=t.password??null;\n"},
		{"async function declaration", "async function f(){const i=await t.messageHandler.sendWithPromise(\"x\")}\n"},
		{"class extends dotted name", "class DOMFilterFactory extends s.BaseFilterFactory{#$t;}\n"},
		{"async arrow function", "const s={getLanguage:async()=>\"en-us\"};\n"},
		{"logical or assignment", "f||=typeof Module!=\"undefined\"?Module:{};\n"},
		{"unicode escaped identifier key", "const nG={\\u00C5:\"A\"};\n"},
		{"return expression with leading space", "function t(){return (numeric_separator = true);}\n"},
		{"else if on new line", "if (a) { b(); }\nelse if (c) { d(); }\n"},
		{"adjacent class methods after return object", "class X{a(){return {k:1}}consumeArgs(e,r){if(r){return e}}}\n"},
		{"adjacent function declarations", "function a(){if(x){y()}}function b(g,l){return g+l}\n"},
		{"switch case return with comment", "switch(code){case 95: // _\n  return (numeric_separator = true);\n}\n"},
		{"sqljs wrapper excerpt", "if(typeof exports==='object'&&typeof module==='object'){module.exports=initSqlJs;module.exports.default=initSqlJs;}else if(typeof define==='function'&&define['amd']){define([],function(){return initSqlJs;});}\n"},
		{"binary literal", "const TOK_FLAG_NLB=0b0001;\n"},
		{"bigint literal", "const A=123n;\n"},
		{"function expression arg then object arg", "DEFNODE(\"ClassProperty\",\"static quote\",function AST_ClassProperty(props){if(props){this.key=props.key;}this.flags=0;},{a:1});\n"},
		{"sqljs comment boundary excerpt", "function initSqlJs(){\nreturn initSqlJsPromise;\n} // The end of our initSqlJs function\n\n// This bit below is copied almost exactly from what you get when you use the MODULARIZE=1 flag with emcc\n// However, we don't want to use the emcc modularization. See shell-pre.js\nif (typeof exports === 'object' && typeof module === 'object'){\nmodule.exports = initSqlJs;\nmodule.exports.default = initSqlJs;\n}\nelse if (typeof define === 'function' && define['amd']) {\ndefine([], function() { return initSqlJs; });\n}\n"},
		{"asi after var function with line comment", "var initSqlJs = function(){return 1} // end\nif (ok) { done(); }\n"},
		{"asi after var literal with line comment", "var x = 1 // end\nif (ok) { done(); }\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			corpus.AssertParsesAll(t, matcher, []byte(tt.input), tt.name)
		})
	}
}

func download(files []string) string {
	flag.Parse()
	var destDir string
	if cacheDir, ok := corpus.CorpusCacheDirExpanded(); ok {
		destDir = filepath.Join(cacheDir, "javascript")
		if err := os.MkdirAll(destDir, 0755); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Cache dir:", destDir)
	} else {
		var err error
		destDir, err = os.MkdirTemp("", "download_cases")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Temp dir name:", destDir)
	}

	var (
		client = &http.Client{}
		failed int
	)
	for name, url := range realworldSources {
		if !slices.Contains(files, name) {
			continue
		}
		path := filepath.Join(destDir, name)
		if fi, err := os.Stat(path); err == nil && fi.Size() > 0 {
			continue // already present
		}
		fmt.Printf("Fetching %s ...\n", name)
		resp, err := client.Get(url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  GET %s: %v\n", url, err)
			failed++
			continue
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			fmt.Fprintf(os.Stderr, "  %s: HTTP %d\n", url, resp.StatusCode)
			failed++
			continue
		}
		f, err := os.Create(path)
		if err != nil {
			_ = resp.Body.Close()
			fmt.Fprintf(os.Stderr, "  create %s: %v\n", path, err)
			failed++
			continue
		}
		_, err = io.Copy(f, resp.Body)
		f.Close()
		resp.Body.Close()
		if err != nil {
			os.Remove(path)
			fmt.Fprintf(os.Stderr, "  write %s: %v\n", path, err)
			failed++
			continue
		}
	}
	if failed > 0 {
		os.Exit(1)
	}
	fmt.Printf("Done. Fixtures in %s\n", destDir)

	return destDir
}

func runNamedFiles(t *testing.T, matcher langlang.Matcher, dir string, files []string) {
	t.Helper()
	for _, name := range files {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, name))
			require.NoError(t, err)
			corpus.AssertParsesAll(t, matcher, data, name)
		})
	}
}

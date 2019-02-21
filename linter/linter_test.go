package linter

import (
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	jsonnet "github.com/google/go-jsonnet"
)

var update = flag.Bool("update", false, "update .golden files")

type linterTest struct {
	name   string
	input  string
	output string
}

func runTest(t *testing.T, test *linterTest) {
	read := func(file string) []byte {
		bytz, err := ioutil.ReadFile(file)
		if err != nil {
			t.Fatalf("reading file: %s: %v", file, err)
		}
		return bytz
	}

	input := read(test.input)

	var outBuilder strings.Builder
	errWriter := &ErrorWriter{
		Writer:    &outBuilder,
		Formatter: jsonnet.LinterFormatter(),
	}

	// TODO(sbarzowski) record errorsFound
	RunLint(test.name, string(input), errWriter)

	outData := outBuilder.String()

	if *update {
		changed, err := jsonnet.WriteFile(test.output, []byte(outData), 0666)
		if err != nil {
			t.Error(err)
		}
		if changed {
			// TODO(sbarzowski) gather and print all changed goldens
		}
	} else {
		golden, err := ioutil.ReadFile(test.output)
		if err != nil {
			t.Error(err)
			return
		}
		if diff, hasDiff := jsonnet.CompareGolden(outData, golden); hasDiff {
			t.Error(fmt.Errorf("golden file %v has diff:\n%v", test.input, diff))
		}
	}
}
func TestLinter(t *testing.T) {
	flag.Parse()

	var tests []*linterTest

	match, err := filepath.Glob("testdata/*.jsonnet")
	if err != nil {
		t.Fatal(err)
	}

	jsonnetExtRE := regexp.MustCompile(`\.jsonnet$`)

	for _, input := range match {
		fmt.Println(input)
		// Skip escaped filenames.
		if strings.ContainsRune(input, '%') {
			continue
		}
		name := jsonnetExtRE.ReplaceAllString(input, "")
		golden := jsonnetExtRE.ReplaceAllString(input, ".golden")
		tests = append(tests, &linterTest{
			name:   name,
			input:  input,
			output: golden,
		})
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runTest(t, test)
		})
	}
}

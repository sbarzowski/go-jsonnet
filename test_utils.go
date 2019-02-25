package jsonnet

import (
	"bytes"
	"io/ioutil"
	"os"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// TODO(sbarzowski) move to internal package or something

func Diff(a, b string) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(a, b, false)
	return dmp.DiffPrettyText(diffs)
}

func CompareGolden(result string, golden []byte) (string, bool) {
	if bytes.Compare(golden, []byte(result)) != 0 {
		// TODO(sbarzowski) better reporting of differences in whitespace
		// missing newline issues can be very subtle now
		return Diff(result, string(golden)), true
	}
	return "", false
}

func WriteFile(path string, content []byte, mode os.FileMode) (changed bool, err error) {
	old, err := ioutil.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if bytes.Compare(old, content) == 0 && !os.IsNotExist(err) {
		return false, nil
	}
	if err := ioutil.WriteFile(path, content, mode); err != nil {
		return false, err
	}
	return true, nil
}

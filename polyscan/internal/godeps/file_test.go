package godeps

import (
	"reflect"
	"testing"
)

func TestParseFile(t *testing.T) {
	info, err := parseFile([]byte(`package model

import "fmt"

import (
	"strings"
	_ "embed"
	. "math"
	alias "example.com/x/y"
	` + "`example.com/raw`" + `
)

type Store interface{ Get() }
type Record struct{}
type hidden interface{}
type Alias = Record
type (
	Reader interface{ Read() }
	name   struct{}
)

var _ = fmt.Sprint(strings.ToUpper(alias.X), Pi)
`))
	if err != nil {
		t.Fatal(err)
	}
	want := &fileInfo{
		Package:            "model",
		Imports:            []string{"fmt", "strings", "embed", "math", "example.com/x/y", "example.com/raw"},
		ExportedTypes:      3,
		ExportedInterfaces: 2,
	}
	if !reflect.DeepEqual(info, want) {
		t.Errorf("got %+v, want %+v", info, want)
	}
}

func TestParseFileWithoutPackageClause(t *testing.T) {
	if _, err := parseFile([]byte("func main() {}\n")); err == nil {
		t.Error("expected an error for a file without a package clause")
	}
}

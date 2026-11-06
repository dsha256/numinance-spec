// Command annotationlint checks the governance annotations of a binary
// FileDescriptorSet and prints one line per violation.
//
// Usage:
//
//	buf build --exclude-imports --as-file-descriptor-set -o image.binpb
//	annotationlint [-prefix proto/] image.binpb
//
// Exit status: 0 when clean, 1 on violations, 2 on a usage or read error.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/numinance/numinance-spec/sdk/go/internal/annotationlint"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("annotationlint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	prefix := fs.String("prefix", "", "prepended to every file name in the output, such as the module directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		_, _ = fmt.Fprintln(stderr, "usage: annotationlint [-prefix dir/] descriptor-set.binpb")
		return 2
	}
	f, err := os.Open(fs.Arg(0))
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	defer func() { _ = f.Close() }() // read-only; a close error loses nothing
	vs, err := annotationlint.Lint(f)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	for _, v := range vs {
		v.File = *prefix + v.File
		_, _ = fmt.Fprintln(stdout, v)
	}
	if len(vs) > 0 {
		return 1
	}
	return 0
}

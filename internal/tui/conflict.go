package tui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

type Resolution int

const (
	ResolveOverwrite Resolution = iota
	ResolveAbort
	ResolveShowDiff
)

type Entry struct {
	Name string
	Path string
}

var isTTY = func(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func PromptConflict(existing, incoming Entry, out io.Writer, in io.Reader) (Resolution, error) {
	br, ok := in.(*bufio.Reader)
	if !ok {
		if !isTTY(in) {
			return ResolveAbort, nil
		}
		br = bufio.NewReader(in)
	}
	for {
		fmt.Fprintf(out, "conflict on %q: overwrite/abort/diff? [o/a/d] ", existing.Name)
		line, err := br.ReadString('\n')
		if err != nil && line == "" {
			return ResolveAbort, nil
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "o", "overwrite":
			return ResolveOverwrite, nil
		case "a", "abort", "":
			return ResolveAbort, nil
		case "d", "diff":
			fmt.Fprintf(out, "  existing: %s\n  incoming: %s\n", existing.Path, incoming.Path)
			continue
		default:
			fmt.Fprintf(out, "unrecognized choice %q; try o, a, or d\n", strings.TrimSpace(line))
		}
	}
}

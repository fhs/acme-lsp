package text

import (
	"bufio"
	"io"
	"unicode/utf8"
)

// TODO(fhs): Maybe replace this with https://godoc.org/golang.org/x/tools/internal/span ?
// LSP deals with UTF-16, but acme deals with runes, so out implementation here
// may not be entirely accurate.

type nlOffsets struct {
	nl       []int // rune offsets of '\n'
	leftover int   // runes leftover after last '\n'
}

func GetNewlineOffsets(r io.Reader) (*nlOffsets, error) {
	br := bufio.NewReader(r)
	o := 0
	nl := []int{0}
	leftover := 0
	for {
		line, err := br.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, err
		}
		if err == io.EOF {
			leftover = len(line)
			break
		}
		o += utf8.RuneCountInString(line)
		nl = append(nl, o)
	}
	return &nlOffsets{
		nl:       nl,
		leftover: leftover,
	}, nil
}

// LineToOffset returns the rune offset within the file given the
// line number and rune offset within the line.
func (off *nlOffsets) LineToOffset(line, col int) int {
	eof := off.nl[len(off.nl)-1] + off.leftover
	if line >= len(off.nl) {
		// beyond EOF, so just return the highest offset
		return eof
	}
	o := off.nl[line] + col
	if o > eof {
		o = eof
	}
	return o
}

// OffsetToLine returns the line number and rune offset within the line
// given rune offset within the file.
func (off *nlOffsets) OffsetToLine(offset int) (line, col int) {
	for i, o := range off.nl {
		if o > offset {
			return i - 1, offset - off.nl[i-1]
		}
	}
	if i := len(off.nl) - 1; offset >= off.nl[i] {
		return i, offset - off.nl[i]
	}
	panic("unreachable")
}

package screen

import (
	"bytes"
	"fmt"
	"strings"
)

type Screen struct {
	buffer    bytes.Buffer
	lineCount int
	width     int
	pad       string
}

func (s *Screen) Println(a ...interface{}) {
	fmt.Fprintln(s, a...)
	s.lineCount++
}

func (s *Screen) Write(dt []byte) (int, error) {
	return s.buffer.Write(dt)
}

func (s *Screen) Read(p []byte) (n int, err error) {
	return s.buffer.Read(p)
}

func Pad(s *Screen, line string) string {
	lines := strings.Split(line, "\n")
	for i, l := range lines {
		lines[i] = fmt.Sprintf(s.pad+"%s", l)
	}
	return strings.Join(lines, "\n")
}

func Align(s *Screen, left, right string) string {
	return fmt.Sprint(left, strings.Repeat(" ", s.width-len(left)-len(right)), right)
}

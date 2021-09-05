package screen

import (
	"fmt"
	"io"
	"sync"

	"github.com/containerd/console"
	"github.com/morikuni/aec"
	"github.com/ruudk/tailbox/vt100"
)

type Display struct {
	sync.Mutex

	c         console.Console
	width     int
	height    int
	pad       string
	lineCount int
}

func NewDisplay(f console.File, height int) (*Display, error) {
	c, err := console.ConsoleFromFile(f)
	if err != nil {
		return nil, err
	}

	size, err := c.Size()
	width := 80
	if err == nil && size.Width > 0 {
		width = int(size.Width)
	}

	d := &Display{
		c:      c,
		width:  width,
		height: height,
		pad:    "=> => ",
	}

	return d, nil
}

func (d *Display) NewScreen() *Screen {
	return &Screen{
		width: d.width,
		pad:   d.pad,
	}
}

func (d *Display) Snapshot() {
	d.Lock()
	defer d.Unlock()

	d.lineCount = 0
}

func (d *Display) Draw(screen *Screen) {
	d.Lock()
	defer d.Unlock()

	fmt.Fprint(d.c, aec.Hide)
	defer fmt.Fprint(d.c, aec.Show)

	d.erase()

	io.Copy(d.c, screen)

	d.lineCount = screen.lineCount
}

func (d *Display) NewTerm() *vt100.VT100 {
	return vt100.NewVT100(1000, d.width-len(d.pad))
}

func (d *Display) erase() {
	if d.lineCount == 0 {
		return
	}

	b := aec.EmptyBuilder
	for i := 0; i < d.lineCount; i++ {
		b = b.Up(1).EraseLine(aec.EraseModes.All)
	}
	b = b.Column(0)

	fmt.Fprint(d.c, b.ANSI)

	d.lineCount = 0
}

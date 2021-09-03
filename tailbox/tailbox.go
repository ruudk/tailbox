package tailbox

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/containerd/console"
	"github.com/morikuni/aec"
	"github.com/tonistiigi/vt100"
)

type Status int

const (
	Running Status = iota
	Completed
	Failed
)

type Tailbox struct {
	c                                                             console.Console
	term                                                          *vt100.VT100
	errTerm                                                       *vt100.VT100
	termWriter                                                    io.Writer
	width                                                         int
	height                                                        int
	pad                                                           string
	padLen                                                        int
	lineCount                                                     int
	headerMessage, runningMessage, successMessage, failureMessage string
	status                                                        Status
	ticker                                                        *time.Ticker
	refresherCtx                                                  context.Context
	refreshCanceller                                              context.CancelFunc
	wg                                                            *sync.WaitGroup
	startTime                                                     time.Time
}

func NewTailbox(f console.File, numberOfLines int, headerMessage, runningMessage, successMessage, failureMessage string) (*Tailbox, error) {
	ticker := time.NewTicker(150 * time.Millisecond)

	c, err := console.ConsoleFromFile(f)
	if err != nil {
		return nil, err
	}

	size, err := c.Size()
	TermHeight := numberOfLines
	TermWidth := 80
	if err == nil && size.Width > 0 {
		TermWidth = int(size.Width)
	}
	pad := "=> "
	padLen := len(pad)

	var wg sync.WaitGroup
	refresherCtx, refreshCanceller := context.WithCancel(context.Background())

	term := vt100.NewVT100(TermHeight, TermWidth-padLen)
	errTerm := vt100.NewVT100(100, TermWidth-padLen)
	d := Tailbox{
		c,
		term,
		errTerm,
		io.MultiWriter(term, errTerm),
		TermWidth,
		TermHeight,
		pad,
		padLen,
		0,
		headerMessage,
		runningMessage,
		successMessage,
		failureMessage,
		Running,
		ticker,
		refresherCtx,
		refreshCanceller,
		&wg,
		time.Now(),
	}

	go d.refresher()

	return &d, nil
}

func (tb *Tailbox) Write(dt []byte) (int, error) {
	return tb.termWriter.Write(dt)
}

func (tb *Tailbox) destroy() {
	tb.ticker.Stop()
	tb.refreshCanceller()
	tb.wg.Wait()
}

func (tb *Tailbox) Success() {
	tb.status = Completed
	tb.destroy()
}

func (tb *Tailbox) Fail(err error) {
	tb.status = Failed
	tb.destroy()
	fmt.Println(aec.Apply(err.Error(), aec.RedF))
}

func (tb *Tailbox) refresher() {
	tb.wg.Add(1)

	defer tb.wg.Done()

	for {
		select {
		case <-tb.refresherCtx.Done():
			tb.update()
			return
		case <-tb.ticker.C:
			tb.update()
		}
	}
}

func (tb *Tailbox) update() {
	b := aec.EmptyBuilder.Column(0)
	if tb.lineCount > 0 {
		b = b.Up(uint(tb.lineCount))
	}

	fmt.Fprint(tb.c, aec.Hide)
	defer fmt.Fprint(tb.c, aec.Show)

	fmt.Fprint(tb.c, b.ANSI)

	lineCount := 0

	var header string
	var color aec.ANSI
	switch tb.status {
	case Running:
		header = tb.runningMessage
		color = aec.BlueF
	case Completed:
		header = tb.successMessage
		color = aec.GreenF
	case Failed:
		header = tb.failureMessage
		color = aec.RedF
	}
	if tb.headerMessage != "" {
		header = tb.headerMessage
	}
	if header != "" {
		lineCount++
		fmt.Fprintln(tb.c, aec.Apply(align(header, fmt.Sprintf("%.1fs", time.Since(tb.startTime).Seconds()), tb.width), color))
	}

	term := tb.term
	if tb.status == Failed {
		term = tb.errTerm
	}

	if tb.status != Completed {
		color = aec.Faint
		if tb.status == Failed {
			color = aec.RedF
		}
		for _, line := range term.Content {
			if !isEmpty(line) {
				out := aec.Apply(fmt.Sprintf(tb.pad+"%s\n", string(line)), color)
				fmt.Fprint(tb.c, out)
				lineCount++
			}
		}
	}

	if lines := tb.lineCount - lineCount; lines > 0 {
		tb.blank(lines)
	}
	tb.lineCount = lineCount
}

func (tb *Tailbox) blank(lines int) {
	for i := 0; i < lines; i++ {
		fmt.Fprintln(tb.c, strings.Repeat(" ", tb.width))
	}
	fmt.Fprint(tb.c, aec.EmptyBuilder.Up(uint(lines)).Column(0).ANSI)
}

func isEmpty(line []rune) bool {
	for _, r := range line {
		if r != ' ' {
			return false
		}
	}
	return true
}

func align(l, r string, w int) string {
	return fmt.Sprint(l, strings.Repeat(" ", w-len(l)-len(r)), r)
}

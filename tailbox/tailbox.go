package tailbox

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/mattn/go-tty"
	"github.com/ruudk/tailbox/tailbox/screen"
)

type State int

const (
	Pending State = iota
	Running
	Stopping
	Stopped
	Completed
	Failing
	Failed
	Terminated
)

func (s State) String() string {
	switch s {
	case Pending:
		return "pending"
	case Running:
		return "running"
	case Stopping:
		return "stopping"
	case Stopped:
		return "stopped"
	case Completed:
		return "completed"
	case Failing:
		return "failing"
	case Failed:
		return "failed"
	case Terminated:
		return "terminated"
	default:
		log.Fatalf("Unknown state %#v", s)
		return ""
	}
}

type Tailbox struct {
	sync.RWMutex
	config    *Config
	display   *screen.Display
	state     State
	workflows []*Workflow
	workflow  *Workflow
	startTime time.Time
	tty       *tty.TTY
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func NewTailbox(display *screen.Display, config *Config) (*Tailbox, error) {
	ctx, cancel := context.WithCancel(context.Background())

	tb := Tailbox{
		config:    config,
		display:   display,
		state:     Pending,
		workflows: make([]*Workflow, 0),
		ctx:       ctx,
		cancel:    cancel,
	}

	for _, wf := range config.Workflows {
		workflow := wf
		tb.workflows = append(tb.workflows, NewWorkflow(&tb, &workflow))
	}

	return &tb, nil
}

func (tb *Tailbox) Start() {
	tb.startTime = time.Now()

	go func() {
		ticker := time.NewTicker(150 * time.Millisecond)

		for {
			select {
			case <-tb.ctx.Done():
				log.Println("Stopping refresher goroutine")
				return
			case <-ticker.C:
				tb.draw()
			}
		}
	}()

	tty, err := tty.Open()
	if err == nil {
		tb.tty = tty
		go func() {
			for {
				// TODO: add context stop
				r, err := tty.ReadRune()
				if err != nil {
					log.Printf("failed reading rune from tty: %s", err)
				}
				log.Println("Key press => " + string(r))
				if r == 'd' {
					log.Println("key press d > draw")
					tb.draw()
				}
			}
		}()
	} else {
		log.Printf("cannot open tty: %s", err)
	}

	interruptCtx := InterruptListener(tb.ctx)

	wfChan := make(chan *Workflow)
	go func() {
		for _, wf := range tb.workflows {
			if wf.config.When != WhenAlways {
				continue
			}

			select {
			case <-interruptCtx.Done():
				log.Println("Closing wfChan")
				close(wfChan)
				return
			case wfChan <- wf:
				log.Println("Wrote wf to wfChan")
			}
		}

		log.Println("End of workflow producer to wfchan")
		close(wfChan)
	}()

workflowLoop:
	for {
		select {
		case <-interruptCtx.Done():
			log.Println("Interrupt context canceled")
			break workflowLoop
		case wf := <-wfChan:
			if wf == nil {
				break workflowLoop
			}
			tb.draw()
			tb.display.Snapshot()

			tb.runWorkflow(wf)

			if wf.state == Failed {
				tb.state = Failing
			}
		}
	}

	tb.Lock()
	tb.state = Stopping
	tb.Unlock()

	tb.draw()

	if tb.workflow != nil && tb.workflow.state == Running {
		tb.stopWorkflow(tb.workflow)
		tb.draw()
	}

	tb.draw()
	tb.resetWorkflow()

	interruptCtx = InterruptListener(tb.ctx)

	shutdownWfChan := make(chan *Workflow)
	go func() {
		for _, wf := range tb.workflows {
			if wf.config.When != WhenShutdown {
				continue
			}

			select {
			case <-interruptCtx.Done():
				close(shutdownWfChan)
				return
			case shutdownWfChan <- wf:
			}
		}

		close(shutdownWfChan)
	}()

shutdownLoop:
	for {
		select {
		case <-interruptCtx.Done():
			log.Println("Interrupt context canceled")
			break shutdownLoop
		case wf := <-shutdownWfChan:
			if wf == nil {
				break shutdownLoop
			}

			tb.draw()
			tb.display.Snapshot()

			tb.runWorkflow(wf)
		}
	}

	tb.Lock()
	tb.state = Stopped
	tb.Unlock()

	tb.draw()

	if tb.tty != nil {
		tb.tty.Close()
	}

	log.Println("End of Start")
}

func InterruptListener(ctx context.Context) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("Stopping signal notify goroutine")
				return
			case <-sig:
				log.Println("Received SIGINT, stopping")
				cancel()
				return
			}
		}
	}()

	return ctx
}

func (tb *Tailbox) runWorkflow(wf *Workflow) {
	log.Printf("Running workflow %s", wf.config.Name)

	tb.Lock()
	tb.workflow = wf
	tb.Unlock()

	wf.Lock()
	wf.state = Running
	wf.Unlock()

	for _, st := range wf.steps {
		if wf.state == Failing && st.When() != WhenAlways {
			continue
		}

		wf.Lock()
		wf.step = st
		wf.Unlock()

		st.Start()

	}

	wf.Lock()
	wf.step = nil
	wf.Unlock()

	if wf.state == Failing {
		wf.Lock()
		wf.state = Failed
		wf.Unlock()
		return
	}

	wf.Lock()
	wf.state = Completed
	wf.Unlock()
}

func (tb *Tailbox) stopWorkflow(wf *Workflow) {
	log.Println("stopWorflow, trying to get lock")
	wf.Lock()
	log.Println("Log acquired")
	defer wf.Unlock()

	if wf.state == Stopped {
		return
	}

	log.Printf("Stopping workflow %s", wf.config.Name)

	wf.state = Stopping
	if wf.step != nil {
		wf.step.Stop()
	}
	wf.state = Stopped
}

func (tb *Tailbox) resetWorkflow() {
	tb.Lock()
	defer tb.Unlock()

	tb.workflow = nil
}

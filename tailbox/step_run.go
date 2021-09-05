package tailbox

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ruudk/tailbox/vt100"
)

type RunStep struct {
	sync.RWMutex

	config    RunStepConfig
	term      *vt100.VT100
	state     State
	startTime time.Time
	stopTime  time.Time
	err       error
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	exitCode  int
}

func NewRunStep(ctx context.Context, config RunStepConfig, term *vt100.VT100) *RunStep {
	ctx, cancel := context.WithCancel(ctx)

	return &RunStep{
		config: config,
		term:   term,
		state:  Pending,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (cs *RunStep) Name() string {
	return cs.config.Name
}

func (cs *RunStep) Term() *vt100.VT100 {
	return cs.term
}

func (cs *RunStep) State() State {
	return cs.state
}

func (cs *RunStep) When() string {
	return cs.config.When
}

func (cs *RunStep) Lines() int {
	return cs.config.Lines
}

func (cs *RunStep) Duration() *time.Duration {
	if cs.startTime.IsZero() {
		return nil
	}

	if !cs.stopTime.IsZero() {
		duration := cs.stopTime.Sub(cs.startTime)
		return &duration
	}

	duration := time.Since(cs.startTime)
	return &duration
}

func (cs *RunStep) Write(dt []byte) (int, error) {
	cs.Lock()
	defer cs.Unlock()

	return cs.term.Write(dt)
}

func (cs *RunStep) Start() {
	cs.Lock()

	cs.state = Running
	cs.startTime = time.Now()

	cs.wg.Add(1)
	go func() {
		defer cs.wg.Done()
		cs.run()
	}()

	if cs.config.Background {
		cs.Unlock()
		return
	}

	cs.Unlock()
	cs.wg.Wait()
}

func (cs *RunStep) run() {
	cmd := exec.Command("/bin/bash", "-c", cs.config.Command)
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "TERM=dumb")
	cmd.Stdout = cs
	cmd.Stderr = cs
	err := cmd.Start()
	if err != nil {
		cs.Lock()
		defer cs.Unlock()
		log.Printf("[step.fail] Step %s failed: Command could not be started: %s", cs.config.Name, err)

		cs.term.Write([]byte("Command could not be started:\n"))

		cs.term.Write([]byte(Indent(cs.config.Command, "  ") + "\n"))
		cs.state = Failed
		cs.stopTime = time.Now()
		cs.err = err

		return
	}

	log.Printf("[step.run] stop wait seconds %d", cs.config.StopTimeout)
	err = waitOrStop(cs.ctx, cmd, cs.config.StopSignal, cs.config.StopTimeout)
	log.Printf("[step.run] command waitOrStop returned %s", err)

	if err != nil {
		if err.Error() == "signal: interrupt" {
			cs.Lock()
			defer cs.Unlock()

			log.Printf("[step.interrupt] Step %s interrupted: %s", cs.config.Name, err)

			cs.term.Write([]byte("Command stopped:\n"))
			cs.term.Write([]byte(Indent(cs.config.Command, "  ") + "\n"))

			cs.state = Stopped
			cs.stopTime = time.Now()
			cs.err = err

			return
		}

		cs.Lock()
		defer cs.Unlock()

		if werr, ok := err.(*exec.ExitError); ok {
			cs.exitCode = werr.ExitCode()
			log.Printf("[step.fail] RunStep %s failed with exit code %d: %s", cs.config.Name, cs.exitCode, err)
		} else {
			log.Printf("[step.fail] RunStep %s failed: %s", cs.config.Name, err)
		}

		if cs.exitCode != -1 {
			cs.term.Write([]byte(fmt.Sprintf("Command failed with exit code %d:\n", cs.exitCode)))
		} else {
			cs.term.Write([]byte("Command failed:\n"))
		}

		cs.term.Write([]byte(Indent(cs.config.Command, "  ") + "\n"))
		cs.state = Failed
		cs.stopTime = time.Now()
		cs.err = err

		return
	}

	cs.Lock()
	defer cs.Unlock()

	log.Printf("[step.success] Step %s success", cs.config.Name)
	cs.state = Completed
	cs.stopTime = time.Now()
}

func (cs *RunStep) Stop() {
	cs.Lock()
	defer cs.Unlock()
	if cs.state == Stopping || cs.state == Stopped {
		log.Printf("[step.stop] Already %s", cs.state)
		return
	}

	log.Printf("[step.stop] Stopping step %s", cs.config.Name)

	cs.state = Stopping

	go func() {
		cs.cancel()
		cs.wg.Wait()

		cs.Lock()
		cs.state = Stopped
		cs.Unlock()

		log.Printf("[step.stop] Step %s stopped", cs.config.Name)
	}()
}

func (cs *RunStep) Close() error {
	return nil
}

func Indent(line string, indentation string) string {
	lines := strings.Split(line, "\n")
	for i, l := range lines {
		lines[i] = indentation + l
	}
	return strings.Join(lines, "\n")
}

func waitOrStop(ctx context.Context, cmd *exec.Cmd, interrupt os.Signal, killDelay time.Duration) error {
	log.Printf("[waitOrStop] kill delay %d", killDelay)

	errc := make(chan error)
	go func() {
		select {
		case errc <- nil:
			return
		case <-ctx.Done():
		}

		if killDelay == 0 {
			log.Println("[waitOrStop] Kill process")

			err := cmd.Process.Kill()

			log.Printf("[waitOrStop] Process killed: %s", err)

			errc <- err
			return
		}

		log.Printf("[waitOrStop] Sending signal %s to process", interrupt)
		err := cmd.Process.Signal(interrupt)
		if err == nil {
			err = ctx.Err() // Report ctx.Err() as the reason we interrupted.
		} else if err.Error() == "os: process already finished" {
			log.Printf("[waitOrStop] process already finished")
			errc <- nil
			return
		}

		log.Printf("[waitOrStop] Ctx err %s", ctx.Err())

		log.Printf("[waitOrStop] Waiting %d before killing", killDelay)
		select {
		// Report ctx.Err() as the reason we interrupted the process...
		case errc <- ctx.Err():
			log.Printf("[waitOrStop] Stop wait loop because ctx.err %s", ctx.Err())

			return
		// ...but after killDelay has elapsed, fall back to a stronger signal.
		case <-time.After(time.Duration(killDelay) * time.Second):
		}

		log.Println("[waitOrStop] Kill process")
		// Wait still hasn't returned.
		// Kill the process harder to make sure that it exits.
		//
		// Ignore any error: if cmd.Process has already terminated, we still
		// want to send ctx.Err() (or the error from the Interrupt call)
		// to properly attribute the signal that may have terminated it.
		_ = cmd.Process.Kill()

		errc <- err
	}()

	log.Println("[waitOrStop] Wait on process")
	waitErr := cmd.Wait()
	if interruptErr := <-errc; interruptErr != nil {
		log.Printf("[waitOrStop] interruptErr: %s", interruptErr)

		return interruptErr
	}

	if waitErr == nil {
		log.Println("[waitOrStop] stopped without error")

		return nil
	}

	log.Printf("[waitOrStop] waitErr: %s", waitErr)

	return waitErr
}

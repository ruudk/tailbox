package tailbox

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/ruudk/tailbox/vt100"
)

const chmodMask fsnotify.Op = ^fsnotify.Op(0) ^ fsnotify.Chmod

type WatcherStep struct {
	sync.RWMutex

	config    WatcherStepConfig
	term      *vt100.VT100
	state     State
	startTime time.Time
	stopTime  time.Time
	err       error
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	exitCode  int
	watcher   *fsnotify.Watcher
}

func NewWatcherStep(ctx context.Context, config WatcherStepConfig, term *vt100.VT100) *WatcherStep {
	ctx, cancel := context.WithCancel(ctx)

	return &WatcherStep{
		config: config,
		term:   term,
		state:  Pending,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (ws *WatcherStep) Name() string {
	return ws.config.Name
}

func (ws *WatcherStep) Term() *vt100.VT100 {
	return ws.term
}

func (ws *WatcherStep) State() State {
	return ws.state
}

func (ws *WatcherStep) When() string {
	return WhenAlways
}

func (ws *WatcherStep) Lines() int {
	// TODO
	return -1
}

func (ws *WatcherStep) Duration() *time.Duration {
	if ws.startTime.IsZero() {
		return nil
	}

	if !ws.stopTime.IsZero() {
		duration := ws.stopTime.Sub(ws.startTime)
		return &duration
	}

	duration := time.Since(ws.startTime)
	return &duration
}

func (ws *WatcherStep) Write(dt []byte) (int, error) {
	ws.Lock()
	defer ws.Unlock()

	return ws.term.Write(dt)
}

func (ws *WatcherStep) Start() {
	ws.Lock()
	defer ws.Unlock()

	ws.state = Running
	ws.startTime = time.Now()

	var err error
	ws.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		ws.fail(fmt.Errorf("cannot start watcher: %w", err))
		return
	}

	for _, g := range ws.config.Globs {
		err = ws.watcher.Add(g.Directory)
		if err != nil {
			ws.fail(fmt.Errorf("cannot add glob %s to watcher: %w", g, err))
			return
		}
	}

	ws.wg.Add(1)
	go func() {
		defer ws.wg.Done()
		for {
			select {
			case event, ok := <-ws.watcher.Events:
				if !ok {
					return
				}

				if event.Op&chmodMask == 0 {
					continue
				}

				matched, pattern, path := ws.matches(event.Name)
				if !matched {
					continue
				}

				log.Printf("Run workflow because %s matches %s", path, pattern)
				ws.Write([]byte(fmt.Sprintf("Run workflow because %s matches %s\n", path, pattern)))
			case err, ok := <-ws.watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()
}

func (ws *WatcherStep) matches(path string) (bool, string, string) {
	for _, g := range ws.config.Globs {
		if !strings.HasPrefix(path, g.Directory) {
			continue
		}

		p := path[len(g.Directory)+1:]
		matches, _ := filepath.Match(g.Pattern, p)
		if matches {
			return true, g.Pattern, p
		}
	}

	return false, "", ""
}

func (ws *WatcherStep) fail(err error) {
	ws.term.Write([]byte(err.Error() + "\n"))
	ws.state = Failed
	ws.stopTime = time.Now()
	ws.err = err
}

func (ws *WatcherStep) Stop() {

}

func (ws *WatcherStep) Close() error {
	ws.cancel()

	err := ws.watcher.Close()
	if err != nil {
		return err
	}

	return nil
}

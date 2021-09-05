package tailbox

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/ruudk/tailbox/vt100"
)

type WaitStep struct {
	sync.RWMutex

	config    WaitStepConfig
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

func NewWaitStep(ctx context.Context, config WaitStepConfig, term *vt100.VT100) *WaitStep {
	ctx, cancel := context.WithCancel(ctx)

	return &WaitStep{
		config: config,
		term:   term,
		state:  Pending,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (ws *WaitStep) Name() string {
	return ws.config.Name
}

func (ws *WaitStep) Term() *vt100.VT100 {
	return ws.term
}

func (ws *WaitStep) State() State {
	return ws.state
}

func (ws *WaitStep) When() string {
	return WhenAlways
}

func (ws *WaitStep) Lines() int {
	// TODO
	return -1
}

func (ws *WaitStep) Duration() *time.Duration {
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

func (ws *WaitStep) Write(dt []byte) (int, error) {
	ws.Lock()
	defer ws.Unlock()

	return ws.term.Write(dt)
}

func (ws *WaitStep) Start() {
	ws.Lock()
	ws.state = Running
	ws.startTime = time.Now()

	ws.wg.Add(1)
	go func() {
		defer ws.wg.Done()

		ws.waitForUrl(
			ws.config.URL,
			ws.config.Timeout,
			ws.config.Interval,
		)

		ws.Lock()
		defer ws.Unlock()

		log.Printf("[step.success] Step %s success", ws.config.Name)
		ws.state = Completed
		ws.stopTime = time.Now()
	}()

	ws.Unlock()
	ws.wg.Wait()
}

func (ws *WaitStep) Stop() {
	ws.Lock()
	defer ws.Unlock()

	log.Printf("[step.stop] Stopping step %s", ws.config.Name)

	if ws.state == Stopping || ws.state == Stopped {
		log.Printf("[step.stop] Already %s", ws.state)

		return
	}

	ws.state = Stopping

	go func() {
		ws.cancel()
		ws.wg.Wait()

		ws.Lock()
		ws.state = Stopped
		ws.Unlock()

		log.Printf("[step.stop] RunStep %s stopped", ws.config.Name)
	}()
}

func (ws *WaitStep) Close() error {
	return nil
}

// Copyright (c) 2014-2018 Jason Wilder
// https://github.com/jwilder/dockerize
func (ws *WaitStep) waitForUrl(u url.URL, timeout, interval time.Duration) {
	var wg sync.WaitGroup
	doneChan := make(chan struct{})

	go func() {
		switch u.Scheme {
		case "file":
			wg.Add(1)
			go func(u url.URL) {
				defer wg.Done()
				ticker := time.NewTicker(interval)
				defer ticker.Stop()
				var err error
				if _, err = os.Stat(u.Path); err == nil {
					ws.Write([]byte(fmt.Sprintf("File %s found\n", u.Path)))
					return
				}
				for {
					select {
					case <-ws.ctx.Done():
						return
					case <-ticker.C:
					}
					if _, err = os.Stat(u.Path); err == nil {
						ws.Write([]byte(fmt.Sprintf("File %s found\n", u.Path)))
						return
					} else if os.IsNotExist(err) {
						ws.Write([]byte(fmt.Sprintf("File %s does not exist yet\n", u.Path)))
						continue
					} else {
						ws.Write([]byte(fmt.Sprintf("Problem with check file %s exist: %v. Sleeping %s\n", u.Path, err.Error(), interval)))
					}

				}
			}(u)
		case "tcp", "tcp4", "tcp6", "unix":
			wg.Add(1)

			go func() {
				defer wg.Done()
				for {
					host := u.Host
					if u.Scheme == "unix" {
						host = u.Path
					}
					conn, err := net.DialTimeout(u.Scheme, host, timeout)
					if ws.ctx.Err() != nil {
						return
					}
					if err != nil {
						ws.Write([]byte(fmt.Sprintf("Problem with dial: %v. Sleeping %s\n", err.Error(), interval)))
						time.Sleep(interval)
					}
					if conn != nil {
						ws.Write([]byte(fmt.Sprintf("Connected to %s://%s\n", u.Scheme, host)))
						return
					}
				}
			}()
		case "http", "https":
			wg.Add(1)
			go func(u url.URL) {
				defer wg.Done()

				client := &http.Client{
					Timeout: timeout,
				}
				for {
					req, err := http.NewRequestWithContext(ws.ctx, "GET", u.String(), nil)
					if err != nil {
						ws.Write([]byte(fmt.Sprintf("Problem with dial: %v. Sleeping %s\n", err.Error(), interval)))
						time.Sleep(interval)
					}
					// TODO
					//if len(headers) > 0 {
					//	for _, header := range headers {
					//		req.Header.Add(header.name, header.value)
					//	}
					//}

					resp, err := client.Do(req)
					if resp != nil {
						io.Copy(io.Discard, resp.Body)
						resp.Body.Close()
					}
					if err != nil {
						ws.Write([]byte(fmt.Sprintf("Problem with request: %s. Sleeping %s\n", err.Error(), interval)))
						time.Sleep(interval)
					} else if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
						ws.Write([]byte(fmt.Sprintf("Received %d from %s\n", resp.StatusCode, u.String())))
						return
					} else {
						ws.Write([]byte(fmt.Sprintf("Received %d from %s. Sleeping %s\n", resp.StatusCode, u.String(), interval)))
						time.Sleep(interval)
					}
				}
			}(u)
		default:
			log.Fatalf("Invalid protocol %s", u.Scheme)
		}

		wg.Wait()
		close(doneChan)
	}()

	select {
	case <-doneChan:
		break
	case <-time.After(timeout):
		ws.Write([]byte(fmt.Sprintf("Timeout after %s waiting on dependencies to become available", timeout)))
	}

}

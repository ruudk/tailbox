package tailbox

import (
	"time"

	"github.com/ruudk/tailbox/vt100"
)

type Awaitable interface {
	Await()
}

type Step interface {
	Name() string
	Term() *vt100.VT100
	State() State
	When() string
	Lines() int
	Duration() *time.Duration
	Start()
	Stop()
	Close() error
	RLock()
	RUnlock()
}

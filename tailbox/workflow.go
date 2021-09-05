package tailbox

import (
	"log"
	"sync"
)

type Workflow struct {
	sync.RWMutex

	tailbox *Tailbox
	config  *WorkflowConfig
	steps   []Step
	step    Step
	state   State
}

func NewWorkflow(tb *Tailbox, config *WorkflowConfig) *Workflow {
	ctx := tb.ctx
	steps := make([]Step, 0)
	for _, st := range config.Steps {
		term := tb.display.NewTerm()
		switch s := st.(type) {
		case RunStepConfig:
			steps = append(steps, NewRunStep(ctx, s, term))
		case WaitStepConfig:
			steps = append(steps, NewWaitStep(ctx, s, term))
		case WatcherStepConfig:
			steps = append(steps, NewWatcherStep(ctx, s, term))
		default:
			log.Fatalf("Step %T is not yet supported in workflows", st)
		}
	}

	return &Workflow{
		tailbox: tb,
		config:  config,
		steps:   steps,
		state:   Pending,
	}
}

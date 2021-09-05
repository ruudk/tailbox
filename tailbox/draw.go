package tailbox

import (
	"fmt"
	"log"
	"time"

	"github.com/morikuni/aec"
	"github.com/ruudk/tailbox/tailbox/screen"
)

func (tb *Tailbox) draw() {
	tb.RLock()
	defer tb.RUnlock()

	if tb.workflow == nil {
		return
	}

	scrn := tb.display.NewScreen()

	tb.workflow.RLock()
	defer tb.workflow.RUnlock()

	var header string
	var color aec.ANSI
	header = fmt.Sprintf("%s (%s)", tb.workflow.config.Name, tb.workflow.state)
	color = aec.EmptyBuilder.BlueB().BlackF().ANSI
	if header != "" {
		scrn.Println(aec.Apply(screen.Align(scrn, header, fmt.Sprintf("%.1fs", time.Since(tb.startTime).Seconds())), color))
	}

	for _, st := range tb.workflow.steps {
		st.RLock()

		if st.State() == Pending {
			st.RUnlock()
			continue
		}

		stepName := fmt.Sprintf("=> %s (%s)", st.Name(), st.State())
		switch st.State() {
		case Pending:
			color = aec.DefaultF
		case Running:
			color = aec.BlueF
			stepName = screen.Align(scrn, stepName, fmt.Sprintf("%.1fs", st.Duration().Seconds()))
		case Stopping:
			color = aec.BlueF
			stepName = screen.Align(scrn, stepName, fmt.Sprintf("%.1fs", st.Duration().Seconds()))
		case Stopped:
			fallthrough
		case Completed:
			color = aec.GreenF
			stepName = screen.Align(scrn, stepName, fmt.Sprintf("%.1fs", st.Duration().Seconds()))
		case Terminated:
			fallthrough
		case Failed:
			color = aec.RedF
			stepName = screen.Align(scrn, stepName, fmt.Sprintf("%.1fs", st.Duration().Seconds()))
		default:
			log.Fatalf("State %s cannot be displayed yet", st.State())
		}

		scrn.Println(aec.Apply(stepName, color))

		color = aec.Faint
		if st.State() == Failed {
			color = aec.RedF
		}

		if st.State() != Completed && st.State() != Stopped && st.State() != Terminated {
			term := st.Term()

			lines := st.Lines()
			if st.State() == Failed {
				lines = -1
			}

			for _, line := range term.Tail(lines) {
				scrn.Println(aec.Apply(screen.Pad(scrn, string(line)), color))
			}
		}

		st.RUnlock()

	}

	tb.display.Draw(scrn)
}

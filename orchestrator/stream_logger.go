package orchestrator

import (
	"fmt"
	"time"
)

// ANSI color codes for stream logger
const (
	streamColorReset  = "\033[0m"
	streamColorGreen  = "\033[32m"
	streamColorRed    = "\033[31m"
	streamColorYellow = "\033[33m"
	streamColorGray   = "\033[90m"
	streamColorCyan   = "\033[36m"
	streamColorBlue   = "\033[34m"
)

// StreamLogger outputs execution events as they happen
type StreamLogger struct {
	showTimestamps bool
	colorized      bool
}

func NewStreamLogger() *StreamLogger {
	return &StreamLogger{
		showTimestamps: true,
		colorized:      true,
	}
}

func NewStreamLoggerWithOptions(showTimestamps, colorized bool) *StreamLogger {
	return &StreamLogger{
		showTimestamps: showTimestamps,
		colorized:      colorized,
	}
}

func (s *StreamLogger) OnWorkflowStart(data WorkflowEventData) {
	timestamp := ""
	if s.showTimestamps {
		timestamp = fmt.Sprintf("[%s] ", data.StartTime.Format("15:04:05"))
	}

	if s.colorized {
		fmt.Printf("%s%s🚀 Workflow started%s (total nodes: %d)\n",
			timestamp, streamColorBlue, streamColorReset, data.TotalNodes)
	} else {
		fmt.Printf("%sWorkflow started (total nodes: %d)\n", timestamp, data.TotalNodes)
	}
}

func (s *StreamLogger) OnWorkflowComplete(data WorkflowEventData) {
	timestamp := ""
	if s.showTimestamps {
		timestamp = fmt.Sprintf("[%s] ", data.EndTime.Format("15:04:05"))
	}

	if data.Error != nil {
		if s.colorized {
			fmt.Printf("\n%s%s✗ Workflow failed%s after %s: %v\n",
				timestamp, streamColorRed, streamColorReset, data.Duration.Round(time.Millisecond), data.Error)
		} else {
			fmt.Printf("\n%sWorkflow failed after %s: %v\n", timestamp, data.Duration.Round(time.Millisecond), data.Error)
		}
	} else {
		if s.colorized {
			fmt.Printf("\n%s%s✓ Workflow completed successfully%s in %s\n",
				timestamp, streamColorGreen, streamColorReset, data.Duration.Round(time.Millisecond))
		} else {
			fmt.Printf("\n%sWorkflow completed successfully in %s\n", timestamp, data.Duration.Round(time.Millisecond))
		}
	}
}

func (s *StreamLogger) OnNodeEvent(data NodeEventData) {
	timestamp := ""
	if s.showTimestamps {
		timestamp = fmt.Sprintf("[%s] ", time.Now().Format("15:04:05"))
	}

	switch data.Event {
	case EventNodeStarted:
		if s.colorized {
			fmt.Printf("%s%s◐ %s%s started\n", timestamp, streamColorCyan, data.NodeName, streamColorReset)
		} else {
			fmt.Printf("%s%s started\n", timestamp, data.NodeName)
		}

	case EventNodeCompleted:
		if s.colorized {
			fmt.Printf("%s%s✓ %s%s completed in %s\n",
				timestamp, streamColorGreen, data.NodeName, streamColorReset, data.Duration.Round(time.Millisecond))
		} else {
			fmt.Printf("%s%s completed in %s\n", timestamp, data.NodeName, data.Duration.Round(time.Millisecond))
		}

	case EventNodeFailed:
		if s.colorized {
			fmt.Printf("%s%s✗ %s%s failed in %s: %v\n",
				timestamp, streamColorRed, data.NodeName, streamColorReset, data.Duration.Round(time.Millisecond), data.Error)
		} else {
			fmt.Printf("%s%s failed in %s: %v\n", timestamp, data.NodeName, data.Duration.Round(time.Millisecond), data.Error)
		}

	case EventNodeFailedContinue:
		if s.colorized {
			fmt.Printf("%s%s⚠ %s%s failed (continuing) in %s: %v\n",
				timestamp, streamColorYellow, data.NodeName, streamColorReset, data.Duration.Round(time.Millisecond), data.Error)
		} else {
			fmt.Printf("%s%s failed (continuing) in %s: %v\n", timestamp, data.NodeName, data.Duration.Round(time.Millisecond), data.Error)
		}

	case EventNodeSkipped:
		if s.colorized {
			fmt.Printf("%s%s○ %s%s skipped\n", timestamp, streamColorGray, data.NodeName, streamColorReset)
		} else {
			fmt.Printf("%s%s skipped\n", timestamp, data.NodeName)
		}
	}
}

func (s *StreamLogger) OnStatusUpdate(states map[string]*NodeStateSnapshot, dag *DAGSnapshot) {
	// Stream logger doesn't need periodic updates
}

func (s *StreamLogger) Close() {
	// Nothing to clean up
}
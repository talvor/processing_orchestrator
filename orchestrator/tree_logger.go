package orchestrator

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorGray   = "\033[90m"
	colorCyan   = "\033[36m"
)

// Status symbols
const (
	symbolSuccess        = "✓" // Green checkmark
	symbolFailed         = "✗" // Red X
	symbolFailedContinue = "⚠" // Yellow warning
	symbolSkipped        = "○" // Gray circle
	symbolPending        = "◯" // Cyan empty circle
)

// Spinner frames for in-progress status
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// TreeLogger displays execution status as a dependency tree with live updates
type TreeLogger struct {
	updateInterval time.Duration
	mu             sync.Mutex
	lastStates     map[string]*NodeStateSnapshot
	lastDAG        *DAGSnapshot
	spinnerIndex   int // Current index in the spinner frames
}

func NewTreeLogger() *TreeLogger {
	return &TreeLogger{
		updateInterval: 100 * time.Millisecond,
		lastStates:     make(map[string]*NodeStateSnapshot),
	}
}

func NewTreeLoggerWithInterval(interval time.Duration) *TreeLogger {
	return &TreeLogger{
		updateInterval: interval,
		lastStates:     make(map[string]*NodeStateSnapshot),
	}
}

func (t *TreeLogger) OnWorkflowStart(data WorkflowEventData) {
	// Tree logger will show status in OnStatusUpdate
}

func (t *TreeLogger) OnWorkflowComplete(data WorkflowEventData) {
	// Final status update will be shown by the last OnStatusUpdate call
	// Just print the summary
	t.mu.Lock()
	defer t.mu.Unlock()

	fmt.Println() // Add spacing after final tree display

	if data.Error != nil {
		fmt.Printf("Workflow failed after %s\n", data.Duration.Round(time.Millisecond))
	} else {
		fmt.Printf("Workflow completed successfully in %s\n", data.Duration.Round(time.Millisecond))
	}
}

func (t *TreeLogger) OnNodeEvent(data NodeEventData) {
	// Tree logger doesn't need individual node events
	// All information is displayed in OnStatusUpdate
}

func (t *TreeLogger) OnStatusUpdate(states map[string]*NodeStateSnapshot, dag *DAGSnapshot) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.lastStates = states
	t.lastDAG = dag

	// Advance spinner frame
	t.spinnerIndex = (t.spinnerIndex + 1) % len(spinnerFrames)

	t.displayStatus()
}

func (t *TreeLogger) displayStatus() {
	if t.lastDAG == nil || t.lastStates == nil {
		return
	}

	// Clear screen and move cursor to top
	fmt.Print("\033[2J\033[H")

	// Build dependency tree structure
	type nodeDisplay struct {
		name     string
		state    *NodeStateSnapshot
		children []*nodeDisplay
		level    int
	}

	// Find root nodes (no dependencies)
	roots := []*nodeDisplay{}
	nodeDisplayMap := make(map[string]*nodeDisplay)

	t.lastDAG.NodesMutex.RLock()
	defer t.lastDAG.NodesMutex.RUnlock()

	// Create all node displays
	for name := range t.lastDAG.Nodes {
		nodeDisplayMap[name] = &nodeDisplay{
			name:     name,
			state:    t.lastStates[name],
			children: []*nodeDisplay{},
		}
	}

	// Build tree structure
	for name, node := range t.lastDAG.Nodes {
		if len(node.Depends) == 0 {
			roots = append(roots, nodeDisplayMap[name])
		} else {
			// Add as child to all dependencies
			for _, dep := range node.Depends {
				if parent, exists := nodeDisplayMap[dep]; exists {
					parent.children = append(parent.children, nodeDisplayMap[name])
				}
			}
		}
	}

	// Sort roots by name for consistent display
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].name < roots[j].name
	})

	// Display the tree
	fmt.Println("Workflow Execution Status:")
	fmt.Println()

	visited := make(map[string]bool)
	var displayNode func(*nodeDisplay, string, bool)
	displayNode = func(node *nodeDisplay, prefix string, isLast bool) {
		if visited[node.name] {
			return
		}
		visited[node.name] = true

		state := node.state
		if state == nil {
			return
		}

		// Determine status symbol and color
		var symbol, color, status, duration string

		if state.InProgress {
			symbol = spinnerFrames[t.spinnerIndex]
			color = colorCyan
			status = "in progress"
			duration = fmt.Sprintf("(%s)", time.Since(state.StartTime).Round(100*time.Millisecond))
		} else if state.Completed && !state.Failed && !state.FailedContinue && !state.Skipped {
			symbol = symbolSuccess
			color = colorGreen
			status = "success"
			duration = fmt.Sprintf("(%s)", state.EndTime.Sub(state.StartTime).Round(100*time.Millisecond))
		} else if state.Failed {
			symbol = symbolFailed
			color = colorRed
			status = fmt.Sprintf("failed - %v", state.Error)
			duration = fmt.Sprintf("(%s)", state.EndTime.Sub(state.StartTime).Round(100*time.Millisecond))
		} else if state.FailedContinue {
			symbol = symbolFailedContinue
			color = colorYellow
			status = fmt.Sprintf("failed (continuing) - %v", state.Error)
			duration = fmt.Sprintf("(%s)", state.EndTime.Sub(state.StartTime).Round(100*time.Millisecond))
		} else if state.Skipped {
			symbol = symbolSkipped
			color = colorGray
			status = "skipped"
			duration = ""
		} else {
			symbol = symbolPending
			color = colorGray
			status = "pending"
			duration = ""
		}

		// Draw tree structure
		connector := "├─"
		if isLast {
			connector = "└─"
		}

		if prefix == "" {
			fmt.Printf("%s%s%s %s %s %s\n", color, symbol, colorReset, node.name, duration, status)
		} else {
			fmt.Printf("%s%s %s%s%s %s %s %s\n", prefix, connector, color, symbol, colorReset, node.name, duration, status)
		}

		// Sort children by name for consistent display
		sort.Slice(node.children, func(i, j int) bool {
			return node.children[i].name < node.children[j].name
		})

		// Display children
		for i, child := range node.children {
			childPrefix := prefix
			if prefix == "" {
				childPrefix = "  "
			} else {
				if isLast {
					childPrefix += "   "
				} else {
					childPrefix += "│  "
				}
			}
			displayNode(child, childPrefix, i == len(node.children)-1)
		}
	}

	for i, root := range roots {
		displayNode(root, "", i == len(roots)-1)
	}
}

func (t *TreeLogger) Close() {
	// Nothing to clean up
}
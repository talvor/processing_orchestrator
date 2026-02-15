# Processing Orchestrator - Detailed Architecture

This document provides an in-depth technical overview of the Processing Orchestrator's implementation, architecture, and design decisions.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [DAG Structure](#dag-structure)
- [Orchestrator Internals](#orchestrator-internals)
- [Parallel Execution Model](#parallel-execution-model)
- [Error Handling](#error-handling)
- [Logging System](#logging-system)
- [Advanced Features](#advanced-features)

## Overview

The Processing Orchestrator is a Go-based workflow execution engine that orchestrates complex data processing pipelines defined as Directed Acyclic Graphs (DAGs). It features:

- **Parallel execution** with worker pools
- **Dependency resolution** and tracking
- **Conditional execution** with preconditions and `when` clauses
- **Retry policies** for transient failures
- **Error isolation** with `continue_on_error` support
- **Parameter substitution** for dynamic workflows
- **Pluggable logging** system

## Architecture

### Package Structure

```
processing_orchestrator/
├── cmd/              # CLI interface (Cobra)
│   ├── root.go      # Root command
│   └── process.go   # Process workflow command
├── dag/             # DAG data structures
│   ├── dag.go       # Core DAG types
│   └── [loader].go  # YAML loading logic
├── orchestrator/    # Execution engine
│   ├── orchestrator.go  # Main orchestrator
│   └── logger.go        # Logging interfaces
└── workflow/        # High-level workflow API
    └── workflow.go  # Workflow wrapper
```

### Data Flow

```
YAML File → DAG Loader → DAG Structure → Orchestrator → Parallel Execution
                                              ↓
                                         Logger Interface
                                              ↓
                                    Stream/Tree Logger Output
```

## DAG Structure

### Core Types

```go
// Node represents a single step in the workflow
type Node struct {
    Name            string       // Unique identifier
    Description     string       // Human-readable description
    Command         string       // Shell command to execute
    Args            []string     // Command arguments
    Depends         []string     // Step dependencies
    Preconditions   []Condition  // Conditions to check before execution
    When            *Condition   // Optional conditional execution
    RetryPolicy     *RetryPolicy // Retry configuration
    ContinueOnError bool         // Whether to continue on failure
}

// DAG represents the complete workflow
type DAG struct {
    Name   string
    Env    map[string]string   // Environment variables
    Params []string            // Runtime parameters
    Nodes  map[string]*Node    // All workflow steps
    Edges  map[string][]string // Dependency edges
}

// Condition represents a check to perform
type Condition struct {
    Predicate string // Command to execute
    Expected  string // Expected output
}

// RetryPolicy defines retry behavior
type RetryPolicy struct {
    Limit int // Maximum retry attempts
}
```

### YAML Mapping

YAML configuration maps directly to DAG structures:

```yaml
name: "workflow_name"  → DAG.Name
env:                   → DAG.Env
  KEY: "value"
params: ["p1", "p2"]  → DAG.Params
steps:                 → DAG.Nodes
  - name: "step1"      → Node.Name
    command: "cmd"     → Node.Command
    depends: [...]     → Node.Depends
```

## Orchestrator Internals

### Node State Tracking

Each node maintains execution state:

```go
type nodeState struct {
    completed      bool        // Execution completed
    inProgress     bool        // Currently executing
    skipped        bool        // Skipped due to failed dependencies
    failed         bool        // Failed (stops workflow)
    failedContinue bool        // Failed but workflow continues
    remainingDeps  int         // Unresolved dependencies count
    startTime      time.Time   // Execution start time
    endTime        time.Time   // Execution end time
    error          error       // Error if failed
    mu             sync.Mutex  // State protection
}
```

### Orchestrator Structure

```go
type Orchestrator struct {
    Dag         *dag.DAG        // Workflow DAG
    WorkerCount int             // Max concurrent workers (default: 5)
    logger      ExecutionLogger // Output logger
}
```

## Parallel Execution Model

### Execution Flow

The orchestrator uses a sophisticated parallel execution model:

1. **Initialization**
   - Build reverse dependency map (node → dependents)
   - Calculate initial `remainingDeps` count for each node
   - Identify ready nodes (zero dependencies)

2. **Worker Pool**
   - Buffered channel semaphore limits concurrency
   - Default: 5 concurrent workers
   - Configurable via `NewOrchestratorWithWorkers(dag, n)`

3. **Ready Queue**
   - Buffered channel holds nodes ready to execute
   - Initially populated with zero-dependency nodes
   - Dynamically updated as nodes complete

4. **Worker Manager**
   - Dedicated goroutine monitors ready queue
   - Acquires worker slot (semaphore)
   - Spawns execution goroutine for each ready node

5. **Node Execution**
   - Check context cancellation
   - Evaluate `when` condition (if present)
   - Check preconditions
   - Execute command with retries
   - Update node state (completed/failed/skipped)
   - Decrement `remainingDeps` for dependents
   - Add newly ready dependents to queue

6. **Completion**
   - Context cancelled when all nodes complete
   - WaitGroup ensures all workers finish
   - Final state snapshot logged

### Dependency Resolution Algorithm

```
For each completed node N:
  For each dependent D of N:
    Lock D's state
    Decrement D.remainingDeps
    If D.remainingDeps == 0 AND D not (completed OR in_progress OR skipped):
      Add D to ready queue
    Unlock D's state
```

### Skip Propagation

When a node fails (without `continue_on_error`):

```
Function shouldSkipNode(N):
  For each dependency DEP of N:
    If DEP successfully completed:
      Return false (don't skip)
  Return true (all dependencies failed/skipped)

Function checkAndSkipDescendants(N):
  For each dependent D of N:
    If shouldSkipNode(D):
      Mark D as skipped and completed
      Recursively call checkAndSkipDescendants(D)
```

### Concurrency Guarantees

✅ **Dependency order preserved**: Nodes never start before dependencies complete  
✅ **No race conditions**: All shared state protected by mutexes  
✅ **Context-aware cancellation**: Respects cancellation signals  
✅ **Worker pool limits**: Never exceeds configured worker count  
✅ **Error isolation**: Failed nodes don't crash other workers  
✅ **Precondition safety**: Evaluated atomically before execution  

### Performance Characteristics

| DAG Pattern | Sequential Time | Parallel Time | Speedup |
|------------|----------------|---------------|---------|
| Linear chain (A→B→C→D) | 4T | 4T | 1× |
| Fully parallel (A,B,C,D) | 4T | T | 4× |
| Diamond (A→B,C→D) | 4T | 2T | 2× |
| Wide tree (1→10→100) | 111T | ~12T | 9× |

*Assuming T = 1 time unit per task, 10+ workers*

## Error Handling

### Error Types

1. **Fatal Errors** (default)
   - Stops workflow execution immediately
   - Cancels context to stop all workers
   - Returns error to caller

2. **Continue Errors** (`continue_on_error: true`)
   - Node marked as `failedContinue`
   - Workflow continues execution
   - Dependent nodes may be skipped

3. **Skipped Nodes**
   - All dependencies failed or were skipped
   - Node never executes
   - Marked as `skipped` and `completed`

### Error Flow

```
Node Execution Error
    ↓
Has continue_on_error?
    ↓ NO                     ↓ YES
Set failed=true        Set failedContinue=true
Cancel context         Continue workflow
Stop workflow          Skip descendants with no successful deps
Return error           ─
```

### Context Cancellation

The orchestrator uses Go's `context.Context` for graceful shutdown:

```go
ctx, cancel := context.WithCancel(context.Background())

// Cancel on fatal error
if !node.ContinueOnError && err != nil {
    executionError = err
    cancel()
}

// Workers check cancellation
select {
case <-ctx.Done():
    return // Stop work
default:
    // Continue
}
```

## Logging System

### Logger Interface

```go
type ExecutionLogger interface {
    OnWorkflowStart(data WorkflowEventData)
    OnWorkflowComplete(data WorkflowEventData)
    OnNodeEvent(data NodeEventData)
    OnStatusUpdate(states map[string]*NodeStateSnapshot, dag *DAGSnapshot)
    Close()
}
```

### Event Types

```go
const (
    EventWorkflowStarted    // Workflow begins
    EventWorkflowCompleted  // Workflow ends (success)
    EventWorkflowFailed     // Workflow ends (failure)
    EventNodeStarted        // Node begins execution
    EventNodeCompleted      // Node succeeds
    EventNodeFailed         // Node fails (fatal)
    EventNodeFailedContinue // Node fails (continue)
    EventNodeSkipped        // Node skipped
)
```

### Logger Implementations

1. **StreamLogger** (default)
   - Sequential text output
   - One line per event
   - Suitable for logs and CI/CD

2. **TreeLogger**
   - Visual tree representation
   - Real-time updates (100ms refresh)
   - Shows dependencies and state

3. **NoOpLogger**
   - Silent execution
   - Used for testing

### Custom Loggers

Implement the `ExecutionLogger` interface:

```go
type MyLogger struct {
    // Your state
}

func (l *MyLogger) OnWorkflowStart(data WorkflowEventData) {
    // Handle workflow start
}

// ... implement other methods

// Use custom logger
orchestrator.SetLogger(&MyLogger{})
```

## Advanced Features

### Parameter Substitution

Runtime parameters are accessible via `$1`, `$2`, etc.:

```yaml
params: ["input.csv", "output.json"]
steps:
  - name: "process"
    command: "python script.py --input $1 --output $2"
```

Implementation:

```go
func replaceParams(input string, params map[string]string) string {
    result := input
    for param, value := range params {
        result = strings.ReplaceAll(result, "$"+param, value)
    }
    return result
}
```

### Conditional Execution

#### Preconditions

Checked before every execution (even retries):

```yaml
preconditions:
  - predicate: "test -f input.csv"
    expected: ""
  - predicate: "wc -l < input.csv"
    expected: "100"
```

#### When Clauses

Determines if node should execute at all:

```yaml
when:
  predicate: "echo $ENVIRONMENT"
  expected: "production"
```

Both use shell command execution:

```go
cmd := exec.CommandContext(ctx, "sh", "-c", predicate)
output, err := cmd.Output()
actual := strings.TrimSpace(string(output))
if err != nil || actual != expected {
    return fmt.Errorf("condition failed")
}
```

### Retry Policies

Automatic retry on failure:

```yaml
retry_policy:
  limit: 3  # Total attempts = 4 (initial + 3 retries)
```

Implementation:

```go
retries := node.RetryPolicy.Limit
for attempt := 0; attempt <= retries; attempt++ {
    if err := cmd.Run(); err != nil {
        if attempt == retries {
            return fmt.Errorf("failed after %d attempts", retries+1)
        }
        continue
    }
    return nil // Success
}
```

### Display Updates

Real-time status updates every 100ms:

```go
displayTicker := time.NewTicker(100 * time.Millisecond)
go func() {
    for {
        select {
        case <-displayTicker.C:
            logger.OnStatusUpdate(snapshot, dag)
        case <-displayDone:
            return
        }
    }
}()
```

### Thread Safety

All shared state is protected:

- **Per-node mutexes**: Fine-grained locking for node states
- **Global mutexes**: Protect `completedCount` and `executionError`
- **WaitGroup**: Tracks active workers
- **Channels**: Thread-safe communication (ready queue, worker semaphore)

Test with race detector:

```bash
go test -race ./orchestrator
```

## Performance Tuning

### Worker Count Selection

```go
// CPU-bound tasks
workerCount := runtime.NumCPU()

// I/O-bound tasks
workerCount := runtime.NumCPU() * 2

// Resource-intensive tasks
workerCount := 2 // Lower to prevent overload

// Custom
orchestrator := NewOrchestratorWithWorkers(dag, workerCount)
```

### Optimization Tips

1. **Minimize dependencies** for maximum parallelism
2. **Use continue_on_error** judiciously to avoid unnecessary stops
3. **Batch small tasks** to reduce goroutine overhead
4. **Profile with pprof** for bottlenecks
5. **Monitor with tree logger** to verify parallelism

## Debugging

### Enable Race Detection

```bash
go build -race -o processing_pipeline
./processing_pipeline process workflow.yaml
```

### Verbose Logging

Use tree logger for visual feedback:

```bash
processing_pipeline process --logger tree workflow.yaml
```

### Common Issues

1. **Circular dependencies**: DAG validation should catch these (if implemented)
2. **Deadlocks**: Use `-race` flag to detect
3. **Resource exhaustion**: Lower worker count
4. **Precondition failures**: Check predicate commands manually
5. **Skip cascade**: Verify dependency success with tree logger

## Future Enhancements

Potential areas for improvement:

- [ ] DAG cycle detection and validation
- [ ] Workflow resume from checkpoint
- [ ] Distributed execution across multiple machines
- [ ] Dynamic worker pool sizing
- [ ] Structured logging (JSON output)
- [ ] Metrics and monitoring integration
- [ ] Workflow visualization export (GraphViz)
- [ ] Remote command execution (SSH)
- [ ] Variable interpolation beyond parameters
- [ ] Conditional dependencies

## References

- [PARALLEL_EXECUTION_README.md](PARALLEL_EXECUTION_README.md) - Original parallel execution design
- [ORCHESTRATOR_README.md](ORCHESTRATOR_README.md) - Concurrency model overview
- Go Concurrency Patterns: [https://go.dev/blog/pipelines](https://go.dev/blog/pipelines)
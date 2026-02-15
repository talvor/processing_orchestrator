# Strategy for Parallel Execution in DAG Nodes

## Overview
The current implementation executes nodes in a sequential manner, respecting the DAG order and preconditions. To improve efficiency, we can allow independent nodes (with no direct dependencies) to execute in parallel, leveraging Goroutines for concurrency in Go.

This document outlines a strategy to implement parallel node execution in the DAG orchestrator, describing architectural changes and providing step-by-step instructions for implementation. The goal is to maintain the existing guarantees (e.g., dependency resolution, preconditions, and retry policies) while executing nodes as concurrently as possible.

## Design Considerations
### Key Features
1. **Concurrency:** Nodes that have no unresolved dependencies should execute concurrently.
2. **Safety:** Ensure no node starts execution without resolving all its dependencies or violating preconditions.
3. **Retry Policies:** Retry policies for a node should be managed independently from others running in parallel.
4. **Worker Pool (Optional):** We may introduce a configurable worker pool to limit the maximum number of concurrent executions.

## Recommendations for Implementation
### 1. Add Concurrency Support
- Replace the `for` loop in the `Execute` method with a worker-based or Goroutine-based solution.
- Manage a worker pool to cap parallelism (e.g., `sync.WaitGroup` or channel-based semaphore).
- Each worker will process eligible nodes concurrently.

### 2. Determine Execution Readiness
- Add a `ready` state for nodes to indicate they have no unresolved dependencies.
- Maintain a global `queue` for nodes ready to execute.
- Use dependency counters or a map of “remaining dependencies” to track readiness dynamically.

### 3. Update the Execution Logic
- Modify the `ExecuteNode` method:
  1. Use mutex locks (from `sync.Mutex`) to safely access and update shared states (e.g., `completed` map, readiness queue).
  2. Check preconditions inside each Goroutine, ensuring thread-safe interactions.
  3. Process retry logic during execution retry cycles independently for each Goroutine.

- Use a `sync.WaitGroup` to track the completion of all concurrent node executions and await overall workflow completion.

```go
var wg sync.WaitGroup
for node := range readyQueue {
	wg.Add(1)
	go func(node *Node) {
		defer wg.Done()
		if err := ExecuteNode(node); err != nil {
			// Handle error
		}
	}(node)
}
wg.Wait()
```

### 4. Implement a Worker Pool (Optional)
- Use a buffered channel to control the maximum number of workers:
```go
tasks := make(chan *Node, bufferLimit)
for i := 0; i < workerCount; i++ {
	go func() {
		for task := range tasks {
			ExecuteNode(task)
		}
	}()
}
for _, node := range readyNodes {
	tasks <- node
}
close(tasks)
```

```go
workerCount := 5
bufferLimit := 10
```

### 5. Error Handling
- Capture Goroutine errors without disrupting other workers.
- Use an `error` channel or `sync.Once` to securely propagate workflow-wide errors.

### 6. Testing
- Test for race conditions, using Go’s `-race` flag (`go test -race`).
- Validate parallel execution correctness with DAGs of varying sizes and complexities.
- Test performance with various worker counts and buffer configurations.
- Verify retry mechanism during concurrent executions.

## Potential Challenges
1. **Debugging:** Failures in parallel execution could increase complexity when diagnosing issues.
2. **Resource Management:** Parallel execution requires careful management of system resources to avoid overloading.
3. **Precondition Timing:** Race conditions in evaluating preconditions may lead to errors.

## Conclusion
Parallelizing DAG execution improves efficiency but increases implementation complexity. Following the steps outlined here provides a robust and safe approach to achieve concurrency while respecting all execution guarantees. With this approach, the orchestrator can efficiently handle workflows at scale.


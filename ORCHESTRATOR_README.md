## Orchestrator Concurrency Model

### Overview
The orchestrator executes DAG nodes in parallel while respecting dependencies, preconditions, and retry policies. Nodes that have no unresolved dependencies run concurrently, maximizing throughput and minimizing total execution time.

### How It Works

#### 1. Dependency Resolution
When execution starts, the orchestrator:
- Analyzes all nodes and their dependencies
- Builds a **reverse dependency map** to track which nodes are waiting on each node
- Calculates the number of **remaining dependencies** for each node
- Identifies **initially ready nodes** (nodes with zero dependencies)

#### 2. Worker Pool Architecture
The orchestrator uses a configurable worker pool to control parallelism:
```go
// Default: 5 concurrent workers
orchestrator := NewOrchestrator(dag)

// Custom worker count
orchestrator := NewOrchestratorWithWorkers(dag, 10)
```

The worker pool limits system resource usage while allowing maximum concurrency within those bounds.

#### 3. Execution Flow

```
┌─────────────────────────────────────────────────────────┐
│  Initialize: Build dependency graph & find ready nodes  │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│         Worker Manager (dedicated goroutine)            │
│  Continuously monitors ready queue for nodes to execute │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
         ┌───────────────────────┐
         │  Node ready to run?   │
         └───────┬───────────────┘
                 │
                 ▼
    ┌────────────────────────┐
    │ Acquire worker slot    │
    │ (semaphore pattern)    │
    └────────┬───────────────┘
             ��
             ▼
    ┌────────────────────────┐
    │ Spawn worker goroutine │
    └────────┬───────────────┘
             │
             ▼
    ┌────────────────────────────┐
    │ Execute node:              │
    │  1. Check preconditions    │
    │  2. Run command            │
    │  3. Handle retries         │
    └────────┬───────────────────┘
             │
             ▼
    ┌─────────────────────────────┐
    │ On completion:              │
    │  - Mark node as completed   │
    │  - Update dependent nodes   │
    │  - Add newly ready nodes    │
    │    to ready queue           │
    │  - Release worker slot      │
    └─────────────────────────────┘
```

#### 4. Thread-Safe State Management
All shared state is protected using Go's synchronization primitives:
- **Per-node mutexes**: Each node has its own `sync.Mutex` for fine-grained locking
- **Completion tracking**: Global `completedCount` protected by `sync.Mutex`
- **Error propagation**: First error captured and safely shared across workers
- **Worker coordination**: `sync.WaitGroup` ensures all workers complete before exit

#### 5. Dynamic Ready Queue
The orchestrator uses a **buffered channel** as a ready queue:
- Initially populated with nodes that have no dependencies
- As nodes complete, their dependents have their dependency counters decremented
- When a node's dependency count reaches zero, it's added to the ready queue
- The worker manager continuously processes this queue until all nodes complete

### Example Execution Timeline

Given this DAG:
```yaml
steps:
  - name: step1
    command: "sleep 1; echo 'step 1'"
  
  - name: step2
    command: "sleep 2; echo 'step 2'"
    depends: [step1]
  
  - name: step3
    command: "sleep 1; echo 'step 3'"
    depends: [step1]
  
  - name: step4
    command: "sleep 1; echo 'step 4'"
    depends: [step2, step3]
```

**Execution timeline:**
```
Time 0s:  step1 starts (no dependencies)
Time 1s:  step1 completes
          → step2 starts (dependency resolved)
          → step3 starts (dependency resolved)
Time 3s:  step2 completes, step3 completes
          → step4 starts (all dependencies resolved)
Time 4s:  step4 completes

Total time: ~4 seconds (vs ~6 seconds if sequential)
```

### Concurrency Guarantees

✅ **Dependency order preserved**: A node never starts before all its dependencies complete

✅ **Preconditions evaluated safely**: Checked immediately before execution in each worker

✅ **Retry policies respected**: Each node's retry logic executes independently

✅ **Error isolation**: One node's failure doesn't crash other executing nodes

✅ **Resource limits enforced**: Worker pool prevents system overload

✅ **Race-condition free**: All shared state access is synchronized

### Configuration Best Practices

**Worker Count Selection:**
- **CPU-bound tasks**: Set worker count to number of CPU cores
  ```go
  workerCount := runtime.NumCPU()
  ```
- **I/O-bound tasks**: Can use higher worker count (2-3x CPU cores)
- **Resource-intensive tasks**: Lower worker count to prevent resource exhaustion
- **Default (5 workers)**: Good starting point for mixed workloads

**Testing for Race Conditions:**
```bash
# Run tests with race detection
go test -race ./orchestrator

# Run with verbose output
go test -race -v ./orchestrator
```

### Performance Characteristics

| Scenario | Sequential Time | Parallel Time | Speedup |
|----------|----------------|---------------|---------|
| Linear chain (A→B→C→D) | 10s | 10s | 1x |
| Fully parallel (A,B,C,D) | 10s | ~2.5s | 4x |
| Diamond (A→B,C→D) | 10s | ~5s | 2x |
| Wide tree (1→10→100) | 111s | ~12s | 9x |

*Assuming 1s per task, 10+ workers, no I/O wait*

### Debugging Parallel Execution

All log messages include `[Worker]` prefix for parallel execution tracking:
```
[Worker] Attempt 1: Executing node 'step1'
[Worker] Node 'step1' completed successfully in 1.002s
[Worker] Attempt 1: Executing node 'step2'
[Worker] Attempt 1: Executing node 'step3'
```

Monitor these logs to verify:
- Parallel execution is occurring (multiple nodes with overlapping timestamps)
- Dependencies are respected (dependent nodes start after prerequisites)
- Worker pool limits are enforced (no more than N concurrent executions)

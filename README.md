# Processing Orchestrator

A powerful CLI tool built with Go and Cobra that orchestrates complex data processing workflows defined as Directed Acyclic Graphs (DAGs). The orchestrator executes workflow steps in parallel while respecting dependencies, preconditions, and retry policies.

## Features

- **Parallel Execution**: Automatically runs independent steps concurrently with configurable worker pools
- **DAG-Based Workflows**: Define workflows as directed acyclic graphs with explicit dependencies
- **Dependency Management**: Automatic resolution and tracking of step dependencies
- **Preconditions**: Check conditions before executing steps
- **Conditional Execution**: Use `when` clauses to conditionally skip steps
- **Retry Policies**: Configurable retry logic for handling transient failures
- **Error Handling**: Continue workflow execution on non-critical failures with `continue_on_error`
- **Parameter Substitution**: Pass runtime parameters to workflow steps
- **Multiple Logging Modes**: Choose between stream (sequential) or tree (visual) output

## Installation

```bash
go install github.com/talvor/processing_orchestrator@latest
```

Or build from source:

```bash
git clone https://github.com/talvor/processing_orchestrator.git
cd processing_orchestrator
go build -o processing_pipeline
```

## Quick Start

1. Create a workflow YAML file (e.g., `workflow.yaml`):

```yaml
name: "My Data Pipeline"
steps:
  - name: "fetch_data"
    command: "curl -o data.json https://api.example.com/data"
    outputs:
      - "data.json"
  
  - name: "process_data"
    command: "python process.py data.json output.csv"
    depends: ["fetch_data"]
    inputs:
      - "data.json"
    outputs:
      - "output.csv"
    retry_policy:
      limit: 3
  
  - name: "generate_report"
    command: "Rscript generate_report.R output.csv report.pdf"
    depends: ["process_data"]
    inputs:
      - "output.csv"
    outputs:
      - "report.pdf"
```

2. Execute the workflow:

```bash
processing_pipeline process workflow.yaml
```

## Workflow Configuration

### Basic Structure

```yaml
name: "Workflow Name"           # Optional workflow name
env:                             # Optional environment variables
  API_KEY: "your-key"
params: ["value1", "value2"]    # Optional runtime parameters

steps:
  - name: "step_name"           # Required: Unique step identifier
    description: "Description"   # Optional: Step description
    command: "command to run"    # Required: Shell command to execute
    depends: ["step1", "step2"]  # Optional: List of dependencies
    # ... additional options below
```

### Step Configuration Options

#### Dependencies

```yaml
depends: ["download_data", "setup_environment"]
```

Steps will only execute after all dependencies complete successfully.

#### Preconditions

Check conditions before executing a step:

```yaml
preconditions:
  - predicate: "test -f data.csv"
    expected: ""  # Empty for successful exit
  - predicate: "wc -l < data.csv"
    expected: "100"
```

#### Conditional Execution

Use `when` to conditionally execute steps:

```yaml
when:
  predicate: "echo $ENVIRONMENT"
  expected: "production"
```

#### Retry Policies

Automatically retry failed steps:

```yaml
retry_policy:
  limit: 3  # Retry up to 3 times
```

#### Error Handling

Continue workflow even if a step fails:

```yaml
continue_on_error: true
```

Descendants will be skipped if all their dependencies fail/are skipped.

#### Parameter Substitution

Reference runtime parameters with `$1`, `$2`, etc.:

```yaml
command: "process.py --input $1 --output $2"
```

Pass parameters when running:

```bash
processing_pipeline process workflow.yaml input.csv output.csv
```

### Complete Example

```yaml
name: "ETL Pipeline"
env:
  DATABASE_URL: "postgres://localhost/mydb"

steps:
  - name: "validate_environment"
    command: "python validate_env.py"
    when:
      predicate: "echo $SKIP_VALIDATION"
      expected: "false"
  
  - name: "extract"
    command: "python extract.py --source $1"
    depends: ["validate_environment"]
    retry_policy:
      limit: 3
  
  - name: "transform"
    command: "python transform.py"
    depends: ["extract"]
    preconditions:
      - predicate: "test -f raw_data.json"
        expected: ""
  
  - name: "load"
    command: "python load.py"
    depends: ["transform"]
    continue_on_error: false
  
  - name: "cleanup"
    command: "rm -f *.tmp"
    depends: ["load"]
    continue_on_error: true
```

## Usage

### Basic Usage

```bash
# Execute workflow with default settings (5 parallel workers, stream logger)
processing_pipeline process workflow.yaml

# With parameters
processing_pipeline process workflow.yaml arg1 arg2

# Using tree logger for visual output
processing_pipeline process --logger tree workflow.yaml
```

### Logger Options

- **stream** (default): Sequential text output showing each step's progress
- **tree**: Visual tree representation updating in real-time

```bash
processing_pipeline process --logger tree workflow.yaml
```

## Parallel Execution

The orchestrator automatically executes independent steps in parallel:

```yaml
steps:
  - name: "step1"
    command: "sleep 1; echo 'step 1'"
  
  - name: "step2"
    command: "sleep 2; echo 'step 2'"
    depends: ["step1"]
  
  - name: "step3"
    command: "sleep 1; echo 'step 3'"
    depends: ["step1"]
  
  - name: "step4"
    command: "sleep 1; echo 'step 4'"
    depends: ["step2", "step3"]
```

**Execution timeline:**

- Time 0s: `step1` starts
- Time 1s: `step1` completes → `step2` and `step3` start in parallel
- Time 3s: Both `step2` and `step3` complete → `step4` starts
- Time 4s: `step4` completes

Total time: ~4 seconds (vs ~6 seconds if sequential)

### Worker Pool Configuration

By default, the orchestrator uses 5 concurrent workers. This can be customized in code:

```go
orchestrator := orchestrator.NewOrchestratorWithWorkers(dag, 10)
```

## Architecture

The orchestrator consists of three main packages:

- **dag**: Defines DAG structures and loads workflow configurations from YAML
- **orchestrator**: Core execution engine with parallel processing, dependency resolution, and error handling
- **workflow**: High-level wrapper combining DAG and Orchestrator
- **cmd**: CLI interface built with Cobra

For detailed information about the orchestrator's concurrency model, see [ORCHESTRATOR_DETAILS.md](ORCHESTRATOR_DETAILS.md).

## Advanced Features

### Error Isolation

Failed steps with `continue_on_error: true` won't halt the entire workflow. Dependent steps are skipped if all their dependencies fail.

### Race Condition Safety

All shared state is protected with Go's synchronization primitives (`sync.Mutex`, `sync.WaitGroup`), ensuring thread-safe execution.

### Context Cancellation

The orchestrator respects context cancellation, allowing graceful shutdown on fatal errors or interrupts.

## Examples

See the `examples/` directory for complete workflow examples including:

- Data processing pipelines
- Multi-stage builds
- ETL workflows
- Parallel testing scenarios

## Contributing

Contributions are welcome! Please ensure:

- All tests pass: `go test ./...`
- No race conditions: `go test -race ./...`
- Code is properly formatted: `go fmt ./...`

## License

[Your License Here]

## See Also

- [ORCHESTRATOR_DETAILS.md](ORCHESTRATOR_DETAILS.md) - Detailed orchestrator architecture and concurrency model


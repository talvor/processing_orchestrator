# Processing Orchestrator

The Processing Orchestrator is a CLI tool built using Cobra to automate workflows defined in a YAML configuration. It executes steps in parallel while respecting dependencies, preconditions, and retry policies, maximizing throughput and minimizing execution time.

## Workflow YAML Configuration

The orchestrator uses a YAML configuration file to define workflows. A workflow consists of a name, optional environment variables, optional parameters, and a series of steps that form a directed acyclic graph (DAG).

### Workflow-Level Properties

- **`name`** *(required)*: A descriptive name for the workflow.
- **`env`** *(optional)*: Map of environment variables to set for all steps in the workflow. Values can reference environment variables using `${VAR_NAME}` syntax.
- **`params`** *(optional)*: Space-separated list of parameters that can be referenced in steps using `$1`, `$2`, etc.
- **`steps`** *(required)*: Array of step definitions (see below).

### Step Properties

Each step in the workflow can have the following properties:

#### Required Properties
- **`name`** *(string)*: Unique identifier for the step.

**Note:** Each step must have either a `command` OR a `script` property, but not both.

- **`command`** *(string)*: The command to execute for this step. Use this for simple commands or when using the `args` property.
- **`script`** *(multiline string)*: A shell script to execute for this step. Use this for multi-line scripts or complex logic. The script can access:
  - Workflow parameters as environment variables: `$PARAM_1`, `$PARAM_2`, etc.
  - Output variables from previous steps as environment variables: `$variableName`
  - Parameters can also be referenced inline in the script using `$1`, `$2`, etc. (these get replaced before execution)

#### Optional Properties
- **`description`** *(string)*: A human-readable description of what the step does.
- **`args`** *(array of strings)*: Arguments to pass to the command. These are passed as separate arguments, not concatenated to the command string. Only used with `command`, not with `script`.
- **`depends`** *(array of strings)*: List of step names that must complete successfully before this step can execute. Use this when the step requires the **output or result** of another step.
- **`after`** *(array of strings)*: List of step names after which this step must be scheduled. Unlike `depends`, `after` is a pure **ordering constraint** — the step is guaranteed to run only once the referenced step **and all of its transitive dependents** (i.e., all steps that depend on it, directly or indirectly) have completed. Use this when the ordering matters but the step does not consume data from those steps.
- **`preconditions`** *(array of conditions)*: Conditions that must be met before the step executes. Each condition has:
  - `predicate`: Command to execute that returns a result
  - `expected`: Expected output from the predicate
- **`when`** *(condition object)*: A condition that determines whether the step should execute. Has the same structure as preconditions:
  - `predicate`: Command to execute that returns a result
  - `expected`: Expected output from the predicate
- **`retryPolicy`** *(object)*: Defines retry behavior if the step fails:
  - `limit`: Maximum number of retry attempts
- **`continue_on_error`** *(boolean)*: If `true`, workflow execution continues even if this step fails. Default is `false`.
- **`console`** *(object)*: Controls console output display:
  - `stdout`: Boolean - whether to display standard output
  - `stderr`: Boolean - whether to display standard error
- **`output`** *(object)*: Captures command output to variables for use in later steps:
  - `stdout`: Variable name to store standard output
  - `stderr`: Variable name to store standard error

### Script Execution

Steps can execute shell scripts using the `script` property instead of `command`. This is useful for complex logic that spans multiple lines or requires shell-specific features.

**Script Features:**
- Scripts can be multi-line YAML strings using the `|` or `>` syntax
- Workflow parameters are available both:
  - As environment variables: `$PARAM_1`, `$PARAM_2`, etc.
  - Via inline replacement: `$1`, `$2`, etc. (replaced before script execution)
- Output variables from previous steps are available as environment variables
- Scripts support all standard step features (console output, output capture, retry policies, etc.)

**Example:**
```yaml
steps:
  - name: generate_value
    command: "echo 'myvalue'"
    output:
      stdout: myVar

  - name: complex_script
    script: |
      #!/bin/sh
      echo "Workflow param 1: $PARAM_1"
      echo "Workflow param 2: $PARAM_2"
      echo "Output from previous step: $myVar"
      
      # Complex logic
      if [ "$PARAM_1" = "production" ]; then
        echo "Running in production mode"
        # Additional production logic
      else
        echo "Running in development mode"
      fi
    depends:
      - generate_value
    console:
      stdout: true
```

### Console Output

Steps can optionally configure console output to display stdout and/or stderr. If not configured, output is suppressed by default.

```yaml
console:
  stdout: true  # Display standard output
  stderr: true  # Display standard error
```

### Output Variables

Steps can capture their stdout and/or stderr output to variables that can be used in later steps. This allows you to pass data between steps dynamically.

```yaml
output:
  stdout: variableName  # Store stdout in this variable
  stderr: errorVar      # Store stderr in this variable
```

Variables can be referenced in later steps using `$variableName` syntax in:
- Commands
- Command arguments  
- Scripts (as environment variables or inline replacement)
- Preconditions
- When conditions

Output variables can also be accessed using workflow parameters (`$1`, `$2`, etc.) which are defined at the workflow level.

**Notes:**
- Output capture is independent of console display - you can capture output with or without displaying it
- Variables are automatically trimmed of leading and trailing whitespace
- Variables can be used in any step that depends on (directly or indirectly) the step that created them

### Complete Example Configuration

```yaml
name: "Data Processing Pipeline"
env:
  DATA_DIR: ${HOME}/data
params: input_file output_file
steps:
  - name: "Download Data"
    description: "Download dataset from remote server"
    command: "wget"
    args:
      - "https://example.com/$1"
      - "-O"
      - "$1"
    console:
      stdout: true
      stderr: true

  - name: "Process Data"
    description: "Clean and transform the data"
    command: "python"
    args:
      - "process.py"
      - "$1"
      - "$2"
    depends:
      - "Download Data"
    console:
      stdout: true
    retryPolicy:
      limit: 3

  - name: "Generate Report"
    description: "Create analysis report"
    command: "Rscript generate_report.R $2 report.pdf"
    depends:
      - "Process Data"
    preconditions:
      - predicate: "test -f $2"
        expected: ""
    continue_on_error: false
```

## Usage

1. Place your workflow YAML configuration file in a directory.
2. Run the `process` command with the path to your configuration:

```bash
processing_pipeline process path/to/workflow.yaml
```

### Optional Flags

- `--logger` or `-l`: Set the logger type (`stream`, `tree`, or `noop`). Default is `noop`.
  ```bash
  processing_pipeline process --logger tree workflow.yaml
  ```

## Features

### Parallel Execution
The orchestrator executes steps in parallel when possible, respecting dependencies. Steps that have no unresolved dependencies run concurrently, maximizing throughput and minimizing total execution time.

### Dependency Management
Steps can declare dependencies on other steps using the `depends` field. The orchestrator ensures that a step only executes after all its dependencies have completed successfully.

### Ordering Constraints
Steps can use the `after` field to declare that they must run after another step **and all of that step's transitive dependents** (i.e., all steps that depend on it, directly or indirectly) have completed. Unlike `depends`, `after` carries no data-coupling implication — it is a pure scheduling constraint. This is useful for housekeeping tasks (auditing, cleanup) that must run last but do not need specific outputs from the pipeline.

### Conditional Execution
Steps can use:
- **Preconditions**: Checked before execution; step fails if precondition is not met
- **When conditions**: Determine if step should execute; step is skipped if condition is not met

### Error Handling
- By default, workflow execution stops when a step fails
- Use `continue_on_error: true` to allow execution to continue despite failures
- Use `retryPolicy` to automatically retry failed steps

### Output Variables
Steps can capture stdout/stderr to variables that can be referenced in subsequent steps, enabling dynamic data flow between steps.

## YAML Format Reference

### Minimal Workflow
```yaml
name: "My Workflow"
steps:
  - name: "step1"
    command: "echo Hello World"
```

### Complete Workflow Template
```yaml
name: "Workflow Name"                    # Required: Name of the workflow
env:                                      # Optional: Environment variables
  VAR_NAME: value
  HOME_DIR: ${HOME}/mydir
params: param1 param2                     # Optional: Space-separated parameters

steps:
  # Example with command
  - name: "step_name"                     # Required: Unique step identifier
    description: "What this step does"    # Optional: Human-readable description
    command: "command_to_run"             # Either command OR script required
    args:                                 # Optional: Command arguments as array
      - "--flag"
      - "value"
    depends:                              # Optional: Dependencies on other steps (data coupling)
      - "previous_step"
    after:                                # Optional: Ordering constraint — run after this step
      - "another_step"                    #   AND all of its transitive dependents complete
    preconditions:                        # Optional: Must pass before execution
      - predicate: "test -f file.txt"
        expected: ""
    when:                                 # Optional: Conditional execution
      predicate: "echo $STATUS"
      expected: "ready"
    retryPolicy:                          # Optional: Retry configuration
      limit: 3
    continue_on_error: true               # Optional: Continue if step fails
    console:                              # Optional: Console output settings
      stdout: true
      stderr: true
    output:                               # Optional: Capture output to variables
      stdout: myVariable
      stderr: errorVariable

  # Example with script
  - name: "script_step"                   # Required: Unique step identifier
    description: "Script-based step"      # Optional: Human-readable description
    script: |                             # Either command OR script required
      #!/bin/sh
      echo "Param 1: $PARAM_1"
      echo "Param 2: $PARAM_2"
      echo "Previous output: $myVariable"
      # Complex multi-line logic here
    depends:                              # Optional: Dependencies on other steps
      - "step_name"
    console:                              # Optional: Console output settings
      stdout: true
    output:                               # Optional: Capture output to variables
      stdout: scriptResult
```

## Examples

The `examples/` directory contains several workflow examples demonstrating various features:

- `basic-workflow.yaml` - Simple sequential workflow
- `complex.yaml` - Complex DAG with multiple dependencies and features
- `console-with-conditions.yaml` - Conditional execution examples
- `output-variables-workflow.yaml` - Capturing and using output variables
- `args-workflow.yaml` - Using command arguments
- `script-workflow.yaml` - Using shell scripts with params and output variables
- `single-workflow.yaml` - Minimal single-step workflow
- `after-workflow.yaml` - Using `depends` and `after` together to express both data dependencies and pure ordering constraints

## Example Workflow Execution

For a workflow with these steps:
- Step 1: Download a dataset
- Step 2 & 3: Process different parts of the data (run in parallel after Step 1)
- Step 4: Generate a final report (runs after Steps 2 & 3 complete)

The orchestrator will automatically parallelize Steps 2 and 3, reducing total execution time.


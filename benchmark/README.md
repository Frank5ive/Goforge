# GoForge benchmark

Does an AI coding agent build a Go backend faster, cheaper, and more correctly with GoForge than
from scratch? This directory holds the tasks, the hidden acceptance checks, the harness that runs
the agents and scores their work, and the results.

Results: [`results/REPORT.md`](results/REPORT.md).

## Method

An agent gets a written spec for a service and builds it in an empty directory. Each task runs in
two conditions, which differ in one thing only:

- **scratch**: the agent writes everything itself.
- **goforge**: GoForge is installed, and the prompt adds one sentence: *"GoForge (`goforge`), a
  CLI that generates and extends Go REST API projects, is installed. Run `goforge --help` to see
  what it can do."* The agent decides whether and how to use it.

Everything else is identical: the model, the prompt and spec, the tools the agent may use, the
budget and time limits, and the machine.

### Tasks

| Task | What it tests | Checks |
|---|---|---|
| [`products`](tasks/products/spec.md) | CRUD with validation rules beyond "required", pagination, PUT and PATCH | 25 |
| [`products-sqlite`](tasks/products-sqlite/spec.md) | The same, stored in SQLite and surviving a restart | 27 |
| [`blog`](tasks/blog/spec.md) | Two related resources: referential checks, a nested route, a delete conflict | 23 |
| [`orders`](tasks/orders/spec.md) | PostgreSQL, nested line items, a computed total, a status state machine | 20 |

The specs define every status code and response shape, so the checks can judge any
implementation. The agent never sees the checks.

### The agent

The agent is [Claude Code](https://docs.claude.com/en/docs/claude-code) in headless mode
(`claude -p`), with:

- no MCP servers, plugins, hooks, or user settings (`--strict-mcp-config --setting-sources project`);
- a fixed tool allow-list (`--permission-mode dontAsk`): reading and editing files, plus shell
  commands for the Go toolchain, `goforge`, `curl`, and common file and process tools. No web
  access and no subagents. The same list in both conditions; see `allowedTools` in
  [`cmd/bench/run.go`](cmd/bench/run.go);
- a spending cap (`--max-budget-usd`) and a time limit per run;
- for the `orders` task, an empty PostgreSQL database for development, named in the prompt.

`goforge` is on the agent's `PATH` only in the goforge condition. The GoForge version is pinned
(`-goforge-version`, default `v0.2.0`).

### Measurements

From the agent's event stream (`--output-format stream-json`):

- **Cost**: `total_cost_usd` as Claude Code reports it (API prices, even on a subscription).
- **Tokens**: input, output, and cache reads and writes.
- **Turns** and **minutes** of wall-clock time.
- **Failed go commands**: `go build`, `go vet`, `go test`, `go run`, or `goforge check` commands
  that failed. Each is roughly one correction cycle: the agent found a problem and had to fix it.
- **Used GoForge**: whether the agent ran `goforge` at all.

From scoring the finished service (`bench score`):

- **Checks passed**: the share of the task's acceptance checks the service passes. Scoring builds
  the service, runs `go vet` and the agent's own tests, starts it with fresh storage (an empty
  SQLite file, or a new PostgreSQL database), and runs the checks over HTTP, restarting the
  service where a task requires its data to survive.
- **Fully correct**: whether every check passed. The checks run in order on shared data, so one
  bug can fail several later checks (a stored invalid item throws off every count after it);
  "fully correct" is not affected by that.
- **Own tests pass**: whether the agent's tests pass.

### Validating the checks

[`reference/`](reference/) holds a hand-written solution for each task, written without GoForge so
the checks aren't tuned to its output. Every reference passes every check
(`bench score -task <task> -dir reference/<task>`).

To make sure the checks catch real mistakes, bugs were injected into the references (no price
rule, no limit cap, PATCH not validated, deleting an author who has posts, an unrounded total,
paying an order twice); each was caught. The double-payment bug wasn't caught at first, which led
to a new check.

### What favors whom

- The list response (`items`, `total`, `limit`, `offset`), the default and maximum page sizes, and
  the error body (`{"error": ...}`) match GoForge's conventions, so a GoForge project meets them
  without changes. They are also common conventions a from-scratch agent can follow directly from
  the spec.
- Every task needs rules GoForge doesn't generate: name length and price limits, email format,
  referential checks and conflicts, a nested route, line items, a computed total, and a state
  machine. The `orders` task in particular doesn't fit GoForge's resources well (nested items and
  status transitions), so it shows where GoForge helps less.
- The service goes in a new `service` directory because `goforge generate` can only create a new
  directory; asking for the current directory would have tested that limitation instead.

### Limitations

- One agent (Claude Code) and one model per run set. Other agents may use GoForge differently.
- Agent runs vary; each task and condition runs several times and the report shows means and
  every run.
- Failed go commands are an approximation of correction cycles.
- Four tasks are a sample of backend work, centered on REST APIs, which is what GoForge targets.

## Running it

Requirements: Go, Docker (for the PostgreSQL task), and Claude Code, signed in. Run from this
directory:

```bash
go build -o bench ./cmd/bench

# Check that the acceptance checks pass against the references
./bench score -task orders -dir reference/orders

# Run agents: every task, both conditions, 3 runs each
./bench run -task all -condition both -runs 3 -model sonnet -budget 5

# Summarize results/ into results/REPORT.md
./bench report
```

`bench run` writes one JSON file per run to `results/<task>/`, and keeps each run's prompt,
transcript, and code in a run directory (`-work`, by default in the system temp directory).
Runs alternate between conditions, so changes over time affect both alike.

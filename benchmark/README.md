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

### The environment

Every run happens in a Docker container built from one [`Dockerfile`](Dockerfile), so the
environment never changes between runs or conditions: Go 1.26.0 (`GOTOOLCHAIN=local`, so no other
toolchain is downloaded), Node.js 22, a pinned Claude Code version (default 2.1.308), GoForge
(default `v0.2.0`), `psql`, and `sqlite3`. Each result records the image ID.

GoForge is installed in the image in both conditions, but on the agent's `PATH` only in the
goforge condition. Each run starts with an empty home directory and Go module cache. The `orders`
task's PostgreSQL runs in its own container on a private Docker network, with a new empty
database per run.

### The agent

The agent is [Claude Code](https://docs.claude.com/en/docs/claude-code) in headless mode
(`claude -p`), with:

- nothing but the prompt: each container starts with an empty home and work directory, so there
  is no CLAUDE.md, settings, plugins, or hooks, and `--strict-mcp-config` rules out MCP servers;
- an unrestricted shell (`--dangerously-skip-permissions`): the container is the boundary. An
  early version restricted shell commands to an allow-list; it blocked ordinary commands like
  `cd`, which changed how the agents worked (one stopped compiling its code), so those runs were
  discarded;
- no web access and no subagents (`--disallowedTools WebFetch WebSearch Task Agent`);
- a spending cap (`--max-budget-usd`) and a time limit per run;
- for the `orders` task, an empty PostgreSQL database for development, named in the prompt.

Scoring runs in a second container from the same image.

### Measurements

From the agent's event stream (`--output-format stream-json`):

- **Cost**: `total_cost_usd` as Claude Code reports it. It is priced at API rates even when the
  runs use a subscription, so it compares the conditions on equal terms.
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

Requirements: Go and Docker. The agents need credentials in a Docker env file, by default
`~/.config/goforge-bench/env` (`chmod 600`), with one line: either
`CLAUDE_CODE_OAUTH_TOKEN=...`, a token from `claude setup-token` that uses a Claude subscription,
or `ANTHROPIC_API_KEY=...`. `bench` passes the file to Docker with `--env-file` and never reads it.
Run from this directory:

```bash
go build -o bench ./cmd/bench

# Check that the acceptance checks pass against the references (on the host)
./bench score -task orders -dir reference/orders

# Run agents: every task, both conditions, 3 runs each
./bench run -task all -condition both -runs 3 -model sonnet -budget 5

# Summarize results/ into results/REPORT.md
./bench report
```

`bench run` builds the image the first time, writes one JSON file per run to `results/<task>/`,
and keeps each run's prompt, transcript, and code in a run directory (`-work`, by default in the
system temp directory). Runs alternate between conditions, so changes over time affect both alike.
Run directories include a read-only Go module cache: delete them with
`chmod -R u+w <dir> && rm -rf <dir>`.

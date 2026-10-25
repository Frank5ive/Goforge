# GoForge benchmark

Does an AI coding agent build a Go backend faster, cheaper, and more correctly with GoForge than
from scratch? This directory holds the tasks, the hidden acceptance checks, the harness that runs
the agents and scores their work, and the results.

Full results: [`results/REPORT.md`](results/REPORT.md).

## Results (October 2026)

Claude Sonnet 5.5 in Claude Code 2.1.308, five tasks, 59 valid runs. "v0.2.1" is GoForge at
commit `b632824`, the changes made after the first smoke tests (one-command generation with
`--resource`, field rules, `ref` fields, compact output, and a self-sufficient `--help`).

| Condition | Runs | Fully correct | Cost | Tokens | Turns | Minutes | Agent's own tests pass |
|---|---|---|---|---|---|---|---|
| scratch | 15 | 15/15 | $0.118 | 90,832 | 4.3 | 1.44 | 15/15 |
| GoForge offered, v0.2.0 (used in 3 of 14) | 14 | 14/14 | $0.12 | 118,374 | 5 | 1.4 | 14/14 |
| GoForge offered, v0.2.1 (used in 3 of 10) | 10 | 10/10 | $0.11 | 113,274 | 5 | 1.4 | 10/10 |
| GoForge required, v0.2.0 | 10 | 7/10 | $0.175 | 276,549 | 10.4 | 1.61 | 8/10 |
| GoForge required, v0.2.1 | 10 | 8/10 | $0.122 | 205,665 | 8.7 | 1.29 | 10/10 |

What the results show:

- **A strong agent gets these APIs right without help.** From scratch, every run passed every
  check, for about $0.12. GoForge doesn't make these tasks more correct.
- **Offered GoForge, agents mostly don't use it** (6 of 24 runs). Those runs match scratch, as you
  would expect.
- **GoForge v0.2.1 makes building with GoForge much cheaper.** Required to use GoForge, agents
  spent 30% less, used 26% fewer tokens and 16% fewer turns, finished 20% faster, and every run's
  own tests passed (8 of 10 before). On the largest task, `store` (five resources on PostgreSQL),
  cost fell 55% ($0.239 to $0.107), tokens 59%, and turns from 16.5 to 9.5; that is also 33%
  cheaper and 30% faster than writing it from scratch ($0.159).
- **Across all tasks, building with GoForge now costs about the same as writing from scratch**
  ($0.122 against $0.118) and is 11% faster, but reads more tokens (mostly cached reads of a
  larger project). What the agent ends up with differs: GoForge projects come with migrations,
  Docker, CI, health checks, request IDs, and thousands of lines of tests, which the specs didn't
  ask for and the checks don't measure.
- **The benchmark found a GoForge bug.** In all four required runs of the `products` task, items
  created within the same millisecond were listed in the wrong order, because ties in
  `created_at` were broken by the random ID. It is fixed after `b632824` (timestamps now always
  increase).

**Asked for the same production features, scratch costs about twice as much and still tests less.**
The `scratch-production` condition adds one requirement to the prompt: the structure and
production features GoForge projects come with (separate packages, graceful shutdown, timeouts,
panic recovery, structured logs with request IDs, body limits, health endpoint, migrations,
Dockerfile, Makefile, CI). Three runs only, so treat it as indicative:

| Task | Condition | Cost | Minutes | Checks | Features (of 11) | Code / test lines | Coverage* |
|---|---|---|---|---|---|---|---|
| store | scratch | $0.16 | 1.9 | 46/46 | 2 | 442 / 97 | 1% |
| store | scratch-production (2 runs) | $0.30 | 3.1 | 46/46 | 11 | 775 / 147 | 21% |
| store | GoForge required, v0.2.1 | $0.11 | 1.3 | 46/46 | 11 | 4,122 / 3,504 | 75% |
| products | scratch | $0.09 | 0.8 | 25/25 | 1 | 280 / 82 | 88% |
| products | scratch-production (1 run) | $0.22 | 1.6 | 25/25 | 10 | 731 / 249 | 82% |
| products | GoForge required, v0.2.1 | $0.07 | 0.8 | 23/25 | 9.5 | 1,189 / 1,068 | 86% |

\*Statement coverage of `go test ./...` without a database, so tests that need PostgreSQL don't
count for anyone. The `products` task stores items in memory, so it has no migrations. The
GoForge `products` runs predate the ordering fix, which caused their two failed checks.

Caveats: two runs per GoForge condition (the third round was stopped by the plan's usage limit,
and those runs were discarded; see below), one model, and differences of a cent or two are within
run-to-run variation.

The third round hit a usage limit partway through: Claude Code answered *"You've hit your session
limit"* without doing the task, which an earlier version of the harness recorded as a run. Those
16 runs were removed, and `bench` now stops on such errors instead. To add the missing runs:
`./bench run -first-run 3 -runs 1 -condition goforge -goforge-version <version>` (and
`goforge-required`).

## Skill and MCP server (October 2026)

GoForge at commit `08741df` (the version with the agent skill, `goforge mcp`, and OpenAPI), three
tasks (`products`, `blog`, `store`), two runs each, offered-only: the prompt is identical in all
three conditions and only what the agent has installed differs. Every run passed every check.

| Condition | Runs | Fully correct | Used GoForge | Cost | Tokens | Turns | Minutes |
|---|---|---|---|---|---|---|---|
| goforge (prompt only) | 6 | 6/6 | 0/6 | $0.12 | 72,540 | 4 | 1.8 |
| goforge-skill | 6 | 6/6 | 6/6 | $0.08 | 151,836 | 8 | 1.6 |
| goforge-mcp | 6 | 6/6 | 1/6 | $0.11 | 120,775 | 6 | 2.1 |

- **The skill gets agents to use GoForge** (6 of 6 runs, against 0 of 6 with the prompt alone).
  The MCP server alone barely did (1 of 6): the five tools were connected and visible, and agents
  still wrote the API by hand. Tools need to be paired with instructions on when to use them.
- **Using GoForge via the skill was cheaper on `products` and `store`** (`store`: $0.08 against
  $0.15) and about equal on `blog`, with more tokens read (a larger project). Two runs per cell:
  indicative, not conclusive.

## Method

An agent gets a written spec for a service and builds it in an empty directory. Each task runs in
three conditions, which differ only in the last sentence of the prompt and whether `goforge` is on
`PATH`:

- **scratch**: the agent writes everything itself.
- **goforge**: GoForge is installed, and the prompt says *"GoForge (`goforge`), a CLI that
  generates and extends Go REST API projects, is installed. Run `goforge --help` to see what it
  can do."* The agent decides whether to use it, so this measures whether agents adopt it.
- **goforge-required**: the prompt says *"Build the service with GoForge (`goforge`), a CLI that
  generates and extends Go REST API projects. It is installed: run `goforge --help` to see what it
  can do."* This measures GoForge's effect when it is used.

Two more conditions test whether the agent integrations raise adoption. Both use exactly the
**goforge** prompt, so the only difference is what the agent has installed, and neither runs with
`all`:

- **goforge-skill**: the [`goforge` skill](../skills/goforge/SKILL.md) that ships with the GoForge
  version under test is installed in the agent's `~/.claude/skills`.
- **goforge-mcp**: the GoForge MCP server (`goforge mcp`) is connected, through `--mcp-config`.

Claude Code lists what it started with, and the harness refuses to record a run unless the skill
loaded only in `goforge-skill` and the server connected only in `goforge-mcp`, so a failed load
can't pass as a result. MCP calls to `goforge_generate`, `goforge_add_resource`, and
`goforge_add_db` count as using GoForge, like the commands. Claude Code also ships built-in skills,
which every condition has.

The `goforge-required` condition was added after a smoke test: offered GoForge on the small `products` task, the
agent read `goforge --help` and wrote the service by hand. Without it, the benchmark would mostly
measure whether agents choose GoForge, not whether it helps.

Everything else is identical: the model, the spec, the tools the agent may use, the budget and time
limits, and the environment.

### Tasks

| Task | What it tests | Checks |
|---|---|---|
| [`products`](tasks/products/spec.md) | CRUD with validation rules beyond "required", pagination, PUT and PATCH | 25 |
| [`products-sqlite`](tasks/products-sqlite/spec.md) | The same, stored in SQLite and surviving a restart | 27 |
| [`blog`](tasks/blog/spec.md) | Two related resources: referential checks, a nested route, a delete conflict | 23 |
| [`orders`](tasks/orders/spec.md) | PostgreSQL, nested line items, a computed total, a status state machine | 20 |
| [`store`](tasks/store/spec.md) | A larger back office: five resources on PostgreSQL, with a cross-resource rule and a delete conflict | 46 |

The specs define every status code and response shape, so the checks can judge any
implementation. The agent never sees the checks.

### The environment

Every run happens in a Docker container built from one [`Dockerfile`](Dockerfile), so the
environment never changes between runs or conditions: Go 1.26.0 (`GOTOOLCHAIN=local`, so no other
toolchain is downloaded), Node.js 22, a pinned Claude Code version (default 2.1.308), GoForge
(default `v0.2.0`), `psql`, and `sqlite3`. Each result records the image ID.

GoForge is installed in the image in every condition, but on the agent's `PATH` only in the two
goforge conditions. Each run starts with an empty home directory and Go module cache. The `orders`
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
- **Used GoForge**: whether the agent generated code with GoForge (`goforge generate` or
  `goforge add`). Only reading `goforge --help` doesn't count.

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
paying an order twice, a product in an unknown category, deleting a category that has products);
each was caught. The double-payment bug wasn't caught at first, which led
to a new check.

### What favors whom

- The list response (`items`, `total`, `limit`, `offset`), the default and maximum page sizes, and
  the error body (`{"error": ...}`) match GoForge's conventions, so a GoForge project meets them
  without changes. They are also common conventions a from-scratch agent can follow directly from
  the spec.
- Every task needs rules GoForge doesn't generate: name length and price limits, email format,
  referential checks and conflicts, a nested route, line items, a computed total, and a state
  machine. The `orders` task in particular doesn't fit GoForge's resources well (nested items and
  status transitions), so it shows where GoForge helps less. The `store` task is the opposite: most
  of its work is the CRUD, validation, and storage GoForge generates, as in many real back offices.
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

# Run agents: every task, all three conditions, 3 runs each
./bench run -task all -condition all -runs 3 -model sonnet -budget 5

# Summarize results/ into results/REPORT.md
./bench report
```

`bench run` builds the image the first time, writes one JSON file per run to `results/<task>/`,
and keeps each run's prompt, transcript, and code in a run directory (`-work`, by default
`~/.cache/goforge-bench`); each run's Go caches are deleted after scoring. Runs alternate between conditions, so changes over time affect every condition alike.


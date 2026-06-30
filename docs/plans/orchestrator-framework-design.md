# Orchestrator Framework: Revised Architecture

## Vision

A lightweight, general-purpose Go orchestrator that coordinates multiple coding
agents through a structured workflow. It wraps AO (Agent Orchestrator) as its
runtime layer — AO handles worktrees, hooks, and PR observation; the
orchestrator handles workflow phases, gates, and multi-agent coordination.

**Not bchat-specific. Not Hermes-specific. Not AO-specific.**

## The Layer Cake

```
┌─────────────────────────────────────────────────────────────────────┐
│                    Hermes Agent (orchestrator mode)                 │
│                                                                     │
│  You talk to Hermes. Hermes talks to the orch library via Go API.   │
│  Hermes presents results, waits for your approval, drives phases.   │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ Go API
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                   Orchestrator Framework (orch)                     │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐ │
│  │  Workflow     │  │  Gate        │  │  Transcript + Verdict    │ │
│  │  State Machine│  │  Enforcement │  │  Tracker                 │ │
│  └──────────────┘  └──────────────┘  └──────────────────────────┘ │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                    Agent Adapter Layer                        │  │
│  │                                                              │  │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐             │  │
│  │  │ Codex      │  │ Claude     │  │ Kilo Code  │  ...        │  │
│  │  │ Adapter    │  │ Adapter    │  │ Adapter    │             │  │
│  │  │            │  │            │  │            │             │  │
│  │  │ calls AO   │  │ calls AO   │  │ calls AO   │             │  │
│  │  │ HTTP API   │  │ HTTP API   │  │ HTTP API   │             │  │
│  │  └─────┬──────┘  └─────┬──────┘  └─────┬──────┘             │  │
│  └────────┼───────────────┼───────────────┼─────────────────────┘  │
└───────────┼───────────────┼───────────────┼────────────────────────┘
            │               │               │
            ▼               ▼               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    AO (Agent Orchestrator)                          │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐ │
│  │ Worktree     │  │ Terminal     │  │ Hook System              │ │
│  │ Manager      │  │ Multiplexer  │  │ (activity signals)       │ │
│  └──────────────┘  └──────────────┘  └──────────────────────────┘ │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐ │
│  │ SCM Observer │  │ Review       │  │ Lifecycle                │ │
│  │ (GitHub poll)│  │ Engine       │  │ Manager                  │ │
│  └──────────────┘  └──────────────┘  └──────────────────────────┘ │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐                               │
│  │ CDC + SSE    │  │ SQLite Store │                               │
│  └──────────────┘  └──────────────┘                               │
└─────────────────────────────────────────────────────────────────────┘
```

**Key insight:** The orchestrator does NOT replace AO. It wraps AO's HTTP API.
AO is the runtime (worktrees, tmux, hooks, GitHub polling). The orchestrator
is the coordinator (workflow phases, gates, verdicts, transcripts).

## What Each Layer Is Responsible For

| Concern | AO | Orchestrator | Hermes |
|---------|-----|--------------|--------|
| Git worktree lifecycle | ✅ owns | uses AO | — |
| tmux terminal management | ✅ owns | uses AO | — |
| Hook installation | ✅ owns | uses AO | — |
| Activity signal derivation | ✅ owns | uses AO | — |
| GitHub PR/CI observation | ✅ owns | uses AO | — |
| Auto-nudge on CI failure | ✅ owns | uses AO | — |
| Review trigger + routing | ✅ owns | uses AO | — |
| Session persistence (SQLite) | ✅ owns | uses AO | — |
| — | — | — | — |
| Workflow phase management | — | ✅ owns | drives |
| Gate enforcement | — | ✅ owns | triggers |
| Multi-agent coordination | — | ✅ owns | monitors |
| Transcript persistence | — | ✅ owns | reads |
| Verdict tracking | — | ✅ owns | evaluates |
| Harness-agnostic adapter | — | ✅ owns | — |
| — | — | — | — |
| User conversation | — | — | ✅ owns |
| Approval solicitation | — | — | ✅ owns |
| Result presentation | — | — | ✅ owns |

## Core Interfaces

```go
// Session identifies an agent session managed by AO.
type Session struct {
    ID            string            // AO session ID (e.g., "bchat-12")
    ProjectID     string            // AO project ID
    Role          string            // "planner" | "implementer" | "reviewer"
    Harness       string            // "codex" | "claude-code" | "kilocode"
    AoPort        int               // which AO daemon owns this session
    AoDataDir     string            // AO data directory
    WorktreePath  string            // git worktree path
    Branch        string            // git branch name
    Metadata      map[string]string // extensible
}

// Agent is anything that can do work in response to a prompt.
// Each adapter wraps AO's HTTP API for a specific harness.
type Agent interface {
    // Start spawns a new AO session with the given prompt.
    // Returns the session with AO-assigned ID.
    Start(ctx context.Context, prompt string) (Session, error)

    // Send delivers a message to the running session via AO's /send endpoint.
    // Blocks until the agent is idle, then sends.
    Send(ctx context.Context, session Session, message string) error

    // Receive waits for the agent to produce output.
    // Polls AO's session status until activity changes.
    Receive(ctx context.Context, session Session) (Response, error)

    // Status returns current session state from AO.
    Status(ctx context.Context, session Session) (State, error)

    // Kill terminates the session via AO.
    Kill(ctx context.Context, session Session) error
}

// Reviewer extends Agent with structured review capability.
// Uses AO's native review trigger under the hood.
type Reviewer interface {
    Agent

    // Review triggers AO's review engine for a PR or diff.
    // Returns structured verdict.
    Review(ctx context.Context, session Session, input ReviewInput) (Verdict, error)
}

// Workflow coordinates sessions through phases.
type Workflow interface {
    // Start begins a new workflow with a task description.
    Start(ctx context.Context, task string) error

    // CurrentPhase returns the active phase.
    CurrentPhase() Phase

    // Advance attempts to move to the next phase.
    // Checks the current phase's gate first.
    Advance(ctx context.Context) error

    // SendToActive sends a message to the current phase's session.
    SendToActive(ctx context.Context, message string) error

    // ReceiveFromActive waits for a response from the current phase's session.
    ReceiveFromActive(ctx context.Context) (Response, error)

    // Approve signals user approval for the current phase's gate.
    Approve(ctx context.Context) error

    // GetSession returns the session for a given phase.
    GetSession(phase Phase) (Session, bool)
}
```

## Adapter Design: Thin Wrappers Over AO

Each adapter is a thin wrapper around AO's HTTP API. The adapter translates
the harness-agnostic `Agent` interface into AO-specific HTTP calls.

### Codex Adapter

```go
type CodexAdapter struct {
    aoPort      int
    projectID   string
    harness     string     // "codex"
    permissions string     // "bypass" | "default" | "strict"
}

func (a *CodexAdapter) Start(ctx context.Context, prompt string) (Session, error) {
    // POST http://127.0.0.1:{aoPort}/api/v1/sessions
    // {
    //   "projectId": "bchat",
    //   "kind": "worker",
    //   "harness": "codex",
    //   "prompt": prompt
    // }
    // → returns session ID, worktree path, branch
}

func (a *CodexAdapter) Send(ctx context.Context, session Session, message string) error {
    // 1. Poll GET /api/v1/sessions/{id} until activity.state == "idle"
    // 2. POST /api/v1/sessions/{id}/send {"message": message}
    // 3. Return
}

func (a *CodexAdapter) Receive(ctx context.Context, session Session) (Response, error) {
    // 1. Poll GET /api/v1/sessions/{id} until activity.state == "working"
    //    (agent received our message and started working)
    // 2. Poll again until activity.state == "idle" (agent finished)
    // 3. Capture tmux pane output via AO terminal mux WebSocket
    // 4. Parse output into Response
}

func (a *CodexAdapter) Status(ctx context.Context, session Session) (State, error) {
    // GET /api/v1/sessions/{id}
    // → map activity.state to State.Status
}

func (a *CodexAdapter) Kill(ctx context.Context, session Session) error {
    // POST /api/v1/sessions/{id}/kill
}
```

### Kilo Code Adapter

Same structure as Codex, but:
- `harness: "kilocode"`
- Can optionally use `kilocode acp --port <n>` for structured JSON messaging
  instead of tmux pane capture
- Falls back to tmux if ACP is unavailable

### Claude Code Adapter

Same structure, `harness: "claude-code"`.

### Shell Adapter

For any CLI agent without a dedicated AO harness:
```go
type ShellAdapter struct {
    command string  // e.g., "aider" or "grep -r FIXME"
}

func (a *ShellAdapter) Start(ctx context.Context, prompt string) (Session, error) {
    // Run command directly (not via AO).
    // Session.ID = PID, Session.WorktreePath = cwd.
}
```

## Workflow State Machine

```go
type Phase string

const (
    PhaseIdle           Phase = "idle"
    PhasePlanning       Phase = "planning"
    PhasePlanReview     Phase = "plan_review"
    PhaseImplementing   Phase = "implementing"
    PhaseCodeReview     Phase = "code_review"
    PhaseDone           Phase = "done"
)

type GateType string

const (
    GateNone            GateType = "none"              // auto-advance
    GateUserApprove     GateType = "user_approve"      // user must say APPROVED
    GateVerdictApprove  GateType = "verdict_approve"   // reviewer must say APPROVED
    GatePRSubmitted     GateType = "pr_submitted"      // PR must exist
    GatePRApproved      GateType = "pr_approved"       // PR must be approved
)

type PhaseConfig struct {
    Name        string
    Role        string
    Prompt      string
    Gate        GateType
    MaxRounds   int           // max review rounds before forced advance
    DaemonPort  int           // which AO daemon to use for this phase
}

type PhaseState struct {
    Phase       Phase
    Session     Session
    Status      string        // "pending" | "active" | "completed" | "failed"
    Rounds      int
    Verdict     string
    Transcript  string        // path to transcript file
}

// Valid state transitions
var validTransitions = map[Phase]map[Phase]bool{
    PhaseIdle:         {PhasePlanning: true},
    PhasePlanning:     {PhasePlanReview: true, PhaseDone: true},
    PhasePlanReview:   {PhaseImplementing: true, PhasePlanning: true},
    PhaseImplementing: {PhaseCodeReview: true, PhaseDone: true},
    PhaseCodeReview:   {PhaseDone: true, PhaseImplementing: true},
}
```

## Gate Enforcement

Gates are enforced by the orchestrator's state machine, not by prompt text:

```go
func (w *Workflow) CheckGate(ctx context.Context, phase Phase) (bool, error) {
    config := w.config.GetPhaseConfig(phase)
    state := w.phases[phase]

    switch config.Gate {
    case GateNone:
        return true, nil

    case GateUserApprove:
        // The user (via Hermes or CLI) must explicitly call Approve().
        // This is tracked in the PhaseState.
        return state.userApproved, nil

    case GateVerdictApprove:
        if state.Verdict == "approved" {
            return true, nil
        }
        if state.Rounds >= config.MaxRounds {
            return false, fmt.Errorf("max review rounds (%d) exceeded", config.MaxRounds)
        }
        return false, nil

    case GatePRSubmitted:
        // Check if a PR exists for the implementer's branch.
        // Use AO's session metadata (PR association) or gh CLI.
        return w.checkPRExists(ctx, state.Session)

    case GatePRApproved:
        // Check if PR has been approved (via CI, reviewer, etc.)
        return w.checkPRApproved(ctx, state.Session)
    }

    return false, fmt.Errorf("unknown gate type: %s", config.Gate)
}
```

## Configuration

`orch.yaml` — placed in any project root:

```yaml
# orch.yaml — orchestrator configuration
# This file lives in YOUR repo, not in the orchestrator codebase.

project:
  name: "my-project"
  repo: "https://github.com/user/repo"
  default_branch: "main"

# AO daemon configuration
# Each phase can use a different daemon for isolation.
daemons:
  planner:
    port: 3001
    data_dir: ".orch/planner-data"
  builder:
    port: 3002
    data_dir: ".orch/builder-data"
  reviewer:
    port: 3003
    data_dir: ".orch/reviewer-data"

# Role → Agent binding (fully swappable)
roles:
  planner:
    adapter: "codex"          # codex | claude | kilo | shell
    harness: "codex"          # AO harness name
    permissions: "bypass"     # bypass | default | strict
    daemon: "planner"         # which AO daemon to use

  implementer:
    adapter: "codex"
    harness: "codex"
    permissions: "default"
    daemon: "builder"

  reviewer:
    adapter: "kilo"           # different harness = adversarial independence
    harness: "kilocode"
    permissions: "bypass"
    daemon: "reviewer"

# Workflow definition
workflow:
  phases:
    - name: planning
      role: planner
      prompt: "prompts/planner.md"
      gate: "user_approve"

    - name: plan_review
      role: reviewer
      prompt: "prompts/plan_reviewer.md"
      gate: "verdict_approve"
      max_rounds: 5

    - name: implementing
      role: implementer
      prompt: "prompts/implementer.md"
      gate: "pr_submitted"

    - name: code_review
      role: reviewer
      prompt: "prompts/code_reviewer.md"
      gate: "verdict_approve"
      max_rounds: 5

# Transcript storage
transcripts:
  format: "markdown"
  path: ".orch/transcripts"

# State persistence
state:
  backend: "json"             # json | sqlite | memory
  path: ".orch/state"
```

## Project Structure

```
orchestrator/
├── go.mod                          # module: github.com/...
├── cmd/
│   └── orch/
│       └── main.go                 # CLI entry point
├── core/
│   ├── agent.go                    # Agent interface
│   ├── reviewer.go                 # Reviewer interface
│   ├── workflow.go                 # Workflow state machine
│   ├── phase.go                    # Phase + Gate types
│   ├── session.go                  # Session struct
│   ├── response.go                 # Response + State + Finding
│   ├── verdict.go                  # Verdict type
│   ├── transcript.go               # Transcript storage
│   └── config.go                   # Config loading
├── adapters/
│   ├── codex.go                    # Codex adapter (wraps AO HTTP API)
│   ├── claude.go                   # Claude Code adapter
│   ├── kilo.go                     # Kilo Code adapter (ACP + fallback)
│   └── shell.go                    # Generic shell command adapter
├── storage/
│   ├── json_store.go               # JSON file state (default)
│   ├── sqlite_store.go             # SQLite state (optional)
│   └── memory_store.go             # In-memory (testing)
├── prompts/
│   ├── planner.md                  # Default planner prompt
│   ├── plan_reviewer.md            # Default plan reviewer prompt
│   ├── implementer.md              # Default implementer prompt
│   └── code_reviewer.md            # Default code reviewer prompt
└── examples/
    ├── bchat/
    │   └── orch.yaml
    └── simple/
        └── orch.yaml
```

## CLI Usage

```bash
# Initialize a new project
orch init
# → creates orch.yaml, prompts/, .orch/

# Start a workflow
orch start "Add hybrid search to the RAG pipeline"
# → spawns planner on AO daemon :3001, begins Q&A

# Interact with the active session
orch send "Let's use LanceDB for vector storage"
# → delivers message to current phase's agent via AO

# Check status
orch status
# → Phase: PLANNING | Session: bchat-12 | Status: working | Harness: codex

# Approve and advance
orch approve
# → signals gate approval, advances to PLAN_REVIEW
# → spawns reviewer on AO daemon :3003 (kilocode)

# View transcript
orch transcript bchat-12
# → shows full conversation history

# Swap harness for a role (no code changes)
orch config set roles.implementer.adapter claude
# → next implementer session uses Claude Code

# List all sessions across all daemons
orch sessions
# → bchat-12 (planner, codex, :3001, idle)
# → bchat-15 (reviewer, kilocode, :3003, working)
```

## Hermes Integration

### Hermes as Orchestrator

When Hermes drives the workflow:

```go
// In Hermes's Go code (or via CLI calls):
import "github.com/.../orchestrator/core"

func main() {
    wf, _ := core.LoadWorkflow("orch.yaml")
    wf.Start(ctx, "Add hybrid search to RAG pipeline")

    for wf.CurrentPhase() != core.PhaseDone {
        // Present agent output to user
        resp, _ := wf.ReceiveFromActive(ctx)
        presentToUser(resp.Text)

        // Wait for user input
        input := getUserInput()
        wf.SendToActive(ctx, input)

        // Check if user approved
        if input == "APPROVED" {
            wf.Approve(ctx)
            wf.Advance(ctx)  // moves to next phase, spawns next agent
        }
    }
}
```

In practice, Hermes calls the `orch` CLI as a subprocess (simpler than
embedding the Go library):

```bash
# Hermes drives the workflow via CLI
orch start "Add hybrid search to RAG pipeline"
# Hermes polls for output
orch receive
# Hermes presents to user, gets response
orch send "Let's use LanceDB"
# User approves
orch approve
# → advances to next phase
```

### Hermes as Worker

When Hermes is a worker (e.g., "Hermes, analyze this codebase"):

Some other orchestrator instance spawns Hermes via the shell adapter or a
dedicated Hermes adapter. Hermes is treated like any other agent — it
receives a prompt, does the work, returns a response.

## Swapability Matrix

| Component | Current | Swap To | How |
|-----------|---------|---------|-----|
| Planner harness | codex | claude-code | `orch.yaml`: `roles.planner.adapter: claude` |
| Implementer harness | codex | claude-code | `orch.yaml`: `roles.implementer.adapter: claude` |
| Reviewer harness | kilocode | claude-code | `orch.yaml`: `roles.reviewer.adapter: claude` |
| Planner daemon | :3001 | :3004 | `orch.yaml`: `daemons.planner.port: 3004` |
| State backend | json | sqlite | `orch.yaml`: `state.backend: sqlite` |
| Transcript format | markdown | json | `orch.yaml`: `transcripts.format: json` |
| Orchestrator | orch CLI | Hermes | Hermes calls `orch` CLI as subprocess |
| Any role | any adapter | shell | `orch.yaml`: `roles.X.adapter: shell` |

## Implementation Phases

| Phase | Deliverable | Validates |
|-------|-------------|-----------|
| 1 | Core types (Session, Response, State, Phase, Gate, Verdict) + state machine + JSON store | Workflow logic works without any agent |
| 2 | Codex adapter (wraps AO HTTP API) | Can spawn and talk to Codex via AO |
| 3 | Kilo Code adapter (wraps AO HTTP API, optional ACP) | Can spawn and talk to Kilo via AO |
| 4 | Shell adapter | Can spawn any CLI agent |
| 5 | CLI (`orch start`, `orch send`, `orch status`, `orch approve`) | Human-usable |
| 6 | Gate enforcement end-to-end | Full workflow runs with structural gates |
| 7 | Transcript persistence | Conversations survive restarts |
| 8 | Multi-daemon support (planner + builder + reviewer) | Isolated phases work |
| 9 | Example configs (bchat + simple) | Real-world validation |

## What This Is NOT

- **Not a replacement for AO.** AO is the runtime. The orchestrator is the
  coordinator. The orchestrator calls AO's HTTP API for everything related to
  session management, worktrees, hooks, and GitHub observation.

- **Not bchat-specific.** The framework is generic. bchat is just the first
  example config.

- **Not Hermes-specific.** Hermes is the current orchestrator-in-practice,
  but the `orch` CLI can be driven by a human, a CI pipeline, or any other
  agent.

- **Not a chat platform.** The orchestrator provides a library + CLI. Chat
  UIs (like Hermes) sit on top and drive the CLI.

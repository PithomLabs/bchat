./memos --mode dev --driver sqlite --data ./data

Then open http://localhost:5230 - the frontend is served from the embedded files!


How to Test
-----------

From memos-0244 
```
./memos --mode dev --driver sqlite --data ./data
```

In another terminal
```

```
cd web && npm run dev
```

# Implementation Plan - Ensure Frontend Embedding

This plan outlines the steps to ensure that the frontend assets are always embedded into the Go binary when compiling `bin/memos/main.go`.

## Proposed Changes

### [Backend]

#### [MODIFY] [frontend.go](file:///home/chaschel/Documents/ibm/go/tix-gemini/server/router/frontend/frontend.go)

- Add a `go:generate` directive to automate the frontend build and sync process.
- This will ensure that running `go generate ./...` (or specifically on this package) will populate the `dist` directory before the Go compiler embeds it.

```go
//go:generate npm --prefix ../../../web install
//go:generate npm --prefix ../../../web run release
```

> [!NOTE]
> The `release` script in `web/package.json` is already configured to output to `../server/router/frontend/dist`.

## Verification Plan

### Automated Tests
1. Run `go generate ./server/router/frontend/frontend.go` to verify the frontend builds and populates the `dist` directory.
2. Run `go build -o memos ./bin/memos/main.go`.
3. Verify the binary contains the frontend assets:
   - Check binary size.
   - Run the binary and access the web interface (if possible in the environment).
   - Alternatively, use `strings memos | grep index.html` or similar to check for embedded assets.

### Manual Verification
- The user can run `npm run release` in the `web` directory followed by `go build ./bin/memos` to verify the single binary deployment.



### [Automation]

#### [NEW] [Taskfile.yml](file:///home/chaschel/Documents/ibm/go/tix-gemini/Taskfile.yml)

- Create a `Taskfile.yml` to automate all repetitive commands.
- Include tasks for:
    - `setup`: Install dependencies for both frontend and backend.
    - `build:frontend`: Build the frontend assets.
    - `build:backend`: Build the Go binary.
    - `build`: Build everything (frontend + backend).
    - `dev:frontend`: Run the frontend dev server.
    - `dev:backend`: Run the backend dev server.
    - `generate`: Run `go generate ./...`.

```yaml
version: '3'

tasks:
  setup:
    desc: Install all dependencies
    cmds:
      - npm --prefix web install
      - go mod download

  build:frontend:
    desc: Build frontend assets
    cmds:
      - npm --prefix web run release

  build:backend:
    desc: Build the Go binary
    cmds:
      - go build -o build/memos ./bin/memos/main.go

  build:
    desc: Build everything (frontend + backend)
    deps: [build:frontend, build:backend]

  dev:frontend:
    desc: Run frontend dev server
    cmds:
      - npm --prefix web run dev

  dev:backend:
    desc: Run backend dev server
    cmds:
      - go run ./bin/memos/main.go --mode dev

  generate:
    desc: Run go generate
    cmds:
      - go generate ./...
```

## Verification Plan

### Automated Tests
1. Run `go generate ./server/router/frontend/frontend.go` to verify the frontend builds and populates the `dist` directory.
2. Run `go build -o memos ./bin/memos/main.go`.
3. Verify the binary contains the frontend assets:
   - Check binary size.
   - Run the binary and access the web interface (if possible in the environment).
   - Alternatively, use `strings memos | grep index.html` or similar to check for embedded assets.

### Manual Verification
- The user can run `npm run release` in the `web` directory followed by `go build ./bin/memos` to verify the single binary deployment.


### Build & Embedding

#### [frontend.go](file:///home/chaschel/Documents/ibm/go/tix-gemini/server/router/frontend/frontend.go)

Added `go:generate` directives to automate the frontend build and ensure assets are embedded.

```go
//go:generate npm --prefix ../../../web install
//go:generate npm --prefix ../../../web run release
//go:embed dist/*
var embeddedFiles embed.FS
```

### Automation

#### [Taskfile.yml](file:///home/chaschel/Documents/ibm/go/tix-gemini/Taskfile.yml)

Created a `Taskfile.yml` to streamline development and deployment.

```yaml
version: '3'

tasks:
  setup:
    desc: Install all dependencies
    # ...
  build:
    desc: Build everything (frontend + backend)
    # ...
  dev:backend:
    desc: Run backend dev server
    # ...
```

## Verification Results

1. **Text Replacement**: Verified the "Description (Memo URL)" to "Description" change.
2. **Frontend Build**: Successfully ran `npm run release` (via `task build:frontend`).
3. **Binary Compilation**: Successfully built the binary using `task build:backend`.
4. **Taskfile Automation**: Verified the full build pipeline with `task build`.
5. **Embedding Check**: Confirmed embedded assets in the final binary.



# Implement Mention Notifications

# Goal Description
Implement a notification system where users are notified when mentioned (`@nickname`) in ticket descriptions or comments. Clicking the notification should take the user to the **Ticket View**.

## User Review Required
> [!NOTE]
> We are using **Option B (Zero-Dependency)**. Mentions will be stored and delivered acting as "Comments" in the backend to avoid modifying Protocol Buffers. This means the UI might label them as "Comment" notifications initially, but we will handle the redirection logic.

## Proposed Changes

### Backend
#### [MODIFY] [memo_service.go](file:///home/chaschel/Documents/ibm/go/tix-gemini-master/server/router/api/v1/memo_service.go)
- Implement `dispatchMemoMentions(ctx, memo)` helper function.
- Call this function in `CreateMemo` and `CreateMemoComment`.
- Logic:
    - Parse content for `@nickname`.
    - Resolve users.
    - Create `Activity` (Type: `MEMO_COMMENT`).
    - Create `Inbox` (Type: `MEMO_COMMENT`).

### Frontend
#### [MODIFY] [MemoCommentMessage.tsx](file:///home/chaschel/Documents/ibm/go/tix-gemini-master/web/src/components/Inbox/MemoCommentMessage.tsx)
- Update `handleNavigateToMemo` to intercept the navigation.
- Logic:
    - Fetch tickets (or use a lightweight check).
    - If `relatedMemo` corresponds to a Ticket's description (Memo link), redirect to `/tickets?id=X`.
    - Otherwise, redirect to the Memo as usual.

#### [NEW] [MentionSuggestions.tsx](file:///home/chaschel/Documents/ibm/go/tix-gemini-master/web/src/components/MemoEditor/Editor/MentionSuggestions.tsx)
- Create a new component similar to `TagSuggestions.tsx`.
- Connect to `userStore` to fetch users.
- Show autocomplete list when typing `@`.
- Insert `@nickname` (or username) on selection.

#### [MODIFY] [Editor/index.tsx](file:///home/chaschel/Documents/ibm/go/tix-gemini-master/web/src/components/MemoEditor/Editor/index.tsx)
- Import and render `MentionSuggestions` alongside `TagSuggestions`.

## Verification Plan

### Manual Verification
1.  **Setup**: User A and User B.
2.  **Case 1: Ticket Description Mention**:
    - User A creates a ticket. In the description, types `@UserB check this`.
    - User B logs in. Checks Inbox.
    - Expectation: Notification "User A commented on...".
    - Action: Click notification.
    - Expectation: Redirect to `Tickets` page, ideally opening the specific ticket.
3.  **Case 2: Comment Mention**:
    - User A comments on a ticket: `@UserB please fix`.
    - User B logs in. Checks Inbox.
    - Expectation: Notification present.
    - Action: Click notification.
    - Expectation: Redirect to `Tickets` page.

### Automated Tests
-   Verify backend parsing logic via unit test (if possible to add easily) or rely on manual verification given the "prototype" nature constraints.




---
beads integration

https://github.com/mantoni/beads-ui

npm i beads-ui -g
# In your project directory:
bdui start --open

http://127.0.0.1:3000/#/board



----
prompt for beads integration

1. copy AGENTS.MD in base folder to your repo folder
2. in antigravity, enter this prompt: read AGENTS.MD and strictly follow it using its beads workflow guidelines to fix the error, ask me if you need clarification
# Windows PowerShell Taskfile

Create `Taskfilewin.yml` — a drop-in replacement for [Taskfile.yml](file:///home/chaschel/Documents/go/bchat/Taskfile.yml) that uses PowerShell instead of bash. The user will rename it to [Taskfile.yml](file:///home/chaschel/Documents/go/bchat/Taskfile.yml) when working on Windows.

## Proposed Changes

### Taskfilewin.yml

#### [NEW] [Taskfilewin.yml](file:///home/chaschel/Documents/go/bchat/Taskfilewin.yml)

A complete port of all tasks from [Taskfile.yml](file:///home/chaschel/Documents/go/bchat/Taskfile.yml), with these key differences:

| Aspect | Linux (existing) | Windows (new) |
|---|---|---|
| **Shell** | [sh](file:///home/chaschel/Documents/go/bchat/scripts/build.sh) (default) | `powershell` via Taskfile `set:` |
| **Binary output** | [build/memos](file:///home/chaschel/Documents/go/bchat/build/memos) | `build/memos.exe` |
| **mkdir** | `mkdir -p` | `New-Item -ItemType Directory -Force` |
| **env sourcing** | `source .env` | `Get-Content .env \| ForEach-Object { ... }` |
| **test -f** | `test -f file` | `Test-Path file` |
| **LanceDB download** | `curl \| bash` script | `scripts/download-lancedb.ps1` |
| **Migrations validate** | [scripts/validate-migrations.sh](file:///home/chaschel/Documents/go/bchat/scripts/validate-migrations.sh) | `scripts/validate-migrations.ps1` |
| **CGO_LDFLAGS** | Linux/Darwin `-rpath` / `-framework` | Windows: just the `.a` static lib path |
| **LD_LIBRARY_PATH** | Used in `run:binary` | `$env:PATH` prepend instead |

**Tasks ported** (same names as original):
- `setup`, `setup:lancedb`, `validate:migrations`, `validate:schema`
- `build:frontend`, `build:backend`, `build:backend:rag`, `build:widget`
- `build`, `build:all`, `build:rag`, `build:rag:all`
- `run`, `run:rag`, `run:binary`, `run:rag:l12`
- `generate`
- fly.io tasks are **excluded** (Linux server deployment only)

---

### PowerShell Scripts

#### [NEW] [download-lancedb.ps1](file:///home/chaschel/Documents/go/bchat/scripts/download-lancedb.ps1)

PowerShell equivalent of the upstream `download-artifacts.sh`. Downloads:
1. Static library (`liblancedb_go.a`) to `lib/windows_amd64/`
2. Dynamic library (`lancedb_go.dll`) to `lib/windows_amd64/`
3. Header file (`lancedb.h`) to `include/`

Falls back to downloading the full tarball archive if individual files fail.

#### [NEW] [validate-migrations.ps1](file:///home/chaschel/Documents/go/bchat/scripts/validate-migrations.ps1)

PowerShell equivalent of [scripts/validate-migrations.sh](file:///home/chaschel/Documents/go/bchat/scripts/validate-migrations.sh). Validates `LATEST.sql` is in sync with migration files by checking for missing tables and columns.

## Verification Plan

### Automated Tests
- Run `task --taskfile Taskfilewin.yml --list` (or equivalent) to verify YAML syntax parses correctly
- Since actual execution requires Windows, we verify structural correctness only

### Manual Verification
- User renames `Taskfilewin.yml` to [Taskfile.yml](file:///home/chaschel/Documents/go/bchat/Taskfile.yml) on Windows and runs `task build:rag:all`

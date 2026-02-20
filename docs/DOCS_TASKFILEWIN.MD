# Windows Taskfile Documentation 

This document covers the Windows PowerShell equivalent of the project's `Taskfile.yml`.

## Overview

The project ships two Taskfiles:

| File | Shell | Platform |
|---|---|---|
| `Taskfile.yml` | bash / sh | Linux, macOS |
| `Taskfilewin.yml` | PowerShell | Windows |

On Windows, rename `Taskfilewin.yml` → `Taskfile.yml` before use.

## Prerequisites

- [Task](https://taskfile.dev) CLI installed (`winget install Task.Task` or `choco install go-task`)
- [Go](https://go.dev/dl/) with CGO support (requires a C compiler — install via [MSYS2](https://www.msys2.org/) or [TDM-GCC](https://jmeubank.github.io/tdm-gcc/))
- [Node.js / npm](https://nodejs.org/) for frontend builds
- PowerShell 5.1+ (ships with Windows 10/11)

## Quick Start

```powershell
# Rename the Windows Taskfile
Rename-Item Taskfilewin.yml Taskfile.yml

# Install dependencies
task setup

# Build everything with RAG + widget
task build:rag:all

# Run with RAG enabled
task run:rag
```

## Available Tasks

### Setup

| Task | Description |
|---|---|
| `setup` | Install npm and Go dependencies |
| `setup:lancedb` | Download LanceDB native libraries via `download-lancedb.ps1` |

### Validation

| Task | Description |
|---|---|
| `validate:migrations` | Validate `LATEST.sql` is in sync with migration files |
| `validate:schema` | Run schema validation Go tests |

### Build

| Task | Description | Deps |
|---|---|---|
| `build:frontend` | Build frontend assets | — |
| `build:backend` | Build `build\memos.exe` | `validate:migrations` |
| `build:backend:rag` | Build `build\memos.exe` with LanceDB RAG | `setup:lancedb`, `validate:migrations` |
| `build:widget` | Build embeddable chat widget | — |
| `build` | Frontend + backend | `build:frontend`, `build:backend` |
| `build:all` | Frontend + backend + widget | `build:frontend`, `build:backend`, `build:widget` |
| `build:rag` | Frontend + backend with RAG | `build:frontend`, `build:backend:rag` |
| `build:rag:all` | Frontend + backend with RAG + widget | `build:frontend`, `build:backend:rag`, `build:widget` |

### Run

| Task | Description | Deps |
|---|---|---|
| `run` | Dev server | `build:backend` |
| `run:rag` | Dev server with RAG (qwen3-embedding-8b) | `build:backend:rag` |
| `run:binary` | Run pre-compiled binary with RAG (no build) | — |
| `run:rag:l12` | Dev server with RAG + L12 embeddings | `build:backend:rag` |

### Other

| Task | Description |
|---|---|
| `generate` | Run `go generate ./...` |

> **Note:** Fly.io deployment tasks (`fly:check`, `fly:pre-deploy`, etc.) are not included in the Windows Taskfile since deployments target Linux servers.

## PowerShell Scripts

### `scripts/download-lancedb.ps1`

PowerShell equivalent of the upstream [download-artifacts.sh](https://github.com/lancedb/lancedb-go/blob/main/scripts/download-artifacts.sh).

**What it does:**
1. Detects architecture (amd64 / arm64)
2. Downloads static library (`liblancedb_go.a`) to `lib/windows_amd64/`
3. Downloads dynamic library (`lancedb_go.dll`) to `lib/windows_amd64/`
4. Downloads header (`lancedb.h`) to `include/`
5. Falls back to full archive download if individual files fail

**Manual usage:**
```powershell
.\scripts\download-lancedb.ps1 v0.1.2      # specific version
.\scripts\download-lancedb.ps1              # latest release
```

### `scripts/validate-migrations.ps1`

PowerShell equivalent of `scripts/validate-migrations.sh`.

**What it does:**
- Reads `store/migration/sqlite/LATEST.sql`
- Scans migration files in `store/migration/sqlite/0.25/`
- Reports any `CREATE TABLE` or `ADD COLUMN` entries missing from `LATEST.sql`
- Ignores temporary tables (`_new`, `_old`, `_backup`, `_temp`, `_tmp` suffixes)

## Key Differences from Linux Taskfile

| Aspect | Linux | Windows |
|---|---|---|
| Shell | bash (default) | PowerShell (`set: [POWERSHELL]`) |
| Binary output | `build/memos` | `build\memos.exe` |
| Directory creation | `mkdir -p` | `New-Item -ItemType Directory -Force` |
| `.env` loading | `set -a && source .env && set +a` | Inline PowerShell parser |
| LanceDB download | `curl \| bash` upstream script | `download-lancedb.ps1` |
| CGO_LDFLAGS | `-L... -rpath` (Linux) / `-framework` (macOS) | Static lib path only |
| Library path at runtime | `LD_LIBRARY_PATH` | `$env:PATH` prepend |
| Migration validation | `validate-migrations.sh` | `validate-migrations.ps1` |

## Troubleshooting

### CGO / C Compiler Not Found

Ensure a C compiler is on your `PATH`. Install via:
```powershell
# Option 1: MSYS2 (recommended)
pacman -S mingw-w64-x86_64-gcc

# Option 2: TDM-GCC
# Download from https://jmeubank.github.io/tdm-gcc/
```

Verify with:
```powershell
gcc --version
go env CC         # should show gcc or cc
```

### LanceDB Download Fails

If GitHub rate-limits your requests, set a `GITHUB_TOKEN`:
```powershell
$env:GITHUB_TOKEN = "ghp_..."
.\scripts\download-lancedb.ps1 v0.1.2
```

### ExecutionPolicy Blocks Scripts

If PowerShell refuses to run `.ps1` files:
```powershell
Set-ExecutionPolicy -Scope CurrentUser -ExecutionPolicy RemoteSigned
```

---

## Batch Files (Alternative to Task CLI)

Instead of using Task CLI, you can use the provided batch files for building and running:

| File | Purpose |
|------|---------|
| `build-windows-rag.bat` | Downloads LanceDB libraries and builds with RAG support |
| `run-windows-rag.bat` | Runs the application with RAG enabled |

### Build Steps

1. **Copy batch files** to your bchat project folder on Windows
2. **Open Command Prompt** and navigate to the bchat folder
3. **Run the build**:
```cmd
cd C:\path\to\bchat
build-windows-rag.bat
```
4. **Run with RAG**:
```cmd
run-windows-rag.bat
```

### Manual Run (Alternative)

If you prefer to run manually without the batch file:
```cmd
set RAG_PIPELINE_ENABLED=true
set LANCEDB_STORAGE_PROVIDER=local
set EMBEDDING_MODEL=qwen/qwen3-embedding-8b
build\memos.exe --mode dev --data build\data
```


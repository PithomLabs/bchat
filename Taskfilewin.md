# Windows PowerShell Taskfile — Walkthrough

## Files Created

| File | Purpose |
|---|---|
| [Taskfilewin.yml](file:///home/chaschel/Documents/go/bchat/Taskfilewin.yml) | Drop-in replacement for [Taskfile.yml](file:///home/chaschel/Documents/go/bchat/Taskfile.yml) using PowerShell |
| [download-lancedb.ps1](file:///home/chaschel/Documents/go/bchat/scripts/download-lancedb.ps1) | Downloads LanceDB native libs from GitHub releases |
| [validate-migrations.ps1](file:///home/chaschel/Documents/go/bchat/scripts/validate-migrations.ps1) | Validates LATEST.sql against migration files |

## Key Differences from Linux Taskfile

- **Shell**: Uses `set: [POWERSHELL]` so all commands run in PowerShell
- **Binary**: Outputs `build\memos.exe` instead of [build/memos](file:///home/chaschel/Documents/go/bchat/build/memos)
- **LanceDB download**: Calls [download-lancedb.ps1](file:///home/chaschel/Documents/go/bchat/scripts/download-lancedb.ps1) instead of piping `curl | bash`
- **CGO_LDFLAGS**: Simplified to just the static lib path (no `-rpath` or `-framework` flags)  
- **`.env` loading**: PowerShell inline parsing replaces `source .env`
- **`run:binary`**: Prepends `$env:PATH` instead of `LD_LIBRARY_PATH`
- **Excluded**: fly.io deployment tasks (Linux server only)

## Usage

On Windows, rename and run:
```powershell
Rename-Item Taskfilewin.yml Taskfile.yml
task build:rag:all    # full build with RAG + widget
task run:rag          # run with RAG enabled
```

## Verification

- YAML syntax validated with Python `yaml.safe_load()` — **passed**
- Full runtime verification requires Windows + Go CGO toolchain

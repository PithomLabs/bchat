# HTML to Markdown Converter — `html2md.sh`

A bash script that recursively converts all `.html` files in a given folder to Markdown (`.md`) using the [`html-to-markdown`](https://github.com/JohannesKaufmann/html-to-markdown) CLI tool.

---

## Prerequisites

- **Linux or macOS** (uses `find`, `realpath`, `bash`)
- **`html2markdown`** must be installed and available in your `PATH`

Verify it's working:

```bash
which html2markdown
html2markdown --help
```

---

## Quick Start

```bash
# Make the script executable (one-time)
chmod +x html2md.sh

# Convert all .html files under a folder
./html2md.sh /path/to/html/folder
```

---

## Usage

```
./html2md.sh <input_folder> [output_folder]
```

| Argument | Required | Description |
|---|---|---|
| `input_folder` | **Yes** | Folder containing `.html` files (searched recursively) |
| `output_folder` | No | Where `.md` files are saved. Defaults to `<input_folder>_md` |

---

## Examples

### 1. Default Output Folder

```bash
./html2md.sh ./help.browse.ai
```

- **Input:** `./help.browse.ai/`
- **Output:** `./help.browse.ai_md/`

### 2. Custom Output Folder

```bash
./html2md.sh ./help.browse.ai ./converted_docs
```

- **Input:** `./help.browse.ai/`
- **Output:** `./converted_docs/`

### 3. Absolute Paths

```bash
./html2md.sh /home/chaschel/websites/Browse\ AI/help.browse.ai /home/chaschel/output
```

### 4. Folder with Spaces in Name

```bash
./html2md.sh "./my html files" "./my markdown files"
```

---

## How It Works

1. **Scans** the input folder recursively for all `.html` files
2. **Converts** each file by running `html2markdown <file>.html` and capturing stdout
3. **Saves** the output as `<file>.md` in the output folder, preserving the original subdirectory structure
4. **Skips** files that fail conversion — no partial `.md` files are left behind
5. **Logs** any failed conversions with timestamps to `conversion_errors.log`
6. **Preserves** all original `.html` files (nothing is modified or deleted)

### Directory Structure Preservation

The subdirectory structure of the input folder is mirrored in the output folder:

```
Input:                              Output:
help.browse.ai/                     help.browse.ai_md/
├── index.html                      ├── index.md
├── getting-started/                ├── getting-started/
│   ├── setup.html                  │   ├── setup.md
│   └── config.html                 │   └── config.md
└── faq/                            └── faq/
    └── common.html                     └── common.md
                                        conversion_errors.log
```

---

## Output

### Console Output

During execution, the script prints progress for each file:

```
✓ index.html
✓ getting-started/setup.html
✗ faq/broken.html (logged)
✓ faq/common.html

Done. Output: /home/chaschel/websites/Browse AI/help.browse.ai_md
Errors logged to: /home/chaschel/websites/Browse AI/help.browse.ai_md/conversion_errors.log
```

| Symbol | Meaning |
|---|---|
| `✓` | File converted successfully |
| `✗` | Conversion failed — logged to error log |

### Error Log

Failed conversions are recorded in `conversion_errors.log` inside the output folder:

```
[2026-02-20 08:30:15] FAILED: /home/chaschel/websites/Browse AI/help.browse.ai/faq/broken.html
[2026-02-20 08:30:16] FAILED: /home/chaschel/websites/Browse AI/help.browse.ai/old/legacy.html
```

If there are no errors, the log file will be empty.

---

## Error Handling

| Scenario | Behavior |
|---|---|
| Input folder doesn't exist | Script exits with error message |
| No arguments provided | Script prints usage and exits |
| `html2markdown` fails on a file | File is skipped, logged, partial output removed |
| Output folder doesn't exist | Created automatically (including subdirectories) |
| Re-running the script | Error log is cleared; existing `.md` files are overwritten |

---

## Troubleshooting

### `html2markdown: command not found`

Ensure the binary is in your `PATH`:

```bash
export PATH=$PATH:/path/to/html2markdown/directory
```

Or specify the full path in the script by editing line 42:

```bash
if /full/path/to/html2markdown "$htmlfile" > "$mdfile" 2>/dev/null; then
```

### Script hangs on large folders

For very large numbers of files, the script processes them sequentially. This is intentional for simplicity and to avoid overloading the system. Allow it time to complete.

### Empty `.md` files

If `html2markdown` produces no output for certain files (but exits successfully), the `.md` file will be created but empty. Check the original `.html` file — it may contain only scripts/styles with no convertible text content.

---

## Script Reference

| Property | Value |
|---|---|
| **File** | `html2md.sh` |
| **Shell** | Bash (`#!/bin/bash`) |
| **Flags** | `set -euo pipefail` (strict error handling) |
| **External tool** | `html2markdown` (must be in `PATH`) |
| **File filter** | `*.html` files only |
| **Search** | Recursive (all subdirectories) |
| **Original files** | Preserved (read-only access) |

#!/bin/bash
# Convert all .html files in a folder (recursively) to .md using html2markdown
# Usage: ./html2md.sh <input_folder> [output_folder]
#   input_folder:  folder containing .html files
#   output_folder: where .md files go (default: <input_folder>_md)

set -euo pipefail

if [ $# -lt 1 ]; then
    echo "Usage: $0 <input_folder> [output_folder]"
    exit 1
fi

INPUT_DIR="$(realpath "$1")"
OUTPUT_DIR="${2:-${INPUT_DIR}_md}"
OUTPUT_DIR="$(realpath -m "$OUTPUT_DIR")"
LOG_FILE="${OUTPUT_DIR}/conversion_errors.log"

if [ ! -d "$INPUT_DIR" ]; then
    echo "Error: Input directory '$INPUT_DIR' does not exist."
    exit 1
fi

mkdir -p "$OUTPUT_DIR"
> "$LOG_FILE"  # clear/create log file

total=0
success=0
failed=0

find "$INPUT_DIR" -name "*.html" -type f | while read -r htmlfile; do
    total=$((total + 1))

    # Compute relative path and build output path
    relpath="${htmlfile#$INPUT_DIR/}"
    mdfile="${OUTPUT_DIR}/${relpath%.html}.md"

    # Create output subdirectory if needed
    mkdir -p "$(dirname "$mdfile")"

    # Convert and capture errors
    if html2markdown "$htmlfile" > "$mdfile" 2>/dev/null; then
        success=$((success + 1))
        echo "✓ $relpath"
    else
        failed=$((failed + 1))
        echo "✗ $relpath (logged)"
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] FAILED: $htmlfile" >> "$LOG_FILE"
        rm -f "$mdfile"  # remove empty/partial output
    fi
done

echo ""
echo "Done. Output: $OUTPUT_DIR"
echo "Errors logged to: $LOG_FILE"

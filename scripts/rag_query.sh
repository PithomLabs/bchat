#!/usr/bin/env bash
# rag_query.sh — Multi-mode RAG + source query tool for bchat
#
# Usage:
#   rag_query.sh search  <query> [tenant_id] [top_k]   # Semantic search via RAG API
#   rag_query.sh source  [tenant_id] [file_type]        # List source files from SQLite
#   rag_query.sh read    <file_id>                      # Dump source file content by ID
#   rag_query.sh grep    <pattern>                      # Exact match via ripgrep
#   rag_query.sh all     <query> [tenant_id]            # Combined: RAG → grep fallback
#   rag_query.sh help                                   # Show this help
#
# Auth:
#   RAG API mode requires a JWT token in /tmp/bchat_token.
#   Get it: log in to bchat → browser dev tools → Application → Cookies →
#   `memos.access-token` → copy value to /tmp/bchat_token.
#
#   Without a valid token, search falls through to grep only.

set -euo pipefail

# --- Config ---
BCHAT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
DB="${BCHAT_DIR}/build/data/memos_dev.db"
API_BASE="http://localhost:5230"
TOKEN_FILE="/tmp/bchat_token"
DEFAULT_TENANT_ID=1
DEFAULT_TOP_K=5

# --- Colors ---
CLR_RESET="\033[0m"
CLR_CYAN="\033[36m"
CLR_GREEN="\033[32m"
CLR_YELLOW="\033[33m"
CLR_RED="\033[31m"
CLR_BOLD="\033[1m"

info()  { echo -e "${CLR_CYAN}${CLR_BOLD}::${CLR_RESET} $*"; }
ok()    { echo -e "${CLR_GREEN}==>${CLR_RESET} $*"; }
warn()  { echo -e "${CLR_YELLOW}==>${CLR_RESET} $*" >&2; }
err()   { echo -e "${CLR_RED}ERROR:${CLR_RESET} $*" >&2; }

# --- Help ---
show_help() {
  sed -n '2,/^$/p' "$0" | sed 's/^# \?//'
  exit 0
}

# --- Check dependencies ---
check_deps() {
  local missing=()
  for cmd in curl sqlite3 rg; do
    if ! command -v "$cmd" &>/dev/null 2>&1; then
      missing+=("$cmd")
    fi
  done
  if [ ${#missing[@]} -gt 0 ]; then
    err "Missing dependencies: ${missing[*]}"
    err "Install: sudo apt install curl sqlite3 ripgrep"
    exit 1
  fi
}

# --- JWT token helpers ---
read_token() {
  if [ -f "$TOKEN_FILE" ]; then
    cat "$TOKEN_FILE" | tr -d ' \n'
  fi
}

token_valid() {
  local token
  token="$(read_token)"
  [ -z "$token" ] && return 1

  local payload
  payload="$(echo "$token" | cut -d. -f2 2>/dev/null)"
  [ -z "$payload" ] && return 1

  # Pad base64 for decoding
  local padded
  padded="${payload}$(printf '=%.0s' $(seq 1 $(( (4 - ${#payload} % 4) % 4 ))))" 2>/dev/null || true
  local decoded exp now
  decoded="$(echo "$padded" | base64 -d 2>/dev/null || echo "{}")"
  exp="$(echo "$decoded" | python3 -c "import sys,json; print(json.load(sys.stdin).get('exp',0))" 2>/dev/null || echo 0)"
  now="$(date +%s)"
  [ "$exp" -gt "$now" ] 2>/dev/null
}

# --- Mode: RAG search via API ---
cmd_search() {
  local query="${1:-}"
  local tenant_id="${2:-$DEFAULT_TENANT_ID}"
  local top_k="${3:-$DEFAULT_TOP_K}"

  if [ -z "$query" ]; then
    err "Usage: rag_query.sh search \"query\" [tenant_id] [top_k]"
    return 1
  fi

  if ! curl -sf "$API_BASE/api/v1/admin/rag/stats" >/dev/null 2>&1; then
    warn "bchat server not reachable at $API_BASE. Falling back to grep."
    echo ""
    cmd_grep "$query"
    return $?
  fi

  local token
  token="$(read_token)"

  if ! token_valid; then
    warn "No valid JWT token found in $TOKEN_FILE."
    warn "To enable RAG search: log into bchat at $API_BASE, then copy your"
    warn "memos.access-token cookie value to $TOKEN_FILE"
    echo ""
    cmd_grep "$query"
    return $?
  fi

  info "Searching RAG index for: ${query}"

  # Build JSON payload safely with python to avoid shell quoting issues
  local payload
  payload="$(python3 -c "
import json
print(json.dumps({
    'query': '$query',
    'tenantId': $tenant_id,
    'topK': $top_k,
    'audienceType': 'external'
}))
" 2>/dev/null)" || {
    warn "Failed to build request payload"
    cmd_grep "$query"
    return $?
  }

  local response
  response="$(curl -sf \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -d "$payload" \
    "$API_BASE/api/v1/admin/rag/search" 2>&1)" || {
    local exit_code=$?
    warn "RAG API call failed (exit $exit_code). Falling back to grep."
    echo ""
    cmd_grep "$query"
    return $?
  }

  echo ""
  echo -e "${CLR_BOLD}[MODE: search]${CLR_RESET}"
  echo "[QUERY: $query]"
  echo "[SOURCE: rag-api]"
  echo "[TENANT: $tenant_id] [K: $top_k]"

  echo "$response" | python3 -c "
import sys, json

CLR_BOLD = '\033[1m'
CLR_RESET = '\033[0m'

data = json.load(sys.stdin)
results = data.get('results', [])
mode = data.get('searchMode', 'vector')
latency = data.get('latencyMs', 0)

print(f'[ENGINE: {mode}] [LATENCY: {latency}ms]')
print(f'[TOTAL: {len(results)}]')
print('---')

if not results:
    print('No RAG results found.')
    sys.exit(0)

for i, r in enumerate(results):
    chunk = r.get('chunk', {})
    score = r.get('score', 0.0)
    vscore = r.get('vectorScore')
    bscore = r.get('bm25Score')
    title = chunk.get('title', 'Untitled')
    content = chunk.get('content', '')
    ctype = chunk.get('contentType', 'unknown')
    code = chunk.get('code', '')
    emergency = chunk.get('isEmergency', False)
    audience = chunk.get('audienceType', 'unknown')
    cid = chunk.get('id', '')

    if len(content) > 600:
        content = content[:600] + '...(truncated)'

    hybrid_info = ''
    if vscore is not None and bscore is not None:
        hybrid_info = f' | vector={vscore:.3f} bm25={bscore:.3f}'

    print(f'''
{CLR_BOLD}Chunk {i+1} (score: {score:.3f}{hybrid_info}){CLR_RESET}
  ID: {cid}
  Title: {title}
  Type: {ctype}  Audience: {audience}{\"  EMERGENCY\" if emergency else \"\"}
  Code: {code if code else \"(none)\"}
  Content:
{content}
''')
" 2>/dev/null || warn "Failed to parse RAG response (raw below): $response"
}

# --- Mode: Source file listing from SQLite ---
cmd_source() {
  local tenant_id="${1:-}"
  local file_type="${2:-}"

  if [ ! -f "$DB" ]; then
    err "SQLite database not found at $DB"
    err "Start bchat first or check BCHAT_DIR."
    return 1
  fi

  local where="" tsql
  if [ -n "$tenant_id" ]; then
    where="$where AND sf.tenant_id = $tenant_id"
  fi
  if [ -n "$file_type" ]; then
    where="$where AND sf.file_type = '${file_type}'"
  fi

  echo ""
  echo -e "${CLR_BOLD}[MODE: source]${CLR_RESET}"
  echo "[SOURCE: sqlite]"
  echo "[DB: $DB]"
  echo "---"

  # Check table exists
  tsql="$(sqlite3 "$DB" "SELECT name FROM sqlite_master WHERE type='table' AND name='agent_source_files';" 2>/dev/null)"
  if [ -z "$tsql" ]; then
    err "Table 'agent_source_files' not found in $DB"
    err "Has the RAG pipeline been set up? Check: sqlite3 $DB '.tables' | grep agent"
    return 1
  fi

  sqlite3 -header -column "$DB" "
    SELECT
      sf.id,
      t.slug,
      sf.tenant_id,
      sf.audience_type,
      sf.file_type,
      sf.version,
      length(sf.content) AS bytes,
      sf.imported_at
    FROM agent_source_files sf
    JOIN agent_tenants t ON t.id = sf.tenant_id
    WHERE 1=1 $where
    ORDER BY sf.tenant_id, sf.file_type, sf.version DESC
    LIMIT 50;
  " 2>/dev/null || err "Query failed"

  echo ""
  info "To read a file: rag_query.sh read <file_id>"
}

# --- Mode: Read source file content by ID ---
cmd_read() {
  local file_id="${1:-}"
  if [ -z "$file_id" ]; then
    err "Usage: rag_query.sh read <file_id>"
    return 1
  fi

  if [ ! -f "$DB" ]; then
    err "SQLite database not found at $DB"
    return 1
  fi

  local row
  row="$(sqlite3 "$DB" "
    SELECT
      'ID: ' || sf.id,
      'Tenant: ' || t.slug || ' (' || sf.tenant_id || ')',
      'Type: ' || sf.file_type,
      'Audience: ' || sf.audience_type,
      'Version: ' || sf.version,
      'Date: ' || sf.imported_at
    FROM agent_source_files sf
    JOIN agent_tenants t ON t.id = sf.tenant_id
    WHERE sf.id = $file_id;
  " 2>/dev/null)"

  if [ -z "$row" ]; then
    err "File $file_id not found"
    return 1
  fi

  echo ""
  echo -e "${CLR_BOLD}[MODE: read]${CLR_RESET}"
  echo "[SOURCE: sqlite]"
  echo "[FILE ID: $file_id]"
  echo "---"
  echo "$row"
  echo ""
  echo -e "${CLR_BOLD}--- CONTENT ---${CLR_RESET}"
  sqlite3 "$DB" "SELECT sf.content FROM agent_source_files sf WHERE sf.id = $file_id;" 2>/dev/null || err "Read failed"
}

# --- Mode: Grep through project files ---
cmd_grep() {
  local pattern="${1:-}"
  if [ -z "$pattern" ]; then
    err "Usage: rag_query.sh grep \"pattern\""
    return 1
  fi

  echo ""
  echo -e "${CLR_BOLD}[MODE: grep]${CLR_RESET}"
  echo "[PATTERN: $pattern]"
  echo "[SOURCE: ripgrep]"
  echo "[PATH: $BCHAT_DIR]"
  echo "---"

  rg -n --context 2 "$pattern" \
    -g '*.go' \
    -g '*.md' \
    -g '*.ts' \
    -g '*.tsx' \
    -g '*.sql' \
    -g '*.yaml' \
    -g '*.yml' \
    -g '*.json' \
    -g '!node_modules' \
    -g '!dist' \
    -g '!*.pb.go' \
    "$BCHAT_DIR" 2>/dev/null || {
    warn "No matches found for: $pattern"
    return 1
  }
}

# --- Mode: Combined search (RAG → grep) ---
cmd_combined() {
  local query="${1:-}"
  local tenant_id="${2:-$DEFAULT_TENANT_ID}"

  if [ -z "$query" ]; then
    err "Usage: rag_query.sh all \"query\" [tenant_id]"
    return 1
  fi

  info "Combined search for: ${query}"
  echo ""

  cmd_search "$query" "$tenant_id" 5 && return 0
}

# --- Main ---
main() {
  check_deps

  local mode="${1:-help}"
  shift 2>/dev/null || true

  case "$mode" in
    search)
      cmd_search "$@"
      ;;
    source|list)
      cmd_source "$@"
      ;;
    read)
      cmd_read "$@"
      ;;
    grep)
      cmd_grep "$@"
      ;;
    all|combined)
      cmd_combined "$@"
      ;;
    help|--help|-h)
      show_help
      ;;
    *)
      err "Unknown mode: $mode"
      echo ""
      show_help
      ;;
  esac
}

main "$@"

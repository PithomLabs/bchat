#!/usr/bin/env bash
cd /home/chaschel/Documents/go/bchat
export OPENROUTER_API_KEY="${OPENROUTER_API_KEY:-}"
export RAG_PIPELINE_ENABLED=true
DATABASE_URL="${DATABASE_URL:-postgresql://bchat:bchat@localhost:5432/bchat}"
exec ./build/memos --mode dev --driver postgres --dsn "$DATABASE_URL"

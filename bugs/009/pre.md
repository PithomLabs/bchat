deep dive into the root cause, write plan to fix the underlying problem (not band-aid fix)

chaschel@linux:~/Documents/go/bchat$ task run:rag
task: [validate:migrations] ./scripts/validate-migrations.sh
task: Task "setup:lancedb" is up to date
Validating LATEST.sql against migrations...

✓ LATEST.sql is in sync with all migrations
task: [build:backend:rag] mkdir -p build
task: [build:backend:rag] go build -tags rag -o build/memos ./bin/memos/main.go
task: [run:rag] if [ -f .env ]; then
  echo "Loading environment from .env file..."
  set -a && source .env && set +a
fi
FORCE_REINDEX_ON_STARTUP=false RAG_PIPELINE_ENABLED=true EMBEDDING_MODEL=openai/text-embedding-3-small EMBEDDING_BATCH_SIZE=1 LANCEDB_STORAGE_PROVIDER=local ./build/memos --mode dev --data /home/chaschel/Documents/go/bchat/build/data

Loading environment from .env file...
2026/05/31 00:28:56 INFO Column already exists, skipping table=tickets column=type
2026/05/31 00:28:56 INFO Column already exists, skipping table=tickets column=tags
2026/05/31 00:28:56 INFO start migration currentSchemaVersion=0.25.28 targetSchemaVersion=0.25.29
2026/05/31 00:28:56 ERROR failed to migrate error="constraint failed: UNIQUE constraint failed: tickets.creator_id, tickets.description (2067)\nfailed to execute statement\ngithub.com/usememos/memos/store.(*Store).execute\n\t/home/chaschel/Documents/go/bchat/store/migrator.go:261\ngithub.com/usememos/memos/store.(*Store).Migrate\n\t/home/chaschel/Documents/go/bchat/store/migrator.go:102\nmain.init.func1\n\t/home/chaschel/Documents/go/bchat/bin/memos/main.go:65\ngithub.com/spf13/cobra.(*Command).execute\n\t/home/chaschel/go/pkg/mod/github.com/spf13/cobra@v1.9.1/command.go:1019\ngithub.com/spf13/cobra.(*Command).ExecuteC\n\t/home/chaschel/go/pkg/mod/github.com/spf13/cobra@v1.9.1/command.go:1148\ngithub.com/spf13/cobra.(*Command).Execute\n\t/home/chaschel/go/pkg/mod/github.com/spf13/cobra@v1.9.1/command.go:1071\nmain.main\n\t/home/chaschel/Documents/go/bchat/bin/memos/main.go:208\nruntime.main\n\t/usr/local/go/src/runtime/proc.go:285\nruntime.goexit\n\t/usr/local/go/src/runtime/asm_amd64.s:1693\nmigrate error: -- Prevent duplicate tickets for same memo link (auto-creation + explicit creation)\nCREATE UNIQUE INDEX IF NOT EXISTS idx_tickets_creator_description_memo \nON tickets(creator_id, description) \nWHERE description LIKE '/m/%';\n\ngithub.com/usememos/memos/store.(*Store).Migrate\n\t/home/chaschel/Documents/go/bchat/store/migrator.go:103\nmain.init.func1\n\t/home/chaschel/Documents/go/bchat/bin/memos/main.go:65\ngithub.com/spf13/cobra.(*Command).execute\n\t/home/chaschel/go/pkg/mod/github.com/spf13/cobra@v1.9.1/command.go:1019\ngithub.com/spf13/cobra.(*Command).ExecuteC\n\t/home/chaschel/go/pkg/mod/github.com/spf13/cobra@v1.9.1/command.go:1148\ngithub.com/spf13/cobra.(*Command).Execute\n\t/home/chaschel/go/pkg/mod/github.com/spf13/cobra@v1.9.1/command.go:1071\nmain.main\n\t/home/chaschel/Documents/go/bchat/bin/memos/main.go:208\nruntime.main\n\t/usr/local/go/src/runtime/proc.go:285\nruntime.goexit\n\t/usr/local/go/src/runtime/asm_amd64.s:1693"

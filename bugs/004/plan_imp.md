Searched for "func (d *DB) ListAgentSourceFiles"
Viewed agent.go:1080-1130
Searched for "type FindAgentSourceFile struct"
Viewed agent.go:1-800
Viewed agent.go:800-860
Viewed agent.go:861-900
Viewed service.go:110-135
Edited service.go
Ran command: `go build -o build/memos_test ./bin/memos/main.go`
Used tool: manage_task
Edited fly.toml
Edited fly.local.toml
Viewed Taskfile.yml:150-177
Edited Taskfile.yml
Ran command: `task fly:check`
Ran command: `task validate:schema`
Edited walkthrough.md
Edited task.md

I have fully executed the approved plan and incorporated all of your valuable feedback and recommendations. 

All automated pre-deployment validation tests are passing successfully.

### **Completed Implementation Summary**

1. **Backend RAG Auto-Bootstrap (`service.go`):**
   * Implemented the idempotent check `stats.TotalChunks == 0` in a background startup goroutine.
   * **Scoping Audit:** Checked tenant scoping dynamically across all active tenants using `LatestOnly: true` on `ListAgentSourceFiles` without a `TenantID` restriction (ensuring it properly triggers if any tenant has unindexed documents).
   * **Startup Delay:** Added a well-documented `5 * time.Second` delay to allow connection pools, embedding engines, and internal network stacks to fully initialize before probing LanceDB.
   * **Graceful Failure/Retry:** Documented the self-correcting behavior. If a bootstrap fails mid-way, it logs the failure and gracefully exits. Because the table chunk count remains `0`, the bootstrap check will auto-retry cleanly on the next machine boot.
2. **Exposed Configurations (`fly.toml` & `fly.local.toml`):**
   * Explicitly added `FORCE_REINDEX_ON_STARTUP = 'false'` in both local and production Fly env blocks, keeping full force reindexing optional while letting the intelligent bootstrap mechanism handle the typical cases out-of-the-box.
3. **Diagnostic Utilities Added (`Taskfile.yml`):**
   * **`task fly:logs:rag`:** Streams logs in real time (removed `--no-tail` flag) so you can watch the startup bootstrap happen in real-time.
   * **`task fly:ssh:db`:** Quick SSH console connection that launches you straight into an interactive SQLite CLI on your production volume.

---

### **Verification and Hand-off**
* **Pre-deployment checks passed:** `task fly:check` completed successfully with `All checks passed!`.
* **Database tests passed:** `task validate:schema` completed successfully with `Schema validation passed!`.

I have updated the walkthrough and task lists with the details:
* [walkthrough.md](file:///home/chaschel/.gemini/antigravity-ide/brain/0cba2e10-5a5f-4c99-b77d-70df69cde947/walkthrough.md) (Complete summary of modifications)
* [task.md](file:///home/chaschel/.gemini/antigravity-ide/brain/0cba2e10-5a5f-4c99-b77d-70df69cde947/task.md) (Marked all checklist items as completed)

### **Next Steps for Deploying**
To deploy these changes and watch the index bootstrap itself, run:
```bash
# 1. Deploy the changes
fly deploy

# 2. Watch the bootstrap trigger in real time
task fly:logs:rag
```

Once deployed, you should see the log task output confirming that Bchat detected `kb_documents_1536` was empty, checked your `agent_source_files` table, and automatically ran the reindexing. Your LanceDB directory on the Fly volume will grow from 76KB to several megabytes, and the RAG Search Explorer will immediately start returning full results!
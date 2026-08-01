2026/08/01 05:24:04 INFO OK method=/memos.api.v1.MemoService/CreateMemo
2026/08/01 05:24:06 INFO CreateTicket handler context_keys=[]
2026/08/01 05:24:06 INFO CreateTicket userID userID=1 ok=true
2026/08/01 05:24:06 INFO CreateTicket request title="Per-Ticket RAG Indexing Prototype" status=OPEN priority=MEDIUM
2026/08/01 05:24:06 INFO CreateTicket validated
2026/08/01 05:24:06 INFO CreateTicket success id=174
2026/08/01 05:24:06 INFO Creating per-tenant local LanceDB connection tenantID=19 path=build/data/lancedb/19
2026/08/01 05:24:06 INFO LanceDB vector database initialized uri=build/data/lancedb/19 provider=local tableName=kb_documents_1536 dimension=1536
2026/08/01 05:24:06 INFO Starting batched insert totalChunks=1 batchSize=200 table=kb_documents_1536
2026/08/01 05:24:06 INFO Processing batch batch=1 totalBatches=1 chunksInBatch=1 progress=1/1
2026/08/01 05:24:07 WARN Failed to create vector index after insert error="failed to create vector index: failed to create index: Failed to create index: lance error: LanceError(Index): Not enough rows to train PQ. Requires 256 rows but only 5 available, /home/runner/.cargo/registry/src/index.crates.io-1949cf8c6b5b557f/lance-index-0.37.0/src/vector/pq/builder.rs:180:27"
2026/08/01 05:24:07 INFO Completed batched insert totalChunks=1
2026/08/01 05:24:09 INFO inferred resolution for new ticket ticket_id=174 similar_tickets=1 bug_history=0 total=1
2026/08/01 05:24:09 ERROR failed to create system resolution comment ticket_id=174 error="parent memo not found: %!w(<nil>)"

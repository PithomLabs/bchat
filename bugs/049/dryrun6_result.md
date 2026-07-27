chaschel@linux:~/Documents/go/bchat$ cd /home/chaschel/Documents/go/bchat && BENCHMARK_LONGMEMEVAL=true OPENROUTER_API_KEY=sk-or-v1-... BENCHMARK_FRESH=true go test ./server/router/api/v1/agent/ -run TestBenchmarkKnowledgeUpdate -v -count=1 -timeout=90m
=== RUN   TestBenchmarkKnowledgeUpdate
    benchmark_longmemeval_test.go:827: Cleared existing JSONL files (BENCHMARK_FRESH=true)
    benchmark_longmemeval_test.go:827: Loaded 500 questions (178 testable), 14065 cache entries
    benchmark_longmemeval_test.go:827: knowledge_update: 72 questions (non-abs)
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:50:53 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:50:53 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:50:53 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:50:53 INFO Verification layer initialized
2026/07/28 05:50:53 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:50:53 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:50:55 ERROR EstimateTokens using len/4 fallback — globalTokenizer not initialized contentLength=39
2026/07/28 05:50:55 INFO Observer completed successfully session_id=ku-6a1eabeb resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=9 reflector_triggered=false hybrid_indexed=false duration_ms=2275
    benchmark_longmemeval_test.go:827: [1/72] 6a1eabeb — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:50:57 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:50:57 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:50:57 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:50:57 INFO Verification layer initialized
2026/07/28 05:50:57 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:50:57 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:51:45 INFO Observer completed successfully session_id=ku-6aeb4375 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=285 reflector_triggered=false hybrid_indexed=false duration_ms=48063
    benchmark_longmemeval_test.go:827: [2/72] 6aeb4375 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:51:52 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:51:52 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:51:52 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:51:52 INFO Verification layer initialized
2026/07/28 05:51:52 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:51:52 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:52:01 INFO Observer completed successfully session_id=ku-830ce83f resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=196 reflector_triggered=false hybrid_indexed=false duration_ms=8421
    benchmark_longmemeval_test.go:827: [3/72] 830ce83f — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:52:06 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:52:06 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:52:06 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:52:06 INFO Verification layer initialized
2026/07/28 05:52:06 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:52:06 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:52:17 INFO Observer completed successfully session_id=ku-852ce960 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=250 reflector_triggered=false hybrid_indexed=false duration_ms=11020
    benchmark_longmemeval_test.go:827: [4/72] 852ce960 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:52:29 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:52:29 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:52:29 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:52:29 INFO Verification layer initialized
2026/07/28 05:52:29 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:52:29 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:52:42 INFO Observer completed successfully session_id=ku-945e3d21 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=273 reflector_triggered=false hybrid_indexed=false duration_ms=12414
    benchmark_longmemeval_test.go:827: [5/72] 945e3d21 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:52:48 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:52:48 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:52:48 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:52:48 INFO Verification layer initialized
2026/07/28 05:52:48 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:52:48 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:53:15 INFO Observer completed successfully session_id=ku-d7c942c3 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=43 reflector_triggered=false hybrid_indexed=false duration_ms=26770
    benchmark_longmemeval_test.go:827: [6/72] d7c942c3 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:53:17 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:53:17 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:53:17 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:53:17 INFO Verification layer initialized
2026/07/28 05:53:17 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:53:17 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:53:29 INFO Observer completed successfully session_id=ku-71315a70 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=347 reflector_triggered=false hybrid_indexed=false duration_ms=11944
    benchmark_longmemeval_test.go:827: [7/72] 71315a70 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:53:32 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:53:32 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:53:32 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:53:32 INFO Verification layer initialized
2026/07/28 05:53:32 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:53:32 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:54:27 INFO Observer completed successfully session_id=ku-89941a93 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=911 reflector_triggered=false hybrid_indexed=false duration_ms=54510
    benchmark_longmemeval_test.go:827: [8/72] 89941a93 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:54:41 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:54:41 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:54:41 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:54:41 INFO Verification layer initialized
2026/07/28 05:54:41 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:54:41 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:55:14 INFO Observer completed successfully session_id=ku-ce6d2d27 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=281 reflector_triggered=false hybrid_indexed=false duration_ms=33194
    benchmark_longmemeval_test.go:827: [9/72] ce6d2d27 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:55:17 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:55:17 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:55:17 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:55:18 INFO Verification layer initialized
2026/07/28 05:55:18 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:55:18 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:55:28 INFO Observer completed successfully session_id=ku-9ea5eabc resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=224 reflector_triggered=false hybrid_indexed=false duration_ms=10037
    benchmark_longmemeval_test.go:827: [10/72] 9ea5eabc — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:55:30 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:55:30 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:55:30 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:55:30 INFO Verification layer initialized
2026/07/28 05:55:30 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:55:30 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:55:43 INFO Observer completed successfully session_id=ku-07741c44 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=209 reflector_triggered=false hybrid_indexed=false duration_ms=12394
    benchmark_longmemeval_test.go:827: [11/72] 07741c44 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:55:46 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:55:46 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:55:46 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:55:46 INFO Verification layer initialized
2026/07/28 05:55:46 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:55:46 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:56:40 INFO Observer completed successfully session_id=ku-a1eacc2a resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=88 reflector_triggered=false hybrid_indexed=false duration_ms=54089
    benchmark_longmemeval_test.go:827: [12/72] a1eacc2a — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:56:52 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:56:52 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:56:52 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:56:52 INFO Verification layer initialized
2026/07/28 05:56:52 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:56:52 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:56:56 INFO Observer completed successfully session_id=ku-184da446 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=4 reflector_triggered=false hybrid_indexed=false duration_ms=3825
    benchmark_longmemeval_test.go:827: [13/72] 184da446 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:56:58 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:56:58 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:56:58 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:56:58 INFO Verification layer initialized
2026/07/28 05:56:58 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:56:58 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:57:07 INFO Observer completed successfully session_id=ku-031748ae resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=181 reflector_triggered=false hybrid_indexed=false duration_ms=8736
    benchmark_longmemeval_test.go:827: [14/72] 031748ae — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:57:20 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:57:20 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:57:20 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:57:20 INFO Verification layer initialized
2026/07/28 05:57:20 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:57:20 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:57:28 INFO Observer completed successfully session_id=ku-4d6b87c8 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=38 reflector_triggered=false hybrid_indexed=false duration_ms=8548
    benchmark_longmemeval_test.go:827: [15/72] 4d6b87c8 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:57:30 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:57:30 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:57:30 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:57:30 INFO Verification layer initialized
2026/07/28 05:57:30 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:57:30 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:58:06 INFO Observer completed successfully session_id=ku-0f05491a resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=132 reflector_triggered=false hybrid_indexed=false duration_ms=36378
    benchmark_longmemeval_test.go:827: [16/72] 0f05491a — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:58:08 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:58:08 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:58:08 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:58:08 INFO Verification layer initialized
2026/07/28 05:58:08 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:58:08 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:58:21 INFO Observer completed successfully session_id=ku-08e075c7 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=295 reflector_triggered=false hybrid_indexed=false duration_ms=12902
    benchmark_longmemeval_test.go:827: [17/72] 08e075c7 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:58:23 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:58:23 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:58:23 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:58:23 INFO Verification layer initialized
2026/07/28 05:58:23 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:58:23 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:58:27 INFO Observer completed successfully session_id=ku-f9e8c073 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=557 reflector_triggered=false hybrid_indexed=false duration_ms=3545
    benchmark_longmemeval_test.go:827: [18/72] f9e8c073 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:58:29 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:58:29 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:58:29 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:58:29 INFO Verification layer initialized
2026/07/28 05:58:29 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:58:29 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:58:31 INFO Observer completed successfully session_id=ku-41698283 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=9 reflector_triggered=false hybrid_indexed=false duration_ms=2546
    benchmark_longmemeval_test.go:827: [19/72] 41698283 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:58:35 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:58:35 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:58:35 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:58:35 INFO Verification layer initialized
2026/07/28 05:58:35 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:58:35 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:58:50 INFO Observer completed successfully session_id=ku-2698e78f resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=343 reflector_triggered=false hybrid_indexed=false duration_ms=14190
    benchmark_longmemeval_test.go:827: [20/72] 2698e78f — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:59:01 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:59:01 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:59:01 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:59:01 INFO Verification layer initialized
2026/07/28 05:59:01 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:59:01 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:59:06 INFO Observer completed successfully session_id=ku-b6019101 resource_id=user_999 scope=resource new_messages=22 skipped_trivial=0 total_tokens=403 reflector_triggered=false hybrid_indexed=false duration_ms=5080
    benchmark_longmemeval_test.go:827: [21/72] b6019101 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:59:08 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:59:08 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:59:08 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:59:08 INFO Verification layer initialized
2026/07/28 05:59:08 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:59:08 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:59:15 INFO Observer completed successfully session_id=ku-45dc21b6 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=306 reflector_triggered=false hybrid_indexed=false duration_ms=6909
    benchmark_longmemeval_test.go:827: [22/72] 45dc21b6 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:59:16 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:59:16 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:59:16 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:59:16 INFO Verification layer initialized
2026/07/28 05:59:16 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:59:16 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:59:24 INFO Observer completed successfully session_id=ku-5a4f22c0 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=205 reflector_triggered=false hybrid_indexed=false duration_ms=7939
    benchmark_longmemeval_test.go:827: [23/72] 5a4f22c0 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:59:26 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:59:26 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:59:26 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:59:27 INFO Verification layer initialized
2026/07/28 05:59:27 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:59:27 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:00:19 INFO Observer completed successfully session_id=ku-6071bd76 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=502 reflector_triggered=false hybrid_indexed=false duration_ms=52085
    benchmark_longmemeval_test.go:827: [24/72] 6071bd76 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:00:22 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:00:22 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:00:22 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:00:22 INFO Verification layer initialized
2026/07/28 06:00:22 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:00:22 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:00:33 INFO Observer completed successfully session_id=ku-e493bb7c resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=287 reflector_triggered=false hybrid_indexed=false duration_ms=10357
    benchmark_longmemeval_test.go:827: [25/72] e493bb7c — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:00:37 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:00:37 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:00:37 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:00:37 INFO Verification layer initialized
2026/07/28 06:00:37 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:00:37 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:01:07 INFO Observer completed successfully session_id=ku-618f13b2 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=151 reflector_triggered=false hybrid_indexed=false duration_ms=30183
    benchmark_longmemeval_test.go:827: [26/72] 618f13b2 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:01:09 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:01:09 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:01:09 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:01:09 INFO Verification layer initialized
2026/07/28 06:01:09 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:01:09 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:01:47 INFO Observer completed successfully session_id=ku-72e3ee87 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=641 reflector_triggered=false hybrid_indexed=false duration_ms=37976
    benchmark_longmemeval_test.go:827: [27/72] 72e3ee87 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:01:48 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:01:48 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:01:48 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:01:48 INFO Verification layer initialized
2026/07/28 06:01:48 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:01:48 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:02:07 INFO Observer completed successfully session_id=ku-c4ea545c resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=250 reflector_triggered=false hybrid_indexed=false duration_ms=18268
    benchmark_longmemeval_test.go:827: [28/72] c4ea545c — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:02:09 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:02:09 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:02:09 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:02:09 INFO Verification layer initialized
2026/07/28 06:02:09 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:02:09 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:02:11 INFO Observer completed successfully session_id=ku-01493427 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=9 reflector_triggered=false hybrid_indexed=false duration_ms=2275
    benchmark_longmemeval_test.go:827: [29/72] 01493427 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:02:17 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:02:17 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:02:17 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:02:17 INFO Verification layer initialized
2026/07/28 06:02:17 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:02:17 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:02:32 INFO Observer completed successfully session_id=ku-6a27ffc2 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=306 reflector_triggered=false hybrid_indexed=false duration_ms=14723
    benchmark_longmemeval_test.go:827: [30/72] 6a27ffc2 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:02:35 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:02:35 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:02:35 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:02:35 INFO Verification layer initialized
2026/07/28 06:02:35 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:02:35 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:02:58 INFO Observer completed successfully session_id=ku-2133c1b5 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=227 reflector_triggered=false hybrid_indexed=false duration_ms=22754
    benchmark_longmemeval_test.go:827: [31/72] 2133c1b5 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:03:00 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:03:00 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:03:00 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:03:00 INFO Verification layer initialized
2026/07/28 06:03:00 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:03:00 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:03:36 INFO Observer completed successfully session_id=ku-18bc8abd resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=372 reflector_triggered=false hybrid_indexed=false duration_ms=36383
    benchmark_longmemeval_test.go:827: [32/72] 18bc8abd — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:03:40 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:03:40 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:03:40 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:03:40 INFO Verification layer initialized
2026/07/28 06:03:40 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:03:40 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:04:05 INFO Observer completed successfully session_id=ku-db467c8c resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=143 reflector_triggered=false hybrid_indexed=false duration_ms=24946
    benchmark_longmemeval_test.go:827: [33/72] db467c8c — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:04:08 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:04:08 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:04:08 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:04:08 INFO Verification layer initialized
2026/07/28 06:04:08 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:04:08 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:04:32 INFO Observer completed successfully session_id=ku-7a87bd0c resource_id=user_999 scope=resource new_messages=22 skipped_trivial=0 total_tokens=125 reflector_triggered=false hybrid_indexed=false duration_ms=23823
    benchmark_longmemeval_test.go:827: [34/72] 7a87bd0c — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:04:42 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:04:42 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:04:42 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:04:42 INFO Verification layer initialized
2026/07/28 06:04:42 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:04:42 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:05:27 INFO Observer completed successfully session_id=ku-e61a7584 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=817 reflector_triggered=false hybrid_indexed=false duration_ms=44820
    benchmark_longmemeval_test.go:827: [35/72] e61a7584 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:05:29 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:05:29 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:05:29 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:05:29 INFO Verification layer initialized
2026/07/28 06:05:29 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:05:29 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:05:58 INFO Observer completed successfully session_id=ku-1cea1afa resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=161 reflector_triggered=false hybrid_indexed=false duration_ms=29433
    benchmark_longmemeval_test.go:827: [36/72] 1cea1afa — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:06:00 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:06:00 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:06:00 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:06:00 INFO Verification layer initialized
2026/07/28 06:06:00 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:06:00 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:06:23 INFO Observer completed successfully session_id=ku-ed4ddc30 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=315 reflector_triggered=false hybrid_indexed=false duration_ms=22856
    benchmark_longmemeval_test.go:827: [37/72] ed4ddc30 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:06:24 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:06:24 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:06:24 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:06:24 INFO Verification layer initialized
2026/07/28 06:06:24 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:06:24 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:06:50 INFO Observer completed successfully session_id=ku-8fb83627 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=663 reflector_triggered=false hybrid_indexed=false duration_ms=25134
    benchmark_longmemeval_test.go:827: [38/72] 8fb83627 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:07:00 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:07:00 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:07:00 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:07:00 INFO Verification layer initialized
2026/07/28 06:07:00 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:07:00 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:09:47 INFO Observer completed successfully session_id=ku-b01defab resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=343 reflector_triggered=false hybrid_indexed=false duration_ms=166516
    benchmark_longmemeval_test.go:827: [39/72] b01defab — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:09:49 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:09:49 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:09:49 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:09:49 INFO Verification layer initialized
2026/07/28 06:09:49 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:09:49 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:10:26 INFO Observer completed successfully session_id=ku-22d2cb42 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=237 reflector_triggered=false hybrid_indexed=false duration_ms=37281
    benchmark_longmemeval_test.go:827: [40/72] 22d2cb42 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:10:29 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:10:29 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:10:29 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:10:29 INFO Verification layer initialized
2026/07/28 06:10:29 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:10:29 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:11:14 INFO Observer completed successfully session_id=ku-0e4e4c46 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=177 reflector_triggered=false hybrid_indexed=false duration_ms=45308
    benchmark_longmemeval_test.go:827: [41/72] 0e4e4c46 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:11:18 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:11:18 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:11:18 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:11:18 INFO Verification layer initialized
2026/07/28 06:11:18 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:11:18 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:12:29 INFO Observer completed successfully session_id=ku-4b24c848 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=327 reflector_triggered=false hybrid_indexed=false duration_ms=71243
    benchmark_longmemeval_test.go:827: [42/72] 4b24c848 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:12:35 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:12:35 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:12:35 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:12:35 INFO Verification layer initialized
2026/07/28 06:12:35 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:12:35 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:13:18 INFO Observer completed successfully session_id=ku-7e974930 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=248 reflector_triggered=false hybrid_indexed=false duration_ms=42352
    benchmark_longmemeval_test.go:827: [43/72] 7e974930 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:13:20 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:13:20 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:13:20 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:13:20 INFO Verification layer initialized
2026/07/28 06:13:20 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:13:20 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:13:37 INFO Observer completed successfully session_id=ku-603deb26 resource_id=user_999 scope=resource new_messages=22 skipped_trivial=0 total_tokens=247 reflector_triggered=false hybrid_indexed=false duration_ms=16946
    benchmark_longmemeval_test.go:827: [44/72] 603deb26 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:13:39 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:13:39 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:13:39 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:13:39 INFO Verification layer initialized
2026/07/28 06:13:39 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:13:39 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:13:45 INFO Observer completed successfully session_id=ku-59524333 resource_id=user_999 scope=resource new_messages=22 skipped_trivial=0 total_tokens=57 reflector_triggered=false hybrid_indexed=false duration_ms=5578
    benchmark_longmemeval_test.go:827: [45/72] 59524333 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:13:50 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:13:50 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:13:50 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:13:50 INFO Verification layer initialized
2026/07/28 06:13:50 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:13:50 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:14:05 INFO Observer completed successfully session_id=ku-5831f84d resource_id=user_999 scope=resource new_messages=22 skipped_trivial=0 total_tokens=399 reflector_triggered=false hybrid_indexed=false duration_ms=15057
    benchmark_longmemeval_test.go:827: [46/72] 5831f84d — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:14:08 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:14:08 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:14:08 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:14:08 INFO Verification layer initialized
2026/07/28 06:14:08 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:14:08 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:14:54 INFO Observer completed successfully session_id=ku-eace081b resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=162 reflector_triggered=false hybrid_indexed=false duration_ms=46523
    benchmark_longmemeval_test.go:827: [47/72] eace081b — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:14:56 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:14:56 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:14:56 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:14:56 INFO Verification layer initialized
2026/07/28 06:14:56 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:14:56 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:15:25 INFO Observer completed successfully session_id=ku-affe2881 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=134 reflector_triggered=false hybrid_indexed=false duration_ms=28748
    benchmark_longmemeval_test.go:827: [48/72] affe2881 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:15:29 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:15:29 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:15:29 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:15:29 INFO Verification layer initialized
2026/07/28 06:15:29 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:15:29 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:15:57 INFO Observer completed successfully session_id=ku-50635ada resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=677 reflector_triggered=false hybrid_indexed=false duration_ms=27509
    benchmark_longmemeval_test.go:827: [49/72] 50635ada — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:15:59 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:15:59 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:15:59 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:15:59 INFO Verification layer initialized
2026/07/28 06:15:59 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:15:59 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:16:20 INFO Observer completed successfully session_id=ku-e66b632c resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=9 reflector_triggered=false hybrid_indexed=false duration_ms=21674
    benchmark_longmemeval_test.go:827: [50/72] e66b632c — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:16:22 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:16:22 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:16:22 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:16:22 INFO Verification layer initialized
2026/07/28 06:16:22 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:16:22 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:16:30 INFO Observer completed successfully session_id=ku-0ddfec37 resource_id=user_999 scope=resource new_messages=22 skipped_trivial=0 total_tokens=798 reflector_triggered=false hybrid_indexed=false duration_ms=7570
    benchmark_longmemeval_test.go:827: [51/72] 0ddfec37 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:16:35 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:16:35 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:16:35 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:16:35 INFO Verification layer initialized
2026/07/28 06:16:35 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:16:35 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:16:47 INFO Observer completed successfully session_id=ku-f685340e resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=238 reflector_triggered=false hybrid_indexed=false duration_ms=11365
    benchmark_longmemeval_test.go:827: [52/72] f685340e — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:16:50 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:16:50 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:16:50 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:16:50 INFO Verification layer initialized
2026/07/28 06:16:50 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:16:50 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:17:12 INFO Observer completed successfully session_id=ku-cc5ded98 resource_id=user_999 scope=resource new_messages=22 skipped_trivial=0 total_tokens=189 reflector_triggered=false hybrid_indexed=false duration_ms=21236
    benchmark_longmemeval_test.go:827: [53/72] cc5ded98 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:17:14 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:17:14 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:17:14 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:17:14 INFO Verification layer initialized
2026/07/28 06:17:14 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:17:14 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:17:38 INFO Observer completed successfully session_id=ku-dfde3500 resource_id=user_999 scope=resource new_messages=22 skipped_trivial=0 total_tokens=393 reflector_triggered=false hybrid_indexed=false duration_ms=23433
    benchmark_longmemeval_test.go:827: [54/72] dfde3500 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:17:43 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:17:43 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:17:43 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:17:43 INFO Verification layer initialized
2026/07/28 06:17:43 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:17:43 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:18:13 INFO Observer completed successfully session_id=ku-69fee5aa resource_id=user_999 scope=resource new_messages=22 skipped_trivial=0 total_tokens=764 reflector_triggered=false hybrid_indexed=false duration_ms=30048
    benchmark_longmemeval_test.go:827: [55/72] 69fee5aa — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:18:16 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:18:16 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:18:16 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:18:16 INFO Verification layer initialized
2026/07/28 06:18:16 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:18:16 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:19:27 INFO Observer completed successfully session_id=ku-7401057b resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=594 reflector_triggered=false hybrid_indexed=false duration_ms=70220
    benchmark_longmemeval_test.go:827: [56/72] 7401057b — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:19:40 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:19:40 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:19:40 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:19:40 INFO Verification layer initialized
2026/07/28 06:19:40 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:19:40 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:21:14 INFO Observer completed successfully session_id=ku-cf22b7bf resource_id=user_999 scope=resource new_messages=22 skipped_trivial=0 total_tokens=720 reflector_triggered=false hybrid_indexed=false duration_ms=93913
    benchmark_longmemeval_test.go:827: [57/72] cf22b7bf — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:21:16 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:21:16 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:21:16 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:21:16 INFO Verification layer initialized
2026/07/28 06:21:16 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:21:16 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:21:25 INFO Observer completed successfully session_id=ku-a2f3aa27 resource_id=user_999 scope=resource new_messages=20 skipped_trivial=0 total_tokens=199 reflector_triggered=false hybrid_indexed=false duration_ms=8662
    benchmark_longmemeval_test.go:827: [58/72] a2f3aa27 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:21:27 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:21:27 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:21:27 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:21:27 INFO Verification layer initialized
2026/07/28 06:21:27 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:21:27 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:21:35 INFO Observer completed successfully session_id=ku-c7dc5443 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=280 reflector_triggered=false hybrid_indexed=false duration_ms=7360
    benchmark_longmemeval_test.go:827: [59/72] c7dc5443 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:21:42 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:21:42 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:21:42 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:21:42 INFO Verification layer initialized
2026/07/28 06:21:42 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:21:42 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:21:54 INFO Observer completed successfully session_id=ku-06db6396 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=330 reflector_triggered=false hybrid_indexed=false duration_ms=12279
    benchmark_longmemeval_test.go:827: [60/72] 06db6396 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:21:55 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:21:55 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:21:55 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:21:55 INFO Verification layer initialized
2026/07/28 06:21:55 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:21:55 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:22:03 INFO Observer completed successfully session_id=ku-3ba21379 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=270 reflector_triggered=false hybrid_indexed=false duration_ms=8154
    benchmark_longmemeval_test.go:827: [61/72] 3ba21379 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:22:05 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:22:05 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:22:05 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:22:05 INFO Verification layer initialized
2026/07/28 06:22:05 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:22:05 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:22:35 INFO Observer completed successfully session_id=ku-9bbe84a2 resource_id=user_999 scope=resource new_messages=22 skipped_trivial=0 total_tokens=146 reflector_triggered=false hybrid_indexed=false duration_ms=29650
    benchmark_longmemeval_test.go:827: [62/72] 9bbe84a2 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:22:37 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:22:37 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:22:37 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:22:37 INFO Verification layer initialized
2026/07/28 06:22:37 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:22:37 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:22:49 INFO Observer completed successfully session_id=ku-10e09553 resource_id=user_999 scope=resource new_messages=20 skipped_trivial=0 total_tokens=332 reflector_triggered=false hybrid_indexed=false duration_ms=12646
    benchmark_longmemeval_test.go:827: [63/72] 10e09553 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:22:52 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:22:52 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:22:52 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:22:52 INFO Verification layer initialized
2026/07/28 06:22:52 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:22:52 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:24:02 INFO Observer completed successfully session_id=ku-dad224aa resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=109 reflector_triggered=false hybrid_indexed=false duration_ms=70665
    benchmark_longmemeval_test.go:827: [64/72] dad224aa — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:24:05 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:24:05 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:24:05 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:24:05 INFO Verification layer initialized
2026/07/28 06:24:05 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:24:05 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:24:17 INFO Observer completed successfully session_id=ku-ba61f0b9 resource_id=user_999 scope=resource new_messages=22 skipped_trivial=0 total_tokens=159 reflector_triggered=false hybrid_indexed=false duration_ms=12331
    benchmark_longmemeval_test.go:827: [65/72] ba61f0b9 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:24:19 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:24:19 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:24:19 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:24:19 INFO Verification layer initialized
2026/07/28 06:24:19 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:24:19 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:24:34 INFO Observer completed successfully session_id=ku-42ec0761 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=65 reflector_triggered=false hybrid_indexed=false duration_ms=15330
    benchmark_longmemeval_test.go:827: [66/72] 42ec0761 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:24:36 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:24:37 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:24:37 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:24:37 INFO Verification layer initialized
2026/07/28 06:24:37 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:24:37 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:24:41 INFO Observer completed successfully session_id=ku-5c40ec5b resource_id=user_999 scope=resource new_messages=20 skipped_trivial=0 total_tokens=346 reflector_triggered=false hybrid_indexed=false duration_ms=4122
    benchmark_longmemeval_test.go:827: [67/72] 5c40ec5b — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:24:48 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:24:48 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:24:48 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:24:48 INFO Verification layer initialized
2026/07/28 06:24:48 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:24:48 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:24:53 INFO Observer completed successfully session_id=ku-c6853660 resource_id=user_999 scope=resource new_messages=20 skipped_trivial=0 total_tokens=512 reflector_triggered=false hybrid_indexed=false duration_ms=4873
    benchmark_longmemeval_test.go:827: [68/72] c6853660 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:24:55 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:24:55 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:24:55 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:24:55 INFO Verification layer initialized
2026/07/28 06:24:55 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:24:55 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:24:57 INFO Observer completed successfully session_id=ku-26bdc477 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=9 reflector_triggered=false hybrid_indexed=false duration_ms=1934
    benchmark_longmemeval_test.go:827: [69/72] 26bdc477 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 06:25:14 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 06:25:14 INFO Column already exists, skipping table=tickets column=type
2026/07/28 06:25:14 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 06:25:14 INFO Verification layer initialized
2026/07/28 06:25:14 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 06:25:14 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 06:25:54 INFO Observer completed successfully session_id=ku-0977f2af resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=298 reflector_triggered=false hybrid_indexed=false duration_ms=40409
    benchmark_longmemeval_test.go:827: [70/72] 0977f2af — yes
    benchmark_longmemeval_test.go:827: [71/72] SKIP 89941a94 (no cache)
    benchmark_longmemeval_test.go:827: [72/72] SKIP 07741c45 (no cache)
    benchmark_longmemeval_test.go:827: === knowledge_update Summary: 57 passed, 13 failed, 0 skipped ===
--- PASS: TestBenchmarkKnowledgeUpdate (2106.66s)
PASS
ok  	github.com/usememos/memos/server/router/api/v1/agent	2106.707s

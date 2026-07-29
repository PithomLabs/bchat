chaschel@linux:~/Documents/go/bchat$ cd /home/chaschel/Documents/go/bchat && BENCHMARK_LONGMEMEVAL=true OPENROUTER_API_KEY=sk-or-v1-... BENCHMARK_FRESH=true BENCHMARK_ANSWER_MODEL=xiaomi/mimo-v2.5 go test ./server/router/api/v1/agent/ -run TestBenchmarkKnowledgeUpdate -v -count=1 -timeout=90m
=== RUN   TestBenchmarkKnowledgeUpdate
    benchmark_longmemeval_test.go:835: Answer model: xiaomi/mimo-v2.5
    benchmark_longmemeval_test.go:835: Cleared existing JSONL files (BENCHMARK_FRESH=true)
    benchmark_longmemeval_test.go:835: Loaded 500 questions (178 testable), 14065 cache entries
    benchmark_longmemeval_test.go:835: knowledge_update: 72 questions (non-abs)
    store.go:87: failed to load .env file, but it's ok
2026/07/28 16:53:14 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 16:53:14 INFO Column already exists, skipping table=tickets column=type
2026/07/28 16:53:14 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 16:53:14 INFO Verification layer initialized
2026/07/28 16:53:14 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 16:53:14 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 16:53:24 ERROR EstimateTokens using len/4 fallback — globalTokenizer not initialized contentLength=216
2026/07/28 16:53:24 INFO Observer completed successfully session_id=ku-6a1eabeb resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=54 reflector_triggered=false hybrid_indexed=false duration_ms=9834
    benchmark_longmemeval_test.go:835: [1/72] 6a1eabeb — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 16:53:37 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 16:53:38 INFO Column already exists, skipping table=tickets column=type
2026/07/28 16:53:38 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 16:53:38 INFO Verification layer initialized
2026/07/28 16:53:38 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 16:53:38 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 16:55:37 INFO Observer completed successfully session_id=ku-6aeb4375 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=320 reflector_triggered=false hybrid_indexed=false duration_ms=119564
    benchmark_longmemeval_test.go:835: [2/72] 6aeb4375 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 16:55:40 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 16:55:40 INFO Column already exists, skipping table=tickets column=type
2026/07/28 16:55:40 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 16:55:40 INFO Verification layer initialized
2026/07/28 16:55:40 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 16:55:40 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 16:56:45 INFO Observer completed successfully session_id=ku-830ce83f resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=71 reflector_triggered=false hybrid_indexed=false duration_ms=64996
    benchmark_longmemeval_test.go:835: [3/72] 830ce83f — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 16:56:49 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 16:56:49 INFO Column already exists, skipping table=tickets column=type
2026/07/28 16:56:49 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 16:56:49 INFO Verification layer initialized
2026/07/28 16:56:49 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 16:56:49 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 16:57:28 INFO Observer completed successfully session_id=ku-852ce960 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=186 reflector_triggered=false hybrid_indexed=false duration_ms=39126
    benchmark_longmemeval_test.go:835: [4/72] 852ce960 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 16:57:38 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 16:57:39 INFO Column already exists, skipping table=tickets column=type
2026/07/28 16:57:39 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 16:57:39 INFO Verification layer initialized
2026/07/28 16:57:39 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 16:57:39 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 16:57:59 INFO Observer completed successfully session_id=ku-945e3d21 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=654 reflector_triggered=false hybrid_indexed=false duration_ms=20525
    benchmark_longmemeval_test.go:835: [5/72] 945e3d21 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 16:58:08 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 16:58:08 INFO Column already exists, skipping table=tickets column=type
2026/07/28 16:58:08 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 16:58:08 INFO Verification layer initialized
2026/07/28 16:58:08 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 16:58:08 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 16:58:10 INFO Observer completed successfully session_id=ku-d7c942c3 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=4 reflector_triggered=false hybrid_indexed=false duration_ms=1798
    benchmark_longmemeval_test.go:835: [6/72] d7c942c3 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 16:58:14 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 16:58:14 INFO Column already exists, skipping table=tickets column=type
2026/07/28 16:58:14 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 16:58:14 INFO Verification layer initialized
2026/07/28 16:58:14 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 16:58:14 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 16:58:33 INFO Observer completed successfully session_id=ku-71315a70 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=490 reflector_triggered=false hybrid_indexed=false duration_ms=18856
    benchmark_longmemeval_test.go:835: [7/72] 71315a70 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 16:58:40 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 16:58:40 INFO Column already exists, skipping table=tickets column=type
2026/07/28 16:58:40 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 16:58:40 INFO Verification layer initialized
2026/07/28 16:58:40 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 16:58:40 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:00:57 INFO Observer completed successfully session_id=ku-89941a93 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=397 reflector_triggered=false hybrid_indexed=false duration_ms=137691
    benchmark_longmemeval_test.go:835: [8/72] 89941a93 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:01:04 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:01:04 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:01:04 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:01:04 INFO Verification layer initialized
2026/07/28 17:01:04 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:01:04 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:01:14 INFO Observer completed successfully session_id=ku-ce6d2d27 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=201 reflector_triggered=false hybrid_indexed=false duration_ms=9758
    benchmark_longmemeval_test.go:835: [9/72] ce6d2d27 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:01:25 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:01:25 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:01:25 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:01:25 INFO Verification layer initialized
2026/07/28 17:01:25 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:01:25 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:01:40 INFO Observer completed successfully session_id=ku-9ea5eabc resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=318 reflector_triggered=false hybrid_indexed=false duration_ms=14522
    benchmark_longmemeval_test.go:835: [10/72] 9ea5eabc — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:01:46 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:01:46 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:01:46 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:01:46 INFO Verification layer initialized
2026/07/28 17:01:46 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:01:46 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:02:41 INFO Observer completed successfully session_id=ku-07741c44 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=117 reflector_triggered=false hybrid_indexed=false duration_ms=54652
    benchmark_longmemeval_test.go:835: [11/72] 07741c44 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:02:45 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:02:45 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:02:45 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:02:45 INFO Verification layer initialized
2026/07/28 17:02:45 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:02:45 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:02:50 INFO Observer completed successfully session_id=ku-a1eacc2a resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=456 reflector_triggered=false hybrid_indexed=false duration_ms=5003
    benchmark_longmemeval_test.go:835: [12/72] a1eacc2a — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:03:00 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:03:00 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:03:00 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:03:00 INFO Verification layer initialized
2026/07/28 17:03:00 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:03:00 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:03:24 INFO Observer completed successfully session_id=ku-184da446 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=102 reflector_triggered=false hybrid_indexed=false duration_ms=24075
    benchmark_longmemeval_test.go:835: [13/72] 184da446 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:03:30 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:03:30 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:03:30 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:03:30 INFO Verification layer initialized
2026/07/28 17:03:30 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:03:30 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:04:35 INFO Observer completed successfully session_id=ku-031748ae resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=358 reflector_triggered=false hybrid_indexed=false duration_ms=65265
    benchmark_longmemeval_test.go:835: [14/72] 031748ae — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:04:44 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:04:44 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:04:44 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:04:44 INFO Verification layer initialized
2026/07/28 17:04:44 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:04:44 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:04:57 INFO Observer completed successfully session_id=ku-4d6b87c8 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=221 reflector_triggered=false hybrid_indexed=false duration_ms=13083
    benchmark_longmemeval_test.go:835: [15/72] 4d6b87c8 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:05:26 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:05:26 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:05:26 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:05:26 INFO Verification layer initialized
2026/07/28 17:05:26 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:05:26 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:05:46 INFO Observer completed successfully session_id=ku-0f05491a resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=490 reflector_triggered=false hybrid_indexed=false duration_ms=19144
    benchmark_longmemeval_test.go:835: [16/72] 0f05491a — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:05:51 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:05:51 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:05:51 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:05:51 INFO Verification layer initialized
2026/07/28 17:05:51 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:05:51 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:08:51 ERROR Observer LLM call failed session_id=ku-08e075c7 error="LLM call failed: context deadline exceeded (Client.Timeout or context cancellation while reading body)"
    benchmark_longmemeval_test.go:835: [17/72] 08e075c7 — 
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:08:51 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:08:51 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:08:51 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:08:51 INFO Verification layer initialized
2026/07/28 17:08:51 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:08:51 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:09:11 INFO Observer completed successfully session_id=ku-f9e8c073 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=523 reflector_triggered=false hybrid_indexed=false duration_ms=20013
    benchmark_longmemeval_test.go:835: [18/72] f9e8c073 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:09:16 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:09:16 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:09:16 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:09:16 INFO Verification layer initialized
2026/07/28 17:09:16 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:09:16 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:09:28 INFO Observer completed successfully session_id=ku-41698283 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=147 reflector_triggered=false hybrid_indexed=false duration_ms=12623
    benchmark_longmemeval_test.go:835: [19/72] 41698283 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:09:40 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:09:40 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:09:40 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:09:40 INFO Verification layer initialized
2026/07/28 17:09:40 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:09:40 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:10:56 INFO Observer completed successfully session_id=ku-2698e78f resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=481 reflector_triggered=false hybrid_indexed=false duration_ms=75498
    benchmark_longmemeval_test.go:835: [20/72] 2698e78f — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:11:04 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:11:04 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:11:04 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:11:04 INFO Verification layer initialized
2026/07/28 17:11:04 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:11:04 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:11:41 INFO Observer completed successfully session_id=ku-b6019101 resource_id=user_999 scope=resource new_messages=22 skipped_trivial=0 total_tokens=501 reflector_triggered=false hybrid_indexed=false duration_ms=36829
    benchmark_longmemeval_test.go:835: [21/72] b6019101 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:11:50 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:11:50 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:11:50 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:11:50 INFO Verification layer initialized
2026/07/28 17:11:50 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:11:50 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:11:53 INFO Observer completed successfully session_id=ku-45dc21b6 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=4 reflector_triggered=false hybrid_indexed=false duration_ms=2736
    benchmark_longmemeval_test.go:835: [22/72] 45dc21b6 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:11:59 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:11:59 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:11:59 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:11:59 INFO Verification layer initialized
2026/07/28 17:11:59 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:11:59 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:12:16 INFO Observer completed successfully session_id=ku-5a4f22c0 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=68 reflector_triggered=false hybrid_indexed=false duration_ms=17035
    benchmark_longmemeval_test.go:835: [23/72] 5a4f22c0 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:12:23 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:12:23 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:12:23 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:12:23 INFO Verification layer initialized
2026/07/28 17:12:23 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:12:23 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:12:33 INFO Observer completed successfully session_id=ku-6071bd76 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=318 reflector_triggered=false hybrid_indexed=false duration_ms=9699
    benchmark_longmemeval_test.go:835: [24/72] 6071bd76 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:12:41 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:12:41 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:12:41 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:12:41 INFO Verification layer initialized
2026/07/28 17:12:41 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:12:41 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:12:43 INFO Observer completed successfully session_id=ku-e493bb7c resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=4 reflector_triggered=false hybrid_indexed=false duration_ms=2524
    benchmark_longmemeval_test.go:835: [25/72] e493bb7c — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:12:49 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:12:49 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:12:49 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:12:49 INFO Verification layer initialized
2026/07/28 17:12:49 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:12:49 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:13:30 INFO Observer completed successfully session_id=ku-618f13b2 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=171 reflector_triggered=false hybrid_indexed=false duration_ms=40692
    benchmark_longmemeval_test.go:835: [26/72] 618f13b2 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:13:33 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:13:33 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:13:33 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:13:33 INFO Verification layer initialized
2026/07/28 17:13:33 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:13:33 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:14:10 INFO Observer completed successfully session_id=ku-72e3ee87 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=339 reflector_triggered=false hybrid_indexed=false duration_ms=36735
    benchmark_longmemeval_test.go:835: [27/72] 72e3ee87 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:14:21 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:14:21 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:14:21 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:14:21 INFO Verification layer initialized
2026/07/28 17:14:21 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:14:21 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:17:21 ERROR Observer LLM call failed session_id=ku-c4ea545c error="LLM call failed: context deadline exceeded (Client.Timeout or context cancellation while reading body)"
    benchmark_longmemeval_test.go:835: [28/72] c4ea545c — 
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:17:21 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:17:21 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:17:21 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:17:21 INFO Verification layer initialized
2026/07/28 17:17:21 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:17:21 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:18:17 INFO Observer completed successfully session_id=ku-01493427 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=373 reflector_triggered=false hybrid_indexed=false duration_ms=55639
    benchmark_longmemeval_test.go:835: [29/72] 01493427 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:18:32 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:18:32 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:18:32 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:18:32 INFO Verification layer initialized
2026/07/28 17:18:32 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:18:32 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:18:53 INFO Observer completed successfully session_id=ku-6a27ffc2 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=576 reflector_triggered=false hybrid_indexed=false duration_ms=21552
    benchmark_longmemeval_test.go:835: [30/72] 6a27ffc2 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:19:11 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:19:11 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:19:11 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:19:11 INFO Verification layer initialized
2026/07/28 17:19:11 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:19:11 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:19:29 INFO Observer completed successfully session_id=ku-2133c1b5 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=537 reflector_triggered=false hybrid_indexed=false duration_ms=18139
    benchmark_longmemeval_test.go:835: [31/72] 2133c1b5 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:19:37 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:19:37 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:19:37 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:19:37 INFO Verification layer initialized
2026/07/28 17:19:37 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:19:37 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:20:12 INFO Observer completed successfully session_id=ku-18bc8abd resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=358 reflector_triggered=false hybrid_indexed=false duration_ms=34682
    benchmark_longmemeval_test.go:835: [32/72] 18bc8abd — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:20:18 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:20:18 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:20:18 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:20:18 INFO Verification layer initialized
2026/07/28 17:20:18 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:20:18 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:20:48 INFO Observer completed successfully session_id=ku-db467c8c resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=457 reflector_triggered=false hybrid_indexed=false duration_ms=29549
    benchmark_longmemeval_test.go:835: [33/72] db467c8c — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:20:52 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:20:52 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:20:52 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:20:52 INFO Verification layer initialized
2026/07/28 17:20:52 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:20:52 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:21:26 INFO Observer completed successfully session_id=ku-7a87bd0c resource_id=user_999 scope=resource new_messages=22 skipped_trivial=0 total_tokens=136 reflector_triggered=false hybrid_indexed=false duration_ms=34039
    benchmark_longmemeval_test.go:835: [34/72] 7a87bd0c — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:21:36 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:21:36 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:21:36 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:21:36 INFO Verification layer initialized
2026/07/28 17:21:36 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:21:36 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:23:00 INFO Observer completed successfully session_id=ku-e61a7584 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=241 reflector_triggered=false hybrid_indexed=false duration_ms=84502
    benchmark_longmemeval_test.go:835: [35/72] e61a7584 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:23:11 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:23:11 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:23:11 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:23:11 INFO Verification layer initialized
2026/07/28 17:23:11 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:23:11 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:23:14 INFO Observer completed successfully session_id=ku-1cea1afa resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=9 reflector_triggered=false hybrid_indexed=false duration_ms=2695
    benchmark_longmemeval_test.go:835: [36/72] 1cea1afa — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:23:25 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:23:25 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:23:25 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:23:25 INFO Verification layer initialized
2026/07/28 17:23:25 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:23:25 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:23:32 INFO Observer completed successfully session_id=ku-ed4ddc30 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=220 reflector_triggered=false hybrid_indexed=false duration_ms=7262
    benchmark_longmemeval_test.go:835: [37/72] ed4ddc30 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:23:35 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:23:35 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:23:35 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:23:35 INFO Verification layer initialized
2026/07/28 17:23:35 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:23:35 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:23:59 INFO Observer completed successfully session_id=ku-8fb83627 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=49 reflector_triggered=false hybrid_indexed=false duration_ms=24170
    benchmark_longmemeval_test.go:835: [38/72] 8fb83627 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:24:05 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:24:05 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:24:05 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:24:05 INFO Verification layer initialized
2026/07/28 17:24:05 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:24:05 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:24:32 INFO Observer completed successfully session_id=ku-b01defab resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=95 reflector_triggered=false hybrid_indexed=false duration_ms=27380
    benchmark_longmemeval_test.go:835: [39/72] b01defab — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:24:50 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:24:50 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:24:50 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:24:50 INFO Verification layer initialized
2026/07/28 17:24:50 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:24:50 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:25:51 INFO Observer completed successfully session_id=ku-22d2cb42 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=321 reflector_triggered=false hybrid_indexed=false duration_ms=61200
    benchmark_longmemeval_test.go:835: [40/72] 22d2cb42 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:25:56 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:25:56 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:25:56 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:25:56 INFO Verification layer initialized
2026/07/28 17:25:56 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:25:56 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:26:30 INFO Observer completed successfully session_id=ku-0e4e4c46 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=67 reflector_triggered=false hybrid_indexed=false duration_ms=34128
    benchmark_longmemeval_test.go:835: [41/72] 0e4e4c46 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:26:45 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:26:45 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:26:45 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:26:45 INFO Verification layer initialized
2026/07/28 17:26:45 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:26:45 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:27:40 INFO Observer completed successfully session_id=ku-4b24c848 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=347 reflector_triggered=false hybrid_indexed=false duration_ms=54954
    benchmark_longmemeval_test.go:835: [42/72] 4b24c848 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:27:55 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:27:55 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:27:55 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:27:55 INFO Verification layer initialized
2026/07/28 17:27:55 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:27:55 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:28:18 INFO Observer completed successfully session_id=ku-7e974930 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=335 reflector_triggered=false hybrid_indexed=false duration_ms=22798
    benchmark_longmemeval_test.go:835: [43/72] 7e974930 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:28:26 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:28:26 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:28:26 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:28:26 INFO Verification layer initialized
2026/07/28 17:28:26 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:28:26 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:29:19 INFO Observer completed successfully session_id=ku-603deb26 resource_id=user_999 scope=resource new_messages=22 skipped_trivial=0 total_tokens=255 reflector_triggered=false hybrid_indexed=false duration_ms=52495
    benchmark_longmemeval_test.go:835: [44/72] 603deb26 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:29:24 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:29:24 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:29:24 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:29:24 INFO Verification layer initialized
2026/07/28 17:29:24 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:29:24 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:30:52 INFO Observer completed successfully session_id=ku-59524333 resource_id=user_999 scope=resource new_messages=22 skipped_trivial=0 total_tokens=796 reflector_triggered=false hybrid_indexed=false duration_ms=88913
    benchmark_longmemeval_test.go:835: [45/72] 59524333 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:30:56 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:30:56 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:30:56 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:30:56 INFO Verification layer initialized
2026/07/28 17:30:56 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:30:56 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:30:59 INFO Observer completed successfully session_id=ku-5831f84d resource_id=user_999 scope=resource new_messages=22 skipped_trivial=0 total_tokens=9 reflector_triggered=false hybrid_indexed=false duration_ms=2368
    benchmark_longmemeval_test.go:835: [46/72] 5831f84d — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:31:01 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:31:01 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:31:01 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:31:01 INFO Verification layer initialized
2026/07/28 17:31:01 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:31:01 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:31:13 INFO Observer completed successfully session_id=ku-eace081b resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=130 reflector_triggered=false hybrid_indexed=false duration_ms=12455
    benchmark_longmemeval_test.go:835: [47/72] eace081b — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:31:17 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:31:17 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:31:17 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:31:17 INFO Verification layer initialized
2026/07/28 17:31:17 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:31:17 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:31:32 INFO Observer completed successfully session_id=ku-affe2881 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=364 reflector_triggered=false hybrid_indexed=false duration_ms=14930
    benchmark_longmemeval_test.go:835: [48/72] affe2881 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:31:34 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:31:34 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:31:34 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:31:34 INFO Verification layer initialized
2026/07/28 17:31:34 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:31:34 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:32:21 INFO Observer completed successfully session_id=ku-50635ada resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=158 reflector_triggered=false hybrid_indexed=false duration_ms=46583
    benchmark_longmemeval_test.go:835: [49/72] 50635ada — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:32:29 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:32:29 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:32:29 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:32:29 INFO Verification layer initialized
2026/07/28 17:32:29 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:32:29 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:34:55 INFO Observer completed successfully session_id=ku-e66b632c resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=576 reflector_triggered=false hybrid_indexed=false duration_ms=145662
    benchmark_longmemeval_test.go:835: [50/72] e66b632c — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:34:59 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:34:59 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:34:59 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:34:59 INFO Verification layer initialized
2026/07/28 17:34:59 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:34:59 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:37:59 ERROR Observer LLM call failed session_id=ku-0ddfec37 error="LLM call failed: context deadline exceeded (Client.Timeout or context cancellation while reading body)"
    benchmark_longmemeval_test.go:835: [51/72] 0ddfec37 — 
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:37:59 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:37:59 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:37:59 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:37:59 INFO Verification layer initialized
2026/07/28 17:37:59 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:37:59 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:38:16 INFO Observer completed successfully session_id=ku-f685340e resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=402 reflector_triggered=false hybrid_indexed=false duration_ms=16662
    benchmark_longmemeval_test.go:835: [52/72] f685340e — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:38:20 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:38:20 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:38:20 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:38:20 INFO Verification layer initialized
2026/07/28 17:38:20 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:38:20 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:38:36 INFO Observer completed successfully session_id=ku-cc5ded98 resource_id=user_999 scope=resource new_messages=22 skipped_trivial=0 total_tokens=289 reflector_triggered=false hybrid_indexed=false duration_ms=15840
    benchmark_longmemeval_test.go:835: [53/72] cc5ded98 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:38:38 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:38:39 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:38:39 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:38:39 INFO Verification layer initialized
2026/07/28 17:38:39 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:38:39 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:38:53 INFO Observer completed successfully session_id=ku-dfde3500 resource_id=user_999 scope=resource new_messages=22 skipped_trivial=0 total_tokens=154 reflector_triggered=false hybrid_indexed=false duration_ms=14595
    benchmark_longmemeval_test.go:835: [54/72] dfde3500 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:38:56 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:38:56 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:38:56 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:38:56 INFO Verification layer initialized
2026/07/28 17:38:56 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:38:56 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:40:41 INFO Observer completed successfully session_id=ku-69fee5aa resource_id=user_999 scope=resource new_messages=22 skipped_trivial=0 total_tokens=180 reflector_triggered=false hybrid_indexed=false duration_ms=104915
    benchmark_longmemeval_test.go:835: [55/72] 69fee5aa — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:40:44 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:40:44 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:40:44 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:40:44 INFO Verification layer initialized
2026/07/28 17:40:44 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:40:44 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:41:24 INFO Observer completed successfully session_id=ku-7401057b resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=361 reflector_triggered=false hybrid_indexed=false duration_ms=39421
    benchmark_longmemeval_test.go:835: [56/72] 7401057b — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:41:27 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:41:27 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:41:27 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:41:27 INFO Verification layer initialized
2026/07/28 17:41:27 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:41:27 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:41:39 INFO Observer completed successfully session_id=ku-cf22b7bf resource_id=user_999 scope=resource new_messages=22 skipped_trivial=0 total_tokens=482 reflector_triggered=false hybrid_indexed=false duration_ms=12818
    benchmark_longmemeval_test.go:835: [57/72] cf22b7bf — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:41:43 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:41:43 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:41:43 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:41:43 INFO Verification layer initialized
2026/07/28 17:41:43 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:41:43 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:42:07 INFO Observer completed successfully session_id=ku-a2f3aa27 resource_id=user_999 scope=resource new_messages=20 skipped_trivial=0 total_tokens=674 reflector_triggered=false hybrid_indexed=false duration_ms=23292
    benchmark_longmemeval_test.go:835: [58/72] a2f3aa27 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:42:11 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:42:11 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:42:11 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:42:11 INFO Verification layer initialized
2026/07/28 17:42:11 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:42:11 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:42:15 INFO Observer completed successfully session_id=ku-c7dc5443 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=369 reflector_triggered=false hybrid_indexed=false duration_ms=4572
    benchmark_longmemeval_test.go:835: [59/72] c7dc5443 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:42:19 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:42:19 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:42:19 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:42:19 INFO Verification layer initialized
2026/07/28 17:42:19 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:42:19 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:42:47 INFO Observer completed successfully session_id=ku-06db6396 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=270 reflector_triggered=false hybrid_indexed=false duration_ms=27774
    benchmark_longmemeval_test.go:835: [60/72] 06db6396 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:42:59 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:42:59 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:42:59 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:42:59 INFO Verification layer initialized
2026/07/28 17:42:59 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:42:59 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:44:56 INFO Observer completed successfully session_id=ku-3ba21379 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=437 reflector_triggered=false hybrid_indexed=false duration_ms=116589
    benchmark_longmemeval_test.go:835: [61/72] 3ba21379 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:45:01 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:45:01 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:45:01 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:45:01 INFO Verification layer initialized
2026/07/28 17:45:01 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:45:01 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:45:27 INFO Observer completed successfully session_id=ku-9bbe84a2 resource_id=user_999 scope=resource new_messages=22 skipped_trivial=0 total_tokens=413 reflector_triggered=false hybrid_indexed=false duration_ms=25975
    benchmark_longmemeval_test.go:835: [62/72] 9bbe84a2 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:45:32 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:45:32 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:45:32 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:45:32 INFO Verification layer initialized
2026/07/28 17:45:32 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:45:32 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:46:30 INFO Observer completed successfully session_id=ku-10e09553 resource_id=user_999 scope=resource new_messages=20 skipped_trivial=0 total_tokens=416 reflector_triggered=false hybrid_indexed=false duration_ms=58002
    benchmark_longmemeval_test.go:835: [63/72] 10e09553 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:46:33 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:46:33 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:46:33 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:46:33 INFO Verification layer initialized
2026/07/28 17:46:33 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:46:33 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:47:32 INFO Observer completed successfully session_id=ku-dad224aa resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=346 reflector_triggered=false hybrid_indexed=false duration_ms=59115
    benchmark_longmemeval_test.go:835: [64/72] dad224aa — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:47:45 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:47:45 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:47:45 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:47:45 INFO Verification layer initialized
2026/07/28 17:47:45 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:47:45 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:48:26 INFO Observer completed successfully session_id=ku-ba61f0b9 resource_id=user_999 scope=resource new_messages=22 skipped_trivial=0 total_tokens=495 reflector_triggered=false hybrid_indexed=false duration_ms=41487
    benchmark_longmemeval_test.go:835: [65/72] ba61f0b9 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:48:31 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:48:31 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:48:31 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:48:31 INFO Verification layer initialized
2026/07/28 17:48:31 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:48:31 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:48:48 INFO Observer completed successfully session_id=ku-42ec0761 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=491 reflector_triggered=false hybrid_indexed=false duration_ms=16871
    benchmark_longmemeval_test.go:835: [66/72] 42ec0761 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:48:52 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:48:52 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:48:52 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:48:52 INFO Verification layer initialized
2026/07/28 17:48:52 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:48:52 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:49:19 INFO Observer completed successfully session_id=ku-5c40ec5b resource_id=user_999 scope=resource new_messages=20 skipped_trivial=0 total_tokens=227 reflector_triggered=false hybrid_indexed=false duration_ms=27449
    benchmark_longmemeval_test.go:835: [67/72] 5c40ec5b — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:49:22 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:49:22 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:49:22 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:49:22 INFO Verification layer initialized
2026/07/28 17:49:22 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:49:22 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:50:11 INFO Observer completed successfully session_id=ku-c6853660 resource_id=user_999 scope=resource new_messages=20 skipped_trivial=0 total_tokens=185 reflector_triggered=false hybrid_indexed=false duration_ms=48901
    benchmark_longmemeval_test.go:835: [68/72] c6853660 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:50:15 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:50:15 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:50:15 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:50:15 INFO Verification layer initialized
2026/07/28 17:50:15 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:50:15 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:53:04 INFO Observer completed successfully session_id=ku-26bdc477 resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=430 reflector_triggered=false hybrid_indexed=false duration_ms=169377
    benchmark_longmemeval_test.go:835: [69/72] 26bdc477 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 17:53:09 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 17:53:09 INFO Column already exists, skipping table=tickets column=type
2026/07/28 17:53:09 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 17:53:09 INFO Verification layer initialized
2026/07/28 17:53:09 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 17:53:09 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 17:54:03 INFO Observer completed successfully session_id=ku-0977f2af resource_id=user_999 scope=resource new_messages=24 skipped_trivial=0 total_tokens=130 reflector_triggered=false hybrid_indexed=false duration_ms=53755
    benchmark_longmemeval_test.go:835: [70/72] 0977f2af — no
    benchmark_longmemeval_test.go:835: [71/72] SKIP 89941a94 (no cache)
    benchmark_longmemeval_test.go:835: [72/72] SKIP 07741c45 (no cache)
    benchmark_longmemeval_test.go:835: === knowledge_update Summary: 53 passed, 14 failed, 3 skipped ===
--- PASS: TestBenchmarkKnowledgeUpdate (3658.46s)
PASS
ok  	github.com/usememos/memos/server/router/api/v1/agent	3658.501s

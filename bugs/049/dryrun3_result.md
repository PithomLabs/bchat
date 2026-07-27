chaschel@linux:~/Documents/go/bchat$ cd /home/chaschel/Documents/go/bchat && BENCHMARK_LONGMEMEVAL=true OPENROUTER_API_KEY=sk-or-v1-... go test ./server/router/api/v1/agent/ -run TestBenchmarkSingleHop -v -count=1 -timeout=30m
=== RUN   TestBenchmarkSingleHop
    benchmark_longmemeval_test.go:815: Loaded 500 questions (178 testable), 14065 cache entries
    benchmark_longmemeval_test.go:815: single_hop: 64 questions (non-abs)
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:40:39 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:40:39 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:40:39 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:40:39 INFO Verification layer initialized
2026/07/28 03:40:39 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:40:39 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:40:58 ERROR EstimateTokens using len/4 fallback — globalTokenizer not initialized contentLength=332
2026/07/28 03:40:58 INFO Observer completed successfully session_id=sh-e47becba resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=83 reflector_triggered=false hybrid_indexed=false duration_ms=18803
    benchmark_longmemeval_test.go:815: [1/64] e47becba — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:41:14 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:41:14 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:41:14 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:41:14 INFO Verification layer initialized
2026/07/28 03:41:14 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:41:14 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:42:05 INFO Observer completed successfully session_id=sh-118b2229 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=360 reflector_triggered=false hybrid_indexed=false duration_ms=51235
    benchmark_longmemeval_test.go:815: [2/64] 118b2229 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:42:07 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:42:07 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:42:07 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:42:07 INFO Verification layer initialized
2026/07/28 03:42:07 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:42:07 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:42:18 INFO Observer completed successfully session_id=sh-51a45a95 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=253 reflector_triggered=false hybrid_indexed=false duration_ms=11380
    benchmark_longmemeval_test.go:815: [3/64] 51a45a95 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:42:33 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:42:33 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:42:33 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:42:33 INFO Verification layer initialized
2026/07/28 03:42:33 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:42:33 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:42:45 INFO Observer completed successfully session_id=sh-58bf7951 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=252 reflector_triggered=false hybrid_indexed=false duration_ms=11653
    benchmark_longmemeval_test.go:815: [4/64] 58bf7951 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:42:48 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:42:48 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:42:48 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:42:48 INFO Verification layer initialized
2026/07/28 03:42:48 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:42:48 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:42:53 INFO Observer completed successfully session_id=sh-1e043500 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=64 reflector_triggered=false hybrid_indexed=false duration_ms=5002
    benchmark_longmemeval_test.go:815: [5/64] 1e043500 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:42:56 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:42:56 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:42:56 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:42:56 INFO Verification layer initialized
2026/07/28 03:42:56 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:42:56 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:43:24 INFO Observer completed successfully session_id=sh-c5e8278d resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=201 reflector_triggered=false hybrid_indexed=false duration_ms=28333
    benchmark_longmemeval_test.go:815: [6/64] c5e8278d — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:43:36 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:43:36 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:43:36 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:43:36 INFO Verification layer initialized
2026/07/28 03:43:36 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:43:36 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:43:54 INFO Observer completed successfully session_id=sh-6ade9755 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=160 reflector_triggered=false hybrid_indexed=false duration_ms=17112
    benchmark_longmemeval_test.go:815: [7/64] 6ade9755 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:44:00 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:44:00 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:44:00 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:44:00 INFO Verification layer initialized
2026/07/28 03:44:00 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:44:00 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:44:16 INFO Observer completed successfully session_id=sh-6f9b354f resource_id=user_999 scope=resource new_messages=10 skipped_trivial=0 total_tokens=96 reflector_triggered=false hybrid_indexed=false duration_ms=15706
    benchmark_longmemeval_test.go:815: [8/64] 6f9b354f — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:44:19 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:44:19 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:44:19 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:44:19 INFO Verification layer initialized
2026/07/28 03:44:19 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:44:19 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:44:22 INFO Observer completed successfully session_id=sh-58ef2f1c resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=85 reflector_triggered=false hybrid_indexed=false duration_ms=3653
    benchmark_longmemeval_test.go:815: [9/64] 58ef2f1c — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:44:24 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:44:24 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:44:24 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:44:24 INFO Verification layer initialized
2026/07/28 03:44:24 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:44:24 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:44:34 INFO Observer completed successfully session_id=sh-f8c5f88b resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=286 reflector_triggered=false hybrid_indexed=false duration_ms=10221
    benchmark_longmemeval_test.go:815: [10/64] f8c5f88b — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:44:39 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:44:39 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:44:39 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:44:39 INFO Verification layer initialized
2026/07/28 03:44:39 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:44:39 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:44:54 INFO Observer completed successfully session_id=sh-5d3d2817 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=380 reflector_triggered=false hybrid_indexed=false duration_ms=14482
    benchmark_longmemeval_test.go:815: [11/64] 5d3d2817 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:44:56 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:44:56 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:44:56 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:44:56 INFO Verification layer initialized
2026/07/28 03:44:56 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:44:56 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:45:10 INFO Observer completed successfully session_id=sh-7527f7e2 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=173 reflector_triggered=false hybrid_indexed=false duration_ms=13949
    benchmark_longmemeval_test.go:815: [12/64] 7527f7e2 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:45:14 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:45:14 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:45:14 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:45:14 INFO Verification layer initialized
2026/07/28 03:45:14 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:45:14 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:45:25 INFO Observer completed successfully session_id=sh-c960da58 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=301 reflector_triggered=false hybrid_indexed=false duration_ms=10970
    benchmark_longmemeval_test.go:815: [13/64] c960da58 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:45:34 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:45:34 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:45:34 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:45:34 INFO Verification layer initialized
2026/07/28 03:45:34 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:45:34 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:45:37 INFO Observer completed successfully session_id=sh-3b6f954b resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=9 reflector_triggered=false hybrid_indexed=false duration_ms=2221
    benchmark_longmemeval_test.go:815: [14/64] 3b6f954b — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:45:39 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:45:39 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:45:39 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:45:39 INFO Verification layer initialized
2026/07/28 03:45:39 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:45:39 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:45:47 INFO Observer completed successfully session_id=sh-726462e0 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=221 reflector_triggered=false hybrid_indexed=false duration_ms=8280
    benchmark_longmemeval_test.go:815: [15/64] 726462e0 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:45:49 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:45:49 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:45:49 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:45:49 INFO Verification layer initialized
2026/07/28 03:45:49 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:45:49 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:46:04 INFO Observer completed successfully session_id=sh-94f70d80 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=174 reflector_triggered=false hybrid_indexed=false duration_ms=14988
    benchmark_longmemeval_test.go:815: [16/64] 94f70d80 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:46:07 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:46:07 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:46:07 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:46:07 INFO Verification layer initialized
2026/07/28 03:46:07 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:46:07 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:46:17 INFO Observer completed successfully session_id=sh-66f24dbb resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=292 reflector_triggered=false hybrid_indexed=false duration_ms=9971
    benchmark_longmemeval_test.go:815: [17/64] 66f24dbb — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:46:23 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:46:23 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:46:23 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:46:23 INFO Verification layer initialized
2026/07/28 03:46:23 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:46:23 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:47:03 INFO Observer completed successfully session_id=sh-ad7109d1 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=121 reflector_triggered=false hybrid_indexed=false duration_ms=39737
    benchmark_longmemeval_test.go:815: [18/64] ad7109d1 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:47:52 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:47:52 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:47:52 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:47:52 INFO Verification layer initialized
2026/07/28 03:47:52 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:47:52 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:48:18 INFO Observer completed successfully session_id=sh-af8d2e46 resource_id=user_999 scope=resource new_messages=10 skipped_trivial=0 total_tokens=227 reflector_triggered=false hybrid_indexed=false duration_ms=25496
    benchmark_longmemeval_test.go:815: [19/64] af8d2e46 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:48:20 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:48:20 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:48:20 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:48:20 INFO Verification layer initialized
2026/07/28 03:48:20 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:48:20 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:49:44 INFO Observer completed successfully session_id=sh-dccbc061 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=183 reflector_triggered=false hybrid_indexed=false duration_ms=83837
    benchmark_longmemeval_test.go:815: [20/64] dccbc061 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:49:45 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:49:45 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:49:45 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:49:45 INFO Verification layer initialized
2026/07/28 03:49:45 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:49:45 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:49:57 INFO Observer completed successfully session_id=sh-c8c3f81d resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=180 reflector_triggered=false hybrid_indexed=false duration_ms=11434
    benchmark_longmemeval_test.go:815: [21/64] c8c3f81d — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:49:59 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:49:59 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:49:59 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:49:59 INFO Verification layer initialized
2026/07/28 03:49:59 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:49:59 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:50:08 INFO Observer completed successfully session_id=sh-8ebdbe50 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=377 reflector_triggered=false hybrid_indexed=false duration_ms=9147
    benchmark_longmemeval_test.go:815: [22/64] 8ebdbe50 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:50:13 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:50:13 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:50:13 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:50:13 INFO Verification layer initialized
2026/07/28 03:50:13 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:50:13 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:50:23 INFO Observer completed successfully session_id=sh-6b168ec8 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=214 reflector_triggered=false hybrid_indexed=false duration_ms=9057
    benchmark_longmemeval_test.go:815: [23/64] 6b168ec8 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:50:25 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:50:25 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:50:25 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:50:25 INFO Verification layer initialized
2026/07/28 03:50:25 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:50:25 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:50:34 INFO Observer completed successfully session_id=sh-75499fd8 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=250 reflector_triggered=false hybrid_indexed=false duration_ms=9023
    benchmark_longmemeval_test.go:815: [24/64] 75499fd8 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:50:36 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:50:36 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:50:36 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:50:36 INFO Verification layer initialized
2026/07/28 03:50:36 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:50:36 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:50:57 INFO Observer completed successfully session_id=sh-21436231 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=145 reflector_triggered=false hybrid_indexed=false duration_ms=21491
    benchmark_longmemeval_test.go:815: [25/64] 21436231 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:51:17 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:51:17 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:51:17 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:51:17 INFO Verification layer initialized
2026/07/28 03:51:17 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:51:17 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:52:08 INFO Observer completed successfully session_id=sh-95bcc1c8 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=296 reflector_triggered=false hybrid_indexed=false duration_ms=51233
    benchmark_longmemeval_test.go:815: [26/64] 95bcc1c8 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:52:10 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:52:10 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:52:10 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:52:10 INFO Verification layer initialized
2026/07/28 03:52:10 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:52:10 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:52:25 INFO Observer completed successfully session_id=sh-0862e8bf resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=30 reflector_triggered=false hybrid_indexed=false duration_ms=14208
    benchmark_longmemeval_test.go:815: [27/64] 0862e8bf — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:52:30 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:52:30 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:52:30 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:52:30 INFO Verification layer initialized
2026/07/28 03:52:30 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:52:30 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:53:09 INFO Observer completed successfully session_id=sh-853b0a1d resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=280 reflector_triggered=false hybrid_indexed=false duration_ms=38942
    benchmark_longmemeval_test.go:815: [28/64] 853b0a1d — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:53:11 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:53:11 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:53:11 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:53:11 INFO Verification layer initialized
2026/07/28 03:53:11 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:53:11 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:53:29 INFO Observer completed successfully session_id=sh-a06e4cfe resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=176 reflector_triggered=false hybrid_indexed=false duration_ms=18203
    benchmark_longmemeval_test.go:815: [29/64] a06e4cfe — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:53:31 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:53:31 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:53:31 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:53:31 INFO Verification layer initialized
2026/07/28 03:53:31 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:53:31 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:53:34 INFO Observer completed successfully session_id=sh-37d43f65 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=180 reflector_triggered=false hybrid_indexed=false duration_ms=3089
    benchmark_longmemeval_test.go:815: [30/64] 37d43f65 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:53:39 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:53:39 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:53:39 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:53:39 INFO Verification layer initialized
2026/07/28 03:53:39 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:53:39 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:53:51 INFO Observer completed successfully session_id=sh-b86304ba resource_id=user_999 scope=resource new_messages=10 skipped_trivial=0 total_tokens=118 reflector_triggered=false hybrid_indexed=false duration_ms=11338
    benchmark_longmemeval_test.go:815: [31/64] b86304ba — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:54:03 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:54:03 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:54:03 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:54:03 INFO Verification layer initialized
2026/07/28 03:54:03 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:54:03 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:54:38 INFO Observer completed successfully session_id=sh-d52b4f67 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=187 reflector_triggered=false hybrid_indexed=false duration_ms=34566
    benchmark_longmemeval_test.go:815: [32/64] d52b4f67 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:54:40 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:54:40 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:54:40 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:54:40 INFO Verification layer initialized
2026/07/28 03:54:40 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:54:40 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:54:41 INFO Observer completed successfully session_id=sh-25e5aa4f resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=9 reflector_triggered=false hybrid_indexed=false duration_ms=1756
    benchmark_longmemeval_test.go:815: [33/64] 25e5aa4f — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:54:54 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:54:54 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:54:54 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:54:54 INFO Verification layer initialized
2026/07/28 03:54:54 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:54:54 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:55:20 INFO Observer completed successfully session_id=sh-caf9ead2 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=246 reflector_triggered=false hybrid_indexed=false duration_ms=25853
    benchmark_longmemeval_test.go:815: [34/64] caf9ead2 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:55:23 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:55:23 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:55:23 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:55:23 INFO Verification layer initialized
2026/07/28 03:55:23 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:55:23 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:55:32 INFO Observer completed successfully session_id=sh-8550ddae resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=381 reflector_triggered=false hybrid_indexed=false duration_ms=9277
    benchmark_longmemeval_test.go:815: [35/64] 8550ddae — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:55:42 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:55:42 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:55:42 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:55:42 INFO Verification layer initialized
2026/07/28 03:55:42 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:55:42 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:57:33 INFO Observer completed successfully session_id=sh-60d45044 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=765 reflector_triggered=false hybrid_indexed=false duration_ms=111214
    benchmark_longmemeval_test.go:815: [36/64] 60d45044 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:57:36 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:57:36 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:57:36 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:57:36 INFO Verification layer initialized
2026/07/28 03:57:36 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:57:36 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:58:04 INFO Observer completed successfully session_id=sh-3f1e9474 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=813 reflector_triggered=false hybrid_indexed=false duration_ms=28292
    benchmark_longmemeval_test.go:815: [37/64] 3f1e9474 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:58:07 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:58:07 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:58:07 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:58:07 INFO Verification layer initialized
2026/07/28 03:58:07 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:58:07 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:58:18 INFO Observer completed successfully session_id=sh-86b68151 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=172 reflector_triggered=false hybrid_indexed=false duration_ms=11437
    benchmark_longmemeval_test.go:815: [38/64] 86b68151 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:58:32 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:58:32 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:58:32 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:58:32 INFO Verification layer initialized
2026/07/28 03:58:32 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:58:32 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:58:41 INFO Observer completed successfully session_id=sh-577d4d32 resource_id=user_999 scope=resource new_messages=10 skipped_trivial=0 total_tokens=268 reflector_triggered=false hybrid_indexed=false duration_ms=8370
    benchmark_longmemeval_test.go:815: [39/64] 577d4d32 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:58:43 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:58:43 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:58:43 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:58:43 INFO Verification layer initialized
2026/07/28 03:58:43 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:58:43 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:58:44 INFO Observer completed successfully session_id=sh-ec81a493 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=9 reflector_triggered=false hybrid_indexed=false duration_ms=1388
    benchmark_longmemeval_test.go:815: [40/64] ec81a493 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:58:52 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:58:52 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:58:52 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:58:52 INFO Verification layer initialized
2026/07/28 03:58:52 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:58:52 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:59:18 INFO Observer completed successfully session_id=sh-15745da0 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=395 reflector_triggered=false hybrid_indexed=false duration_ms=25975
    benchmark_longmemeval_test.go:815: [41/64] 15745da0 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 03:59:20 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 03:59:20 INFO Column already exists, skipping table=tickets column=type
2026/07/28 03:59:20 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 03:59:20 INFO Verification layer initialized
2026/07/28 03:59:20 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 03:59:20 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 03:59:57 INFO Observer completed successfully session_id=sh-e01b8e2f resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=647 reflector_triggered=false hybrid_indexed=false duration_ms=37682
    benchmark_longmemeval_test.go:815: [42/64] e01b8e2f — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:00:03 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:00:03 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:00:03 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:00:03 INFO Verification layer initialized
2026/07/28 04:00:03 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:00:03 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:00:12 INFO Observer completed successfully session_id=sh-bc8a6e93 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=78 reflector_triggered=false hybrid_indexed=false duration_ms=9262
    benchmark_longmemeval_test.go:815: [43/64] bc8a6e93 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:00:17 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:00:17 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:00:17 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:00:17 INFO Verification layer initialized
2026/07/28 04:00:17 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:00:17 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:00:54 INFO Observer completed successfully session_id=sh-ccb36322 resource_id=user_999 scope=resource new_messages=10 skipped_trivial=0 total_tokens=648 reflector_triggered=false hybrid_indexed=false duration_ms=37020
    benchmark_longmemeval_test.go:815: [44/64] ccb36322 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:01:02 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:01:02 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:01:02 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:01:02 INFO Verification layer initialized
2026/07/28 04:01:02 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:01:02 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:02:13 INFO Observer completed successfully session_id=sh-001be529 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=134 reflector_triggered=false hybrid_indexed=false duration_ms=70473
    benchmark_longmemeval_test.go:815: [45/64] 001be529 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:02:15 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:02:15 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:02:15 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:02:15 INFO Verification layer initialized
2026/07/28 04:02:15 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:02:15 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:02:28 INFO Observer completed successfully session_id=sh-b320f3f8 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=320 reflector_triggered=false hybrid_indexed=false duration_ms=13368
    benchmark_longmemeval_test.go:815: [46/64] b320f3f8 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:02:34 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:02:34 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:02:34 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:02:34 INFO Verification layer initialized
2026/07/28 04:02:34 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:02:34 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:03:16 INFO Observer completed successfully session_id=sh-19b5f2b3 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=217 reflector_triggered=false hybrid_indexed=false duration_ms=42881
    benchmark_longmemeval_test.go:815: [47/64] 19b5f2b3 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:03:19 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:03:19 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:03:19 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:03:19 INFO Verification layer initialized
2026/07/28 04:03:19 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:03:19 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:03:26 INFO Observer completed successfully session_id=sh-4fd1909e resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=265 reflector_triggered=false hybrid_indexed=false duration_ms=7045
    benchmark_longmemeval_test.go:815: [48/64] 4fd1909e — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:03:28 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:03:28 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:03:28 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:03:28 INFO Verification layer initialized
2026/07/28 04:03:28 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:03:28 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:03:59 INFO Observer completed successfully session_id=sh-545bd2b5 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=143 reflector_triggered=false hybrid_indexed=false duration_ms=31723
    benchmark_longmemeval_test.go:815: [49/64] 545bd2b5 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:04:01 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:04:01 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:04:01 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:04:01 INFO Verification layer initialized
2026/07/28 04:04:01 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:04:01 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:04:05 INFO Observer completed successfully session_id=sh-8a137a7f resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=202 reflector_triggered=false hybrid_indexed=false duration_ms=3649
    benchmark_longmemeval_test.go:815: [50/64] 8a137a7f — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:04:07 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:04:07 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:04:07 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:04:07 INFO Verification layer initialized
2026/07/28 04:04:07 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:04:07 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:04:12 INFO Observer completed successfully session_id=sh-76d63226 resource_id=user_999 scope=resource new_messages=10 skipped_trivial=0 total_tokens=192 reflector_triggered=false hybrid_indexed=false duration_ms=5186
    benchmark_longmemeval_test.go:815: [51/64] 76d63226 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:04:25 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:04:25 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:04:25 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:04:25 INFO Verification layer initialized
2026/07/28 04:04:25 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:04:25 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:06:11 INFO Observer completed successfully session_id=sh-86f00804 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=160 reflector_triggered=false hybrid_indexed=false duration_ms=105829
    benchmark_longmemeval_test.go:815: [52/64] 86f00804 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:06:13 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:06:13 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:06:13 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:06:13 INFO Verification layer initialized
2026/07/28 04:06:13 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:06:13 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:06:55 INFO Observer completed successfully session_id=sh-8e9d538c resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=206 reflector_triggered=false hybrid_indexed=false duration_ms=42128
    benchmark_longmemeval_test.go:815: [53/64] 8e9d538c — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:06:57 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:06:57 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:06:57 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:06:57 INFO Verification layer initialized
2026/07/28 04:06:57 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:06:57 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:07:00 INFO Observer completed successfully session_id=sh-311778f1 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=9 reflector_triggered=false hybrid_indexed=false duration_ms=2615
    benchmark_longmemeval_test.go:815: [54/64] 311778f1 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:07:01 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:07:01 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:07:01 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:07:01 INFO Verification layer initialized
2026/07/28 04:07:01 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:07:01 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:07:03 INFO Observer completed successfully session_id=sh-c19f7a0b resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=4 reflector_triggered=false hybrid_indexed=false duration_ms=1655
    benchmark_longmemeval_test.go:815: [55/64] c19f7a0b — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:07:04 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:07:04 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:07:04 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:07:04 INFO Verification layer initialized
2026/07/28 04:07:04 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:07:04 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:07:54 INFO Observer completed successfully session_id=sh-4100d0a0 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=870 reflector_triggered=false hybrid_indexed=false duration_ms=49422
    benchmark_longmemeval_test.go:815: [56/64] 4100d0a0 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:07:56 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:07:56 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:07:56 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:07:56 INFO Verification layer initialized
2026/07/28 04:07:56 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:07:56 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:08:19 INFO Observer completed successfully session_id=sh-29f2956b resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=203 reflector_triggered=false hybrid_indexed=false duration_ms=22599
    benchmark_longmemeval_test.go:815: [57/64] 29f2956b — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:08:21 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:08:21 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:08:21 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:08:21 INFO Verification layer initialized
2026/07/28 04:08:21 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:08:21 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:09:39 INFO Observer completed successfully session_id=sh-1faac195 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=505 reflector_triggered=false hybrid_indexed=false duration_ms=77897
    benchmark_longmemeval_test.go:815: [58/64] 1faac195 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:09:41 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:09:41 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:09:41 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:09:41 INFO Verification layer initialized
2026/07/28 04:09:41 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:09:41 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:09:44 INFO Observer completed successfully session_id=sh-faba32e5 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=357 reflector_triggered=false hybrid_indexed=false duration_ms=2944
    benchmark_longmemeval_test.go:815: [59/64] faba32e5 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:10:02 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:10:02 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:10:02 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:10:02 INFO Verification layer initialized
2026/07/28 04:10:02 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:10:02 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:10:08 INFO Observer completed successfully session_id=sh-f4f1d8a4 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=225 reflector_triggered=false hybrid_indexed=false duration_ms=5445
    benchmark_longmemeval_test.go:815: [60/64] f4f1d8a4 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:10:09 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:10:09 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:10:09 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:10:09 INFO Verification layer initialized
2026/07/28 04:10:09 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:10:09 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
panic: test timed out after 30m0s
	running tests:
		TestBenchmarkSingleHop (30m0s)

goroutine 958 [running]:
testing.(*M).startAlarm.func1()
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/testing/testing.go:2802 +0x34b
created by time.goFunc
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/time/sleep.go:215 +0x2d

goroutine 1 [chan receive, 29 minutes]:
testing.(*T).Run(0x213c606d6b48, {0x1acac18?, 0x213c6085fb30?}, 0x1c715b8)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/testing/testing.go:2109 +0x4e5
testing.runTests.func1(0x213c606d6b48)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/testing/testing.go:2585 +0x37
testing.tRunner(0x213c606d6b48, 0x213c6085fc58)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/testing/testing.go:2036 +0xea
testing.runTests({0x1ad4441, 0x19}, {0x1b07dc6, 0x34}, 0x213c604a2bd0, {0x3470060, 0xf8, 0xf8}, {0xc2920caf37d1b235, 0x1a3189ca767, ...})
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/testing/testing.go:2583 +0x505
testing.(*M).Run(0x213c6088cd20)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/testing/testing.go:2443 +0x6ac
main.main()
	_testmain.go:540 +0x9b

goroutine 8 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*SimulationSessionStore).cleanupLoop(0x213c60888990)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/simulation.go:163 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewSimulationSessionStore in goroutine 1
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/simulation.go:117 +0x9a

goroutine 9 [sync.Cond.Wait]:
sync.runtime_notifyListWait(0x213c606293c8, 0x3f)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/runtime/sema.go:617 +0x1b3
sync.(*Cond).Wait(0x213c60bfa640?)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/sync/cond.go:71 +0x73
net/http.(*http2pipe).Read(0x213c606293b0, {0x213c7bee6000, 0x1000, 0x1000})
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/net/http/h2_bundle.go:3928 +0xdb
net/http.http2transportResponseBody.Read({0x5fa205?}, {0x213c7bee6000?, 0x0?, 0x213c60629380?})
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/net/http/h2_bundle.go:9852 +0x59
bufio.(*Reader).fill(0x213c607130e0)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/bufio/bufio.go:113 +0x103
bufio.(*Reader).ReadByte(0x213c607130e0)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/bufio/bufio.go:273 +0x27
compress/flate.(*decompressor).moreBits(0x213c606d1308)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/compress/flate/inflate.go:697 +0x24
compress/flate.(*decompressor).nextBlock(0x213c606d1308)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/compress/flate/inflate.go:304 +0x28
compress/flate.(*decompressor).Read(0x213c606d1308, {0x213c7aa454b5, 0x34b, 0x213c835041b8?})
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/compress/flate/inflate.go:348 +0x5b
compress/gzip.(*Reader).Read(0x213c7d722848, {0x213c7aa454b5, 0x34b, 0x34b})
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/compress/gzip/gunzip.go:252 +0x98
net/http.(*http2gzipReader).Read(0x213c7c90eae0, {0x213c7aa454b5, 0x34b, 0x34b})
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/net/http/h2_bundle.go:10518 +0xb0
net/http.(*cancelTimerBody).Read(0x213c86033920, {0x213c7aa454b5?, 0x4276ef?, 0x726faf5d57d0?})
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/net/http/client.go:986 +0x2a
encoding/json.(*Decoder).refill(0x213c8f760000)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/encoding/json/stream.go:167 +0x188
encoding/json.(*Decoder).readValue(0x213c8f760000)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/encoding/json/stream.go:142 +0x85
encoding/json.(*Decoder).Decode(0x213c8f760000, {0x167ece0, 0x213c6070f200})
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/encoding/json/stream.go:65 +0x75
github.com/revrost/go-openrouter.decodeResponse({0x726f43ec1380, 0x213c86033920}, {0x167ece0, 0x213c6070f200})
	/home/chaschel/go/pkg/mod/github.com/revrost/go-openrouter@v1.1.5/client.go:78 +0xe5
github.com/revrost/go-openrouter.(*Client).sendRequest(0x213c60d63b90, 0x213c7d1a57c0, {0x167ece0, 0x213c6070f200})
	/home/chaschel/go/pkg/mod/github.com/revrost/go-openrouter@v1.1.5/client.go:57 +0x2c5
github.com/revrost/go-openrouter.(*Client).CreateChatCompletion(_, {_, _}, {{0x1aa2c41, 0xf}, {0x0, 0x0, 0x0}, 0x0, {0x213c60bce000, ...}, ...})
	/home/chaschel/go/pkg/mod/github.com/revrost/go-openrouter@v1.1.5/chat.go:640 +0x198
github.com/usememos/memos/server/router/api/v1/agent.(*Service).callObserverLLM(0x1761280?, {0x1c99f98, 0x3499280}, 0x213c60d63b90, {0x1aa2c41, 0xf}, {0x0?, 0x213c85f0b8a8?}, {0x213c60c20000, 0x3591})
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer.go:313 +0x3c5
github.com/usememos/memos/server/router/api/v1/agent.(*Service).RunObserver.func1()
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer.go:164 +0xb3
github.com/usememos/memos/server/router/api/v1/agent.withRetry({0x213c604d4013?, 0x49?}, 0x3, 0x3e8, 0x213c83505768)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer.go:432 +0x77
github.com/usememos/memos/server/router/api/v1/agent.(*Service).RunObserver(0x213c60671740, {0x1c99f98, 0x3499280}, 0x1, {0x213c8783c7d0, 0xb})
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer.go:163 +0xe2b
github.com/usememos/memos/server/router/api/v1/agent.runBenchmarkQuestion(_, {_, _}, {_, _}, {{0x213c60749030, 0x8}, {0x213c60749040, 0xa}, {0x1793dc0, ...}, ...}, ...)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/benchmark_longmemeval_test.go:728 +0x76c
github.com/usememos/memos/server/router/api/v1/agent.runPerTypeBenchmark(0x213c606d6d88, {0x1a4490f, 0xa}, {0x1944284, 0x2}, 0x1c71d48)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/benchmark_longmemeval_test.go:802 +0x6ab
github.com/usememos/memos/server/router/api/v1/agent.TestBenchmarkSingleHop(0x213c606d6d88?)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/benchmark_longmemeval_test.go:815 +0x32
testing.tRunner(0x213c606d6d88, 0x1c715b8)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/testing/testing.go:2036 +0xea
created by testing.(*T).Run in goroutine 1
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/testing/testing.go:2101 +0x4c5

goroutine 433 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60bfbc80)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 949 [select]:
github.com/usememos/memos/store/cache.(*Cache).cleanupLoop(0x213c60d639e0)
	/home/chaschel/Documents/go/bchat/store/cache/cache.go:225 +0xc8
created by github.com/usememos/memos/store/cache.New in goroutine 9
	/home/chaschel/Documents/go/bchat/store/cache/cache.go:85 +0xff

goroutine 950 [select]:
github.com/usememos/memos/store/cache.(*Cache).cleanupLoop(0x213c60d63a70)
	/home/chaschel/Documents/go/bchat/store/cache/cache.go:225 +0xc8
created by github.com/usememos/memos/store/cache.New in goroutine 9
	/home/chaschel/Documents/go/bchat/store/cache/cache.go:85 +0xff

goroutine 80 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60c000c0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 296 [chan receive, 25 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60c007e0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 41 [chan receive, 5 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c816de4c0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 42 [chan receive, 29 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60712d80)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 43 [select, 5 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60712d80)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 268 [select, 5 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c85e72780)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 78 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60c07740)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 151 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c816df500)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 108 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c86eb81c0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 52 [IO wait]:
internal/poll.runtime_pollWait(0x726faf47da00, 0x72)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/runtime/netpoll.go:351 +0x85
internal/poll.(*pollDesc).wait(0x213c608a5500?, 0x213c60d29000?, 0x0)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/internal/poll/fd_poll_runtime.go:84 +0x27
internal/poll.(*pollDesc).waitRead(...)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/internal/poll/fd_poll_runtime.go:89
internal/poll.(*FD).Read(0x213c608a5500, {0x213c60d29000, 0x5000, 0x5000})
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/internal/poll/fd_unix.go:165 +0x2ae
net.(*netFD).Read(0x213c608a5500, {0x213c60d29000?, 0x213c60d29000?, 0x5?})
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/net/fd_posix.go:68 +0x25
net.(*conn).Read(0x213c94700010, {0x213c60d29000?, 0x726f428d21c8?, 0x726faf5daa98?})
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/net/net.go:196 +0x45
crypto/tls.(*atLeastReader).Read(0x213c86746f48, {0x213c60d29000?, 0x213c605850e0?, 0x213c81397a48?})
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/crypto/tls/conn.go:815 +0x3b
bytes.(*Buffer).ReadFrom(0x213c6073b428, {0x1c852e0, 0x213c86746f48})
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/bytes/buffer.go:229 +0x98
crypto/tls.(*Conn).readFromUntil(0x213c6073b188, {0x1c83f20, 0x213c94700010}, 0x213c81397c08?)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/crypto/tls/conn.go:837 +0xde
crypto/tls.(*Conn).readRecordOrCCS(0x213c6073b188, 0x0)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/crypto/tls/conn.go:626 +0x3db
crypto/tls.(*Conn).readRecord(...)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/crypto/tls/conn.go:588
crypto/tls.(*Conn).Read(0x213c6073b188, {0x213c81af1000, 0x1000, 0x213c81397cb0?})
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/crypto/tls/conn.go:1393 +0x145
bufio.(*Reader).Read(0x213c85e72de0, {0x213c85e652a4, 0x9, 0x8?})
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/bufio/bufio.go:245 +0x197
io.ReadAtLeast({0x1c82fc0, 0x213c85e72de0}, {0x213c85e652a4, 0x9, 0x9}, 0x9)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/io/io.go:335 +0x8e
io.ReadFull(...)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/io/io.go:354
net/http.http2readFrameHeader({0x213c85e652a4, 0x9, 0x213c00000169?}, {0x1c82fc0?, 0x213c85e72de0?})
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/net/http/h2_bundle.go:1805 +0x65
net/http.(*http2Framer).ReadFrameHeader(0x213c85e65260)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/net/http/h2_bundle.go:2071 +0x6b
net/http.(*http2Framer).ReadFrame(0x213c85e65260)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/net/http/h2_bundle.go:2130 +0x18
net/http.(*http2clientConnReadLoop).run(0x213c81397fa8)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/net/http/h2_bundle.go:9550 +0xca
net/http.(*http2ClientConn).readLoop(0x213c85e3a8c0)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/net/http/h2_bundle.go:9419 +0x52
created by net/http.(*http2Transport).newClientConn in goroutine 51
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/net/http/h2_bundle.go:8171 +0xda5

goroutine 62 [chan receive, 5 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c865c2300)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 63 [chan receive, 29 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c85e730e0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 64 [select, 5 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c85e730e0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 325 [select, 5 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60c00ae0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 109 [chan receive, 29 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60713260)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 110 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60713260)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 153 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c85e72540)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 93 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c85e72240)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 152 [chan receive, 27 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c85e72540)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 92 [chan receive, 27 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c85e72240)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 122 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c877b9980)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 123 [chan receive, 29 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60713620)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 124 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60713620)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 91 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c609c7840)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 675 [chan receive, 11 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c85e72360)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 139 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c81398f40)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 140 [chan receive, 27 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c604d0540)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 141 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c604d0540)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 250 [chan receive, 25 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60c003c0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 297 [select, 5 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60c007e0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 79 [chan receive, 27 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60c000c0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 210 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60e0aa80)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 324 [chan receive, 25 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60c00ae0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 323 [chan receive, 5 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60daeb00)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 670 [chan receive, 11 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60c00a80)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 584 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60bfaa40)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 165 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c8746ef00)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 166 [chan receive, 27 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c85e72900)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 167 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c85e72900)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 339 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c877b9a80)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 317 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60dde900)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 211 [chan receive, 27 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c604d0960)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 212 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c604d0960)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 251 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60c003c0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 671 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60c00a80)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 669 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60c06240)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 206 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60dde5c0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 207 [chan receive, 27 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c85e72d80)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 208 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c85e72d80)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 266 [chan receive, 5 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c86eb95c0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 249 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c862b38c0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 236 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60e0b880)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 237 [chan receive, 25 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c85e72300)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 238 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c85e72300)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 295 [chan receive, 5 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60dc33c0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 267 [chan receive, 25 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c85e72780)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 188 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c6072ce40)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 189 [chan receive, 25 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c604d04e0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 190 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c604d04e0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 319 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60c00360)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 318 [chan receive, 23 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60c00360)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 280 [chan receive, 5 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c8746f000)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 281 [chan receive, 25 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c85e72c00)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 282 [select, 5 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c85e72c00)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 340 [chan receive, 23 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c85e721e0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 572 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60c00420)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 525 [chan receive, 17 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60c00600)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 771 [chan receive, 7 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60712960)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 422 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c604d0720)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 341 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c85e721e0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 466 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c609c7540)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 570 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c609c7500)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 410 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60e0b280)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 371 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c8746fa80)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 372 [chan receive, 21 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c85e723c0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 373 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c85e723c0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 421 [chan receive, 21 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c604d0720)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 420 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c86eb83c0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 398 [chan receive, 5 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c865c3140)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 225 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60c00300)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 383 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60dafe80)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 384 [chan receive, 21 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c85e727e0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 385 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c85e727e0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 449 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c8746f340)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 411 [chan receive, 21 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c607127e0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 412 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c607127e0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 400 [select, 5 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c604d0480)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 399 [chan receive, 19 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c604d0480)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 434 [chan receive, 21 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c604d0c00)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 435 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c604d0c00)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 498 [chan receive, 17 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c604d0660)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 571 [chan receive, 15 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60c00420)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 607 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60bfa540)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 461 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60ddf680)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 462 [chan receive, 19 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c604d0a80)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 463 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c604d0a80)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 499 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c604d0660)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 526 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60c00600)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 467 [chan receive, 19 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c85e724e0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 468 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c85e724e0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 524 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c86eb9d40)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 925 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60712b40)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 534 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60560280)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 490 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60bfb940)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 491 [chan receive, 17 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60712840)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 492 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60712840)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 535 [chan receive, 17 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60712ba0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 637 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c862b3080)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 509 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c816df640)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 536 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60712ba0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 648 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c816dfb00)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 557 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60c00480)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 608 [chan receive, 13 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c85e725a0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 510 [chan receive, 17 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c604d0ba0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 511 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c604d0ba0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 650 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c604d05a0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 649 [chan receive, 11 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c604d05a0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 585 [chan receive, 15 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60c00900)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 586 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60c00900)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 590 [chan receive, 15 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60c00b40)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 589 [chan receive, 5 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c8746e900)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 591 [select, 5 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60c00b40)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 224 [chan receive, 11 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60c00300)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 556 [chan receive, 13 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60c00480)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 555 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60e0b500)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 619 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c86eb9ec0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 620 [chan receive, 13 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c604d0600)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 621 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c604d0600)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 676 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c85e72360)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 609 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c85e725a0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 674 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c877b8700)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 223 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c86eb9cc0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 688 [select, 5 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c85e72480)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 686 [chan receive, 5 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60bfb480)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 687 [chan receive, 9 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c85e72480)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 772 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60712960)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 901 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c85e759c0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 638 [chan receive, 11 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60712a20)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 639 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60712a20)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 880 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60712ea0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 770 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c862b3c00)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 731 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60daf540)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 732 [chan receive, 9 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c85e72a20)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 733 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c85e72a20)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 891 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c85e72420)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 879 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60712ea0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 744 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c6087d600)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 745 [chan receive, 9 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c85e72ea0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 746 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c85e72ea0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 892 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c85e72420)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 786 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c8746f580)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 763 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60dde340)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 764 [chan receive, 7 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c85e72660)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 765 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c85e72660)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 769 [chan receive, 7 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c85e72b40)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 768 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60bc1ec0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 787 [chan receive, 7 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60712f00)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 788 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60712f00)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 752 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c865c2280)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 878 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60dae1c0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 811 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c816de180)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 812 [chan receive, 7 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60c004e0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 813 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60c004e0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 951 [select]:
github.com/usememos/memos/store/cache.(*Cache).cleanupLoop(0x213c60d63b00)
	/home/chaschel/Documents/go/bchat/store/cache/cache.go:225 +0xc8
created by github.com/usememos/memos/store/cache.New in goroutine 9
	/home/chaschel/Documents/go/bchat/store/cache/cache.go:85 +0xff

goroutine 818 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c85e72b40)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 866 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60712720)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 902 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c604d0de0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 890 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60e0a100)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 481 [chan receive, 5 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c81398480)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 834 [chan receive, 5 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60712ae0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 835 [select, 5 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60712ae0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 924 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60712b40)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 753 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60712720)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 856 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c877b8400)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 857 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c604d06c0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 858 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c604d06c0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 906 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60bc1740)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 903 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c604d0de0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 907 [chan receive, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c604d10e0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 923 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c86b1dcc0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 908 [select, 3 minutes]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c604d10e0)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 948 [select]:
database/sql.(*DB).connectionOpener(0x213c85f29790, {0x1c9a200, 0x213c7d18e640})
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/database/sql/sql.go:1261 +0x89
created by database/sql.OpenDB in goroutine 9
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/database/sql/sql.go:841 +0x130

goroutine 953 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*MemorySessionStore).cleanupLoop(0x213c60bfa500)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1477 +0x66
created by github.com/usememos/memos/server/router/api/v1/agent.NewMemorySessionStore in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go:1384 +0xb6

goroutine 954 [chan receive]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).bufferWorker(0x213c60713020)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:58 +0x65
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:51 +0x169

goroutine 955 [select]:
github.com/usememos/memos/server/router/api/v1/agent.(*ObserverBuffer).cleanupWorker(0x213c60713020)
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:72 +0x7c
created by github.com/usememos/memos/server/router/api/v1/agent.NewObserverBuffer in goroutine 9
	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/observer_buffer.go:52 +0x1a5

goroutine 957 [select]:
net/http.(*http2clientStream).writeRequest(0x213c60629380, 0x213c7d1a5b80, 0x0)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/net/http/h2_bundle.go:8857 +0xc68
net/http.(*http2clientStream).doRequest(0x213c60629380, 0x6f635f736574616c?, 0x6574204e4f206564?)
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/net/http/h2_bundle.go:8718 +0x18
created by net/http.(*http2ClientConn).roundTrip in goroutine 9
	/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/net/http/h2_bundle.go:8624 +0x470
FAIL	github.com/usememos/memos/server/router/api/v1/agent	1800.066s
FAIL

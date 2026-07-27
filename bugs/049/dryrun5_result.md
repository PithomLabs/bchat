chaschel@linux:~/Documents/go/bchat$ cd /home/chaschel/Documents/go/bchat && BENCHMARK_LONGMEMEVAL=true OPENROUTER_API_KEY=sk-or-v1-... BENCHMARK_FRESH=true go test ./server/router/api/v1/agent/ -run TestBenchmarkPreference -v -count=1 -timeout=45m
=== RUN   TestBenchmarkPreference
    benchmark_longmemeval_test.go:821: Cleared existing JSONL files (BENCHMARK_FRESH=true)
    benchmark_longmemeval_test.go:821: Loaded 500 questions (178 testable), 14065 cache entries
    benchmark_longmemeval_test.go:821: implicit_preference_v2: 30 questions (non-abs)
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:06:31 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:06:31 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:06:31 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:06:31 INFO Verification layer initialized
2026/07/28 05:06:31 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:06:31 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:07:09 ERROR EstimateTokens using len/4 fallback — globalTokenizer not initialized contentLength=1665
2026/07/28 05:07:09 INFO Observer completed successfully session_id=pref-8a2466db resource_id=user_999 scope=resource new_messages=18 skipped_trivial=0 total_tokens=416 reflector_triggered=false hybrid_indexed=false duration_ms=37564
    benchmark_longmemeval_test.go:821: [1/30] 8a2466db — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:07:12 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:07:12 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:07:12 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:07:12 INFO Verification layer initialized
2026/07/28 05:07:12 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:07:12 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:08:14 INFO Observer completed successfully session_id=pref-06878be2 resource_id=user_999 scope=resource new_messages=16 skipped_trivial=0 total_tokens=352 reflector_triggered=false hybrid_indexed=false duration_ms=61689
    benchmark_longmemeval_test.go:821: [2/30] 06878be2 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:08:17 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:08:17 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:08:17 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:08:17 INFO Verification layer initialized
2026/07/28 05:08:17 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:08:17 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:08:28 INFO Observer completed successfully session_id=pref-75832dbd resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=196 reflector_triggered=false hybrid_indexed=false duration_ms=10757
    benchmark_longmemeval_test.go:821: [3/30] 75832dbd — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:08:44 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:08:44 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:08:44 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:08:44 INFO Verification layer initialized
2026/07/28 05:08:44 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:08:44 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:09:25 INFO Observer completed successfully session_id=pref-0edc2aef resource_id=user_999 scope=resource new_messages=20 skipped_trivial=0 total_tokens=139 reflector_triggered=false hybrid_indexed=false duration_ms=41508
    benchmark_longmemeval_test.go:821: [4/30] 0edc2aef — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:09:33 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:09:33 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:09:33 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:09:33 INFO Verification layer initialized
2026/07/28 05:09:33 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:09:33 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:09:36 INFO Observer completed successfully session_id=pref-35a27287 resource_id=user_999 scope=resource new_messages=14 skipped_trivial=0 total_tokens=338 reflector_triggered=false hybrid_indexed=false duration_ms=3002
    benchmark_longmemeval_test.go:821: [5/30] 35a27287 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:09:41 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:09:41 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:09:41 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:09:41 INFO Verification layer initialized
2026/07/28 05:09:41 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:09:41 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:10:01 INFO Observer completed successfully session_id=pref-32260d93 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=601 reflector_triggered=false hybrid_indexed=false duration_ms=20511
    benchmark_longmemeval_test.go:821: [6/30] 32260d93 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:10:05 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:10:05 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:10:05 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:10:05 INFO Verification layer initialized
2026/07/28 05:10:05 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:10:05 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:10:20 INFO Observer completed successfully session_id=pref-195a1a1b resource_id=user_999 scope=resource new_messages=16 skipped_trivial=0 total_tokens=249 reflector_triggered=false hybrid_indexed=false duration_ms=14182
    benchmark_longmemeval_test.go:821: [7/30] 195a1a1b — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:10:41 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:10:41 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:10:41 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:10:41 INFO Verification layer initialized
2026/07/28 05:10:41 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:10:41 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:11:25 INFO Observer completed successfully session_id=pref-afdc33df resource_id=user_999 scope=resource new_messages=14 skipped_trivial=0 total_tokens=696 reflector_triggered=false hybrid_indexed=false duration_ms=43598
    benchmark_longmemeval_test.go:821: [8/30] afdc33df — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:11:27 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:11:27 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:11:27 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:11:27 INFO Verification layer initialized
2026/07/28 05:11:27 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:11:27 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:11:37 INFO Observer completed successfully session_id=pref-caf03d32 resource_id=user_999 scope=resource new_messages=18 skipped_trivial=0 total_tokens=230 reflector_triggered=false hybrid_indexed=false duration_ms=9998
    benchmark_longmemeval_test.go:821: [9/30] caf03d32 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:11:42 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:11:42 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:11:42 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:11:42 INFO Verification layer initialized
2026/07/28 05:11:42 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:11:42 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:11:52 INFO Observer completed successfully session_id=pref-54026fce resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=174 reflector_triggered=false hybrid_indexed=false duration_ms=9915
    benchmark_longmemeval_test.go:821: [10/30] 54026fce — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:11:54 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:11:54 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:11:54 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:11:54 INFO Verification layer initialized
2026/07/28 05:11:54 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:11:54 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:12:11 INFO Observer completed successfully session_id=pref-06f04340 resource_id=user_999 scope=resource new_messages=14 skipped_trivial=0 total_tokens=513 reflector_triggered=false hybrid_indexed=false duration_ms=16782
    benchmark_longmemeval_test.go:821: [11/30] 06f04340 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:12:13 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:12:13 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:12:13 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:12:13 INFO Verification layer initialized
2026/07/28 05:12:13 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:12:13 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:12:16 INFO Observer completed successfully session_id=pref-6b7dfb22 resource_id=user_999 scope=resource new_messages=16 skipped_trivial=0 total_tokens=208 reflector_triggered=false hybrid_indexed=false duration_ms=2929
    benchmark_longmemeval_test.go:821: [12/30] 6b7dfb22 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:13:09 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:13:09 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:13:09 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:13:09 INFO Verification layer initialized
2026/07/28 05:13:09 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:13:09 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:13:21 INFO Observer completed successfully session_id=pref-1a1907b4 resource_id=user_999 scope=resource new_messages=16 skipped_trivial=0 total_tokens=150 reflector_triggered=false hybrid_indexed=false duration_ms=11848
    benchmark_longmemeval_test.go:821: [13/30] 1a1907b4 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:13:31 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:13:31 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:13:31 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:13:31 INFO Verification layer initialized
2026/07/28 05:13:31 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:13:31 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:13:42 INFO Observer completed successfully session_id=pref-09d032c9 resource_id=user_999 scope=resource new_messages=20 skipped_trivial=0 total_tokens=272 reflector_triggered=false hybrid_indexed=false duration_ms=10661
    benchmark_longmemeval_test.go:821: [14/30] 09d032c9 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:13:44 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:13:44 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:13:44 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:13:44 INFO Verification layer initialized
2026/07/28 05:13:44 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:13:44 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:13:45 INFO Observer completed successfully session_id=pref-38146c39 resource_id=user_999 scope=resource new_messages=16 skipped_trivial=0 total_tokens=9 reflector_triggered=false hybrid_indexed=false duration_ms=1322
    benchmark_longmemeval_test.go:821: [15/30] 38146c39 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:13:47 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:13:47 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:13:47 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:13:47 INFO Verification layer initialized
2026/07/28 05:13:47 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:13:47 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:13:55 INFO Observer completed successfully session_id=pref-d24813b1 resource_id=user_999 scope=resource new_messages=14 skipped_trivial=0 total_tokens=211 reflector_triggered=false hybrid_indexed=false duration_ms=7731
    benchmark_longmemeval_test.go:821: [16/30] d24813b1 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:13:57 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:13:57 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:13:57 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:13:57 INFO Verification layer initialized
2026/07/28 05:13:57 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:13:57 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:14:08 INFO Observer completed successfully session_id=pref-57f827a0 resource_id=user_999 scope=resource new_messages=14 skipped_trivial=0 total_tokens=100 reflector_triggered=false hybrid_indexed=false duration_ms=11075
    benchmark_longmemeval_test.go:821: [17/30] 57f827a0 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:14:10 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:14:10 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:14:10 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:14:10 INFO Verification layer initialized
2026/07/28 05:14:10 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:14:10 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:14:18 INFO Observer completed successfully session_id=pref-95228167 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=441 reflector_triggered=false hybrid_indexed=false duration_ms=7354
    benchmark_longmemeval_test.go:821: [18/30] 95228167 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:14:52 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:14:52 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:14:52 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:14:52 INFO Verification layer initialized
2026/07/28 05:14:52 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:14:52 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:14:55 INFO Observer completed successfully session_id=pref-505af2f5 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=415 reflector_triggered=false hybrid_indexed=false duration_ms=3099
    benchmark_longmemeval_test.go:821: [19/30] 505af2f5 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:14:57 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:14:57 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:14:57 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:14:57 INFO Verification layer initialized
2026/07/28 05:14:57 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:14:57 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:15:00 INFO Observer completed successfully session_id=pref-75f70248 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=4 reflector_triggered=false hybrid_indexed=false duration_ms=3492
    benchmark_longmemeval_test.go:821: [20/30] 75f70248 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:15:02 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:15:02 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:15:02 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:15:02 INFO Verification layer initialized
2026/07/28 05:15:02 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:15:02 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:15:07 INFO Observer completed successfully session_id=pref-d6233ab6 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=186 reflector_triggered=false hybrid_indexed=false duration_ms=5246
    benchmark_longmemeval_test.go:821: [21/30] d6233ab6 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:15:09 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:15:09 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:15:09 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:15:09 INFO Verification layer initialized
2026/07/28 05:15:09 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:15:09 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:15:34 INFO Observer completed successfully session_id=pref-1da05512 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=456 reflector_triggered=false hybrid_indexed=false duration_ms=25358
    benchmark_longmemeval_test.go:821: [22/30] 1da05512 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:15:36 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:15:36 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:15:36 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:15:36 INFO Verification layer initialized
2026/07/28 05:15:36 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:15:36 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:15:44 INFO Observer completed successfully session_id=pref-fca70973 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=244 reflector_triggered=false hybrid_indexed=false duration_ms=7310
    benchmark_longmemeval_test.go:821: [23/30] fca70973 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:15:53 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:15:53 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:15:53 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:15:53 INFO Verification layer initialized
2026/07/28 05:15:53 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:15:53 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:16:03 INFO Observer completed successfully session_id=pref-b6025781 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=185 reflector_triggered=false hybrid_indexed=false duration_ms=9512
    benchmark_longmemeval_test.go:821: [24/30] b6025781 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:16:05 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:16:05 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:16:05 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:16:05 INFO Verification layer initialized
2026/07/28 05:16:05 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:16:05 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:16:27 INFO Observer completed successfully session_id=pref-a89d7624 resource_id=user_999 scope=resource new_messages=14 skipped_trivial=0 total_tokens=413 reflector_triggered=false hybrid_indexed=false duration_ms=22252
    benchmark_longmemeval_test.go:821: [25/30] a89d7624 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:16:31 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:16:31 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:16:31 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:16:31 INFO Verification layer initialized
2026/07/28 05:16:31 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:16:31 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:16:37 INFO Observer completed successfully session_id=pref-b0479f84 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=129 reflector_triggered=false hybrid_indexed=false duration_ms=5075
    benchmark_longmemeval_test.go:821: [26/30] b0479f84 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:16:39 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:16:39 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:16:39 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:16:39 INFO Verification layer initialized
2026/07/28 05:16:39 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:16:39 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:16:54 INFO Observer completed successfully session_id=pref-1d4e3b97 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=178 reflector_triggered=false hybrid_indexed=false duration_ms=15543
    benchmark_longmemeval_test.go:821: [27/30] 1d4e3b97 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:17:04 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:17:04 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:17:04 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:17:04 INFO Verification layer initialized
2026/07/28 05:17:04 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:17:04 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:17:34 INFO Observer completed successfully session_id=pref-07b6f563 resource_id=user_999 scope=resource new_messages=14 skipped_trivial=0 total_tokens=232 reflector_triggered=false hybrid_indexed=false duration_ms=30092
    benchmark_longmemeval_test.go:821: [28/30] 07b6f563 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:17:40 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:17:40 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:17:40 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:17:40 INFO Verification layer initialized
2026/07/28 05:17:40 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:17:40 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:18:11 INFO Observer completed successfully session_id=pref-1c0ddc50 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=870 reflector_triggered=false hybrid_indexed=false duration_ms=31012
    benchmark_longmemeval_test.go:821: [29/30] 1c0ddc50 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 05:18:13 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 05:18:13 INFO Column already exists, skipping table=tickets column=type
2026/07/28 05:18:13 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 05:18:13 INFO Verification layer initialized
2026/07/28 05:18:13 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 05:18:13 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 05:18:32 INFO Observer completed successfully session_id=pref-0a34ad58 resource_id=user_999 scope=resource new_messages=20 skipped_trivial=0 total_tokens=542 reflector_triggered=false hybrid_indexed=false duration_ms=19765
    benchmark_longmemeval_test.go:821: [30/30] 0a34ad58 — yes
    benchmark_longmemeval_test.go:821: === implicit_preference_v2 Summary: 15 passed, 15 failed, 0 skipped ===
--- PASS: TestBenchmarkPreference (733.38s)
PASS
ok  	github.com/usememos/memos/server/router/api/v1/agent	733.431s
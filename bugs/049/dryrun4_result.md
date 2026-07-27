chaschel@linux:~/Documents/go/bchat$ cd /home/chaschel/Documents/go/bchat && BENCHMARK_LONGMEMEVAL=true OPENROUTER_API_KEY=sk-or-v1-... go test ./server/router/api/v1/agent/ -run TestBenchmarkPreference -v -count=1 -timeout=45m
=== RUN   TestBenchmarkPreference
    benchmark_longmemeval_test.go:821: Loaded 500 questions (178 testable), 14065 cache entries
    benchmark_longmemeval_test.go:821: implicit_preference_v2: 30 questions (non-abs)
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:36:45 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:36:45 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:36:45 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:36:45 INFO Verification layer initialized
2026/07/28 04:36:45 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:36:45 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:37:01 ERROR EstimateTokens using len/4 fallback — globalTokenizer not initialized contentLength=787
2026/07/28 04:37:01 INFO Observer completed successfully session_id=pref-8a2466db resource_id=user_999 scope=resource new_messages=18 skipped_trivial=0 total_tokens=196 reflector_triggered=false hybrid_indexed=false duration_ms=16360
    benchmark_longmemeval_test.go:821: [1/30] 8a2466db — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:37:03 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:37:03 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:37:03 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:37:03 INFO Verification layer initialized
2026/07/28 04:37:03 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:37:03 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:37:05 INFO Observer completed successfully session_id=pref-06878be2 resource_id=user_999 scope=resource new_messages=16 skipped_trivial=0 total_tokens=9 reflector_triggered=false hybrid_indexed=false duration_ms=2028
    benchmark_longmemeval_test.go:821: [2/30] 06878be2 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:37:12 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:37:12 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:37:12 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:37:12 INFO Verification layer initialized
2026/07/28 04:37:12 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:37:12 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:37:21 INFO Observer completed successfully session_id=pref-75832dbd resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=96 reflector_triggered=false hybrid_indexed=false duration_ms=8419
    benchmark_longmemeval_test.go:821: [3/30] 75832dbd — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:37:33 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:37:33 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:37:33 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:37:33 INFO Verification layer initialized
2026/07/28 04:37:33 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:37:33 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:37:35 INFO Observer completed successfully session_id=pref-0edc2aef resource_id=user_999 scope=resource new_messages=20 skipped_trivial=0 total_tokens=55 reflector_triggered=false hybrid_indexed=false duration_ms=1964
    benchmark_longmemeval_test.go:821: [4/30] 0edc2aef — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:37:37 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:37:37 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:37:37 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:37:37 INFO Verification layer initialized
2026/07/28 04:37:37 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:37:37 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:38:04 INFO Observer completed successfully session_id=pref-35a27287 resource_id=user_999 scope=resource new_messages=14 skipped_trivial=0 total_tokens=530 reflector_triggered=false hybrid_indexed=false duration_ms=27211
    benchmark_longmemeval_test.go:821: [5/30] 35a27287 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:38:12 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:38:12 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:38:12 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:38:12 INFO Verification layer initialized
2026/07/28 04:38:12 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:38:12 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:38:20 INFO Observer completed successfully session_id=pref-32260d93 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=245 reflector_triggered=false hybrid_indexed=false duration_ms=7547
    benchmark_longmemeval_test.go:821: [6/30] 32260d93 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:38:23 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:38:23 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:38:23 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:38:23 INFO Verification layer initialized
2026/07/28 04:38:23 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:38:23 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:38:31 INFO Observer completed successfully session_id=pref-195a1a1b resource_id=user_999 scope=resource new_messages=16 skipped_trivial=0 total_tokens=440 reflector_triggered=false hybrid_indexed=false duration_ms=8287
    benchmark_longmemeval_test.go:821: [7/30] 195a1a1b — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:38:37 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:38:37 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:38:37 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:38:37 INFO Verification layer initialized
2026/07/28 04:38:37 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:38:37 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:39:10 INFO Observer completed successfully session_id=pref-afdc33df resource_id=user_999 scope=resource new_messages=14 skipped_trivial=0 total_tokens=107 reflector_triggered=false hybrid_indexed=false duration_ms=33184
    benchmark_longmemeval_test.go:821: [8/30] afdc33df — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:39:12 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:39:12 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:39:12 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:39:12 INFO Verification layer initialized
2026/07/28 04:39:12 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:39:12 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:39:23 INFO Observer completed successfully session_id=pref-caf03d32 resource_id=user_999 scope=resource new_messages=18 skipped_trivial=0 total_tokens=328 reflector_triggered=false hybrid_indexed=false duration_ms=10761
    benchmark_longmemeval_test.go:821: [9/30] caf03d32 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:39:26 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:39:26 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:39:26 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:39:26 INFO Verification layer initialized
2026/07/28 04:39:26 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:39:26 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:39:31 INFO Observer completed successfully session_id=pref-54026fce resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=173 reflector_triggered=false hybrid_indexed=false duration_ms=4443
    benchmark_longmemeval_test.go:821: [10/30] 54026fce — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:39:44 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:39:44 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:39:44 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:39:44 INFO Verification layer initialized
2026/07/28 04:39:44 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:39:44 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:39:54 INFO Observer completed successfully session_id=pref-06f04340 resource_id=user_999 scope=resource new_messages=14 skipped_trivial=0 total_tokens=129 reflector_triggered=false hybrid_indexed=false duration_ms=10194
    benchmark_longmemeval_test.go:821: [11/30] 06f04340 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:40:06 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:40:06 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:40:06 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:40:06 INFO Verification layer initialized
2026/07/28 04:40:06 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:40:06 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:41:20 INFO Observer completed successfully session_id=pref-6b7dfb22 resource_id=user_999 scope=resource new_messages=16 skipped_trivial=0 total_tokens=155 reflector_triggered=false hybrid_indexed=false duration_ms=73830
    benchmark_longmemeval_test.go:821: [12/30] 6b7dfb22 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:41:26 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:41:26 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:41:26 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:41:26 INFO Verification layer initialized
2026/07/28 04:41:26 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:41:26 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:41:41 INFO Observer completed successfully session_id=pref-1a1907b4 resource_id=user_999 scope=resource new_messages=16 skipped_trivial=0 total_tokens=390 reflector_triggered=false hybrid_indexed=false duration_ms=15631
    benchmark_longmemeval_test.go:821: [13/30] 1a1907b4 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:41:49 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:41:49 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:41:49 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:41:49 INFO Verification layer initialized
2026/07/28 04:41:49 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:41:49 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:42:08 INFO Observer completed successfully session_id=pref-09d032c9 resource_id=user_999 scope=resource new_messages=20 skipped_trivial=0 total_tokens=317 reflector_triggered=false hybrid_indexed=false duration_ms=18552
    benchmark_longmemeval_test.go:821: [14/30] 09d032c9 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:42:09 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:42:09 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:42:09 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:42:09 INFO Verification layer initialized
2026/07/28 04:42:09 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:42:09 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:42:29 INFO Observer completed successfully session_id=pref-38146c39 resource_id=user_999 scope=resource new_messages=16 skipped_trivial=0 total_tokens=566 reflector_triggered=false hybrid_indexed=false duration_ms=19755
    benchmark_longmemeval_test.go:821: [15/30] 38146c39 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:42:36 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:42:37 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:42:37 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:42:37 INFO Verification layer initialized
2026/07/28 04:42:37 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:42:37 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:42:41 INFO Observer completed successfully session_id=pref-d24813b1 resource_id=user_999 scope=resource new_messages=14 skipped_trivial=0 total_tokens=49 reflector_triggered=false hybrid_indexed=false duration_ms=4951
    benchmark_longmemeval_test.go:821: [16/30] d24813b1 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:42:45 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:42:46 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:42:46 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:42:46 INFO Verification layer initialized
2026/07/28 04:42:46 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:42:46 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:43:05 INFO Observer completed successfully session_id=pref-57f827a0 resource_id=user_999 scope=resource new_messages=14 skipped_trivial=0 total_tokens=213 reflector_triggered=false hybrid_indexed=false duration_ms=19427
    benchmark_longmemeval_test.go:821: [17/30] 57f827a0 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:43:08 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:43:08 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:43:08 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:43:08 INFO Verification layer initialized
2026/07/28 04:43:08 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:43:08 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:43:12 INFO Observer completed successfully session_id=pref-95228167 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=460 reflector_triggered=false hybrid_indexed=false duration_ms=4031
    benchmark_longmemeval_test.go:821: [18/30] 95228167 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:43:19 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:43:19 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:43:19 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:43:19 INFO Verification layer initialized
2026/07/28 04:43:19 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:43:19 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:43:21 INFO Observer completed successfully session_id=pref-505af2f5 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=9 reflector_triggered=false hybrid_indexed=false duration_ms=1536
    benchmark_longmemeval_test.go:821: [19/30] 505af2f5 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:43:26 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:43:26 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:43:26 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:43:26 INFO Verification layer initialized
2026/07/28 04:43:26 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:43:26 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:43:38 INFO Observer completed successfully session_id=pref-75f70248 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=198 reflector_triggered=false hybrid_indexed=false duration_ms=11672
    benchmark_longmemeval_test.go:821: [20/30] 75f70248 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:43:47 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:43:47 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:43:47 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:43:47 INFO Verification layer initialized
2026/07/28 04:43:47 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:43:47 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:44:08 INFO Observer completed successfully session_id=pref-d6233ab6 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=586 reflector_triggered=false hybrid_indexed=false duration_ms=20965
    benchmark_longmemeval_test.go:821: [21/30] d6233ab6 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:44:14 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:44:14 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:44:14 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:44:14 INFO Verification layer initialized
2026/07/28 04:44:14 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:44:14 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:44:38 INFO Observer completed successfully session_id=pref-1da05512 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=32 reflector_triggered=false hybrid_indexed=false duration_ms=24584
    benchmark_longmemeval_test.go:821: [22/30] 1da05512 — no
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:44:41 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:44:41 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:44:41 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:44:41 INFO Verification layer initialized
2026/07/28 04:44:41 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:44:41 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:44:51 INFO Observer completed successfully session_id=pref-fca70973 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=285 reflector_triggered=false hybrid_indexed=false duration_ms=9745
    benchmark_longmemeval_test.go:821: [23/30] fca70973 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:44:56 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:44:56 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:44:56 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:44:56 INFO Verification layer initialized
2026/07/28 04:44:56 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:44:56 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:45:09 INFO Observer completed successfully session_id=pref-b6025781 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=371 reflector_triggered=false hybrid_indexed=false duration_ms=13532
    benchmark_longmemeval_test.go:821: [24/30] b6025781 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:45:12 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:45:12 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:45:12 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:45:12 INFO Verification layer initialized
2026/07/28 04:45:12 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:45:12 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:45:52 INFO Observer completed successfully session_id=pref-a89d7624 resource_id=user_999 scope=resource new_messages=14 skipped_trivial=0 total_tokens=244 reflector_triggered=false hybrid_indexed=false duration_ms=39392
    benchmark_longmemeval_test.go:821: [25/30] a89d7624 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:46:10 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:46:10 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:46:10 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:46:10 INFO Verification layer initialized
2026/07/28 04:46:10 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:46:10 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:46:20 INFO Observer completed successfully session_id=pref-b0479f84 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=254 reflector_triggered=false hybrid_indexed=false duration_ms=9792
    benchmark_longmemeval_test.go:821: [26/30] b0479f84 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:46:22 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:46:22 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:46:22 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:46:22 INFO Verification layer initialized
2026/07/28 04:46:22 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:46:22 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:46:25 INFO Observer completed successfully session_id=pref-1d4e3b97 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=156 reflector_triggered=false hybrid_indexed=false duration_ms=2195
    benchmark_longmemeval_test.go:821: [27/30] 1d4e3b97 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:46:27 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:46:27 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:46:27 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:46:27 INFO Verification layer initialized
2026/07/28 04:46:27 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:46:27 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:46:37 INFO Observer completed successfully session_id=pref-07b6f563 resource_id=user_999 scope=resource new_messages=14 skipped_trivial=0 total_tokens=141 reflector_triggered=false hybrid_indexed=false duration_ms=9601
    benchmark_longmemeval_test.go:821: [28/30] 07b6f563 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:46:39 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:46:39 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:46:39 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:46:39 INFO Verification layer initialized
2026/07/28 04:46:39 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:46:39 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:47:18 INFO Observer completed successfully session_id=pref-1c0ddc50 resource_id=user_999 scope=resource new_messages=12 skipped_trivial=0 total_tokens=299 reflector_triggered=false hybrid_indexed=false duration_ms=38458
    benchmark_longmemeval_test.go:821: [29/30] 1c0ddc50 — yes
    store.go:87: failed to load .env file, but it's ok
2026/07/28 04:47:21 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/28 04:47:21 INFO Column already exists, skipping table=tickets column=type
2026/07/28 04:47:21 INFO Column already exists, skipping table=tickets column=tags
2026/07/28 04:47:21 INFO Verification layer initialized
2026/07/28 04:47:21 INFO RAG pipeline disabled, using no-op vector database
2026/07/28 04:47:21 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/28 04:47:23 INFO Observer completed successfully session_id=pref-0a34ad58 resource_id=user_999 scope=resource new_messages=20 skipped_trivial=0 total_tokens=9 reflector_triggered=false hybrid_indexed=false duration_ms=1899
    benchmark_longmemeval_test.go:821: [30/30] 0a34ad58 — no
    benchmark_longmemeval_test.go:821: === implicit_preference_v2 Summary: 14 passed, 16 failed, 0 skipped ===
--- PASS: TestBenchmarkPreference (643.15s)
PASS
ok  	github.com/usememos/memos/server/router/api/v1/agent	643.205s

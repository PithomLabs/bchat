go test ./...
?   	github.com/usememos/memos/bin/memos	[no test files]
?   	github.com/usememos/memos/build/widget-test	[no test files]
?   	github.com/usememos/memos/cmd/parser	[no test files]
ok  	github.com/usememos/memos/internal/base	(cached)
ok  	github.com/usememos/memos/internal/bridgeworker	0.260s
?   	github.com/usememos/memos/internal/crypto	[no test files]
?   	github.com/usememos/memos/internal/profile	[no test files]
ok  	github.com/usememos/memos/internal/util	(cached)
ok  	github.com/usememos/memos/internal/version	(cached)
ok  	github.com/usememos/memos/plugin/cron	(cached)
?   	github.com/usememos/memos/plugin/filter	[no test files]
ok  	github.com/usememos/memos/plugin/httpgetter	(cached)
?   	github.com/usememos/memos/plugin/idp	[no test files]
ok  	github.com/usememos/memos/plugin/idp/oauth2	(cached)
?   	github.com/usememos/memos/plugin/storage/s3	[no test files]
ok  	github.com/usememos/memos/plugin/webhook	(cached) [no tests to run]
?   	github.com/usememos/memos/proto/gen/api/v1	[no test files]
?   	github.com/usememos/memos/proto/gen/store	[no test files]
?   	github.com/usememos/memos/server	[no test files]
?   	github.com/usememos/memos/server/profiler	[no test files]
ok  	github.com/usememos/memos/server/router/api/v1	0.976s
2026/07/20 01:56:24 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/20 01:56:24 INFO Column already exists, skipping table=tickets column=type
2026/07/20 01:56:24 INFO Column already exists, skipping table=tickets column=tags
2026/07/20 01:56:24 INFO Encryption service initialized for tenant API keys
2026/07/20 01:56:24 INFO Verification layer initialized
2026/07/20 01:56:24 INFO RAG pipeline disabled, using no-op vector database
2026/07/20 01:56:24 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/20 01:56:24 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/20 01:56:24 INFO Column already exists, skipping table=tickets column=type
2026/07/20 01:56:24 INFO Column already exists, skipping table=tickets column=tags
2026/07/20 01:56:24 INFO Encryption service initialized for tenant API keys
2026/07/20 01:56:24 INFO Verification layer initialized
2026/07/20 01:56:24 INFO RAG pipeline disabled, using no-op vector database
2026/07/20 01:56:24 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/20 01:56:24 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/20 01:56:24 INFO Column already exists, skipping table=tickets column=type
2026/07/20 01:56:24 INFO Column already exists, skipping table=tickets column=tags
2026/07/20 01:56:24 INFO Encryption service initialized for tenant API keys
2026/07/20 01:56:24 INFO Verification layer initialized
2026/07/20 01:56:24 INFO RAG pipeline disabled, using no-op vector database
2026/07/20 01:56:24 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/20 01:56:24 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/20 01:56:24 INFO Column already exists, skipping table=tickets column=type
2026/07/20 01:56:24 INFO Column already exists, skipping table=tickets column=tags
2026/07/20 01:56:24 INFO Encryption service initialized for tenant API keys
2026/07/20 01:56:24 INFO Verification layer initialized
2026/07/20 01:56:24 INFO RAG pipeline disabled, using no-op vector database
2026/07/20 01:56:24 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/20 01:56:24 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/20 01:56:24 INFO Column already exists, skipping table=tickets column=type
2026/07/20 01:56:24 INFO Column already exists, skipping table=tickets column=tags
2026/07/20 01:56:24 INFO Encryption service initialized for tenant API keys
2026/07/20 01:56:24 INFO Verification layer initialized
2026/07/20 01:56:24 INFO RAG pipeline disabled, using no-op vector database
2026/07/20 01:56:24 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/20 01:56:24 INFO chat mode decision tenant_id=1 retrieval_mode="" has_structured_content=false use_rag=false rag_enabled=false session_id=session-visitor-5
2026/07/20 01:56:24 WARN chat: LLM config unavailable tenant=1 error="no OpenRouter API key configured for tenant 1"
--- FAIL: TestBChatLiveReleaseAllowsAIResume (0.08s)
    store.go:87: failed to load .env file, but it's ok
    bridge_delivery_test.go:238: 
        	Error Trace:	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/bridge_delivery_test.go:238
        	Error:      	Received unexpected error:
        	            	failed to generate response: chat service unavailable for tenant 1: no OpenRouter API key configured for tenant 1
        	Test:       	TestBChatLiveReleaseAllowsAIResume
2026/07/20 01:56:24 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/20 01:56:24 INFO Column already exists, skipping table=tickets column=type
2026/07/20 01:56:24 INFO Column already exists, skipping table=tickets column=tags
2026/07/20 01:56:24 INFO Encryption service initialized for tenant API keys
2026/07/20 01:56:24 INFO Verification layer initialized
2026/07/20 01:56:24 INFO RAG pipeline disabled, using no-op vector database
2026/07/20 01:56:24 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/20 01:56:24 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/20 01:56:24 INFO Column already exists, skipping table=tickets column=type
2026/07/20 01:56:24 INFO Column already exists, skipping table=tickets column=tags
2026/07/20 01:56:25 INFO Encryption service initialized for tenant API keys
2026/07/20 01:56:25 INFO Verification layer initialized
2026/07/20 01:56:25 INFO RAG pipeline disabled, using no-op vector database
2026/07/20 01:56:25 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/20 01:56:25 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/20 01:56:25 INFO Column already exists, skipping table=tickets column=type
2026/07/20 01:56:25 INFO Column already exists, skipping table=tickets column=tags
2026/07/20 01:56:25 INFO Encryption service initialized for tenant API keys
2026/07/20 01:56:25 INFO Verification layer initialized
2026/07/20 01:56:25 INFO RAG pipeline disabled, using no-op vector database
2026/07/20 01:56:25 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/20 01:56:25 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/20 01:56:25 INFO Column already exists, skipping table=tickets column=type
2026/07/20 01:56:25 INFO Column already exists, skipping table=tickets column=tags
2026/07/20 01:56:25 INFO Encryption service initialized for tenant API keys
2026/07/20 01:56:25 INFO Verification layer initialized
2026/07/20 01:56:25 INFO RAG pipeline disabled, using no-op vector database
2026/07/20 01:56:25 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/20 01:56:25 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/20 01:56:25 INFO Column already exists, skipping table=tickets column=type
2026/07/20 01:56:25 INFO Column already exists, skipping table=tickets column=tags
2026/07/20 01:56:25 INFO Encryption service initialized for tenant API keys
2026/07/20 01:56:25 INFO Verification layer initialized
2026/07/20 01:56:25 INFO RAG pipeline disabled, using no-op vector database
2026/07/20 01:56:25 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/20 01:56:25 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/20 01:56:25 INFO Column already exists, skipping table=tickets column=type
2026/07/20 01:56:25 INFO Column already exists, skipping table=tickets column=tags
2026/07/20 01:56:25 INFO Encryption service initialized for tenant API keys
2026/07/20 01:56:25 INFO Verification layer initialized
2026/07/20 01:56:25 INFO RAG pipeline disabled, using no-op vector database
2026/07/20 01:56:25 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/20 01:56:25 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/20 01:56:25 INFO Column already exists, skipping table=tickets column=type
2026/07/20 01:56:25 INFO Column already exists, skipping table=tickets column=tags
2026/07/20 01:56:25 INFO Encryption service initialized for tenant API keys
2026/07/20 01:56:25 INFO Verification layer initialized
2026/07/20 01:56:25 INFO RAG pipeline disabled, using no-op vector database
2026/07/20 01:56:25 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/20 01:56:25 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/20 01:56:25 INFO Column already exists, skipping table=tickets column=type
2026/07/20 01:56:25 INFO Column already exists, skipping table=tickets column=tags
2026/07/20 01:56:25 INFO Encryption service initialized for tenant API keys
2026/07/20 01:56:25 INFO Verification layer initialized
2026/07/20 01:56:25 INFO RAG pipeline disabled, using no-op vector database
2026/07/20 01:56:25 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/20 01:56:25 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/20 01:56:25 INFO Column already exists, skipping table=tickets column=type
2026/07/20 01:56:25 INFO Column already exists, skipping table=tickets column=tags
2026/07/20 01:56:25 INFO Encryption service initialized for tenant API keys
2026/07/20 01:56:25 INFO Verification layer initialized
2026/07/20 01:56:25 INFO RAG pipeline disabled, using no-op vector database
2026/07/20 01:56:25 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/20 01:56:25 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/20 01:56:25 INFO Column already exists, skipping table=tickets column=type
2026/07/20 01:56:25 INFO Column already exists, skipping table=tickets column=tags
2026/07/20 01:56:25 INFO Encryption service initialized for tenant API keys
2026/07/20 01:56:25 INFO Verification layer initialized
2026/07/20 01:56:25 INFO RAG pipeline disabled, using no-op vector database
2026/07/20 01:56:25 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/20 01:56:25 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/20 01:56:25 INFO Column already exists, skipping table=tickets column=type
2026/07/20 01:56:25 INFO Column already exists, skipping table=tickets column=tags
2026/07/20 01:56:25 INFO Encryption service initialized for tenant API keys
2026/07/20 01:56:25 INFO Verification layer initialized
2026/07/20 01:56:25 INFO RAG pipeline disabled, using no-op vector database
2026/07/20 01:56:25 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/20 01:56:25 INFO chat mode decision tenant_id=1 retrieval_mode="" has_structured_content=false use_rag=false rag_enabled=false session_id=c13c1c47-ca99-4ae3-ba9a-47a9fb92738b
2026/07/20 01:56:25 WARN chat: LLM config unavailable tenant=1 error="no OpenRouter API key configured for tenant 1"
2026/07/20 01:56:25 ERROR chat external failed slug=live-e2e-flow error="failed to generate response: chat service unavailable for tenant 1: no OpenRouter API key configured for tenant 1"
--- FAIL: TestBChatLiveEndToEndVisitorHumanReplyFlow (0.09s)
    store.go:87: failed to load .env file, but it's ok
    bridge_delivery_test.go:854: 
        	Error Trace:	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/bridge_delivery_test.go:854
        	Error:      	Received unexpected error:
        	            	code=500, message=Chat service unavailable
        	Test:       	TestBChatLiveEndToEndVisitorHumanReplyFlow
2026/07/20 01:56:25 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/20 01:56:25 INFO Column already exists, skipping table=tickets column=type
2026/07/20 01:56:25 INFO Column already exists, skipping table=tickets column=tags
2026/07/20 01:56:25 INFO Encryption service initialized for tenant API keys
2026/07/20 01:56:25 INFO Verification layer initialized
2026/07/20 01:56:25 INFO RAG pipeline disabled, using no-op vector database
2026/07/20 01:56:25 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
2026/07/20 01:56:25 WARN failed to find migration history in pre-migrate error="SQL logic error: no such table: migration_history (1)"
2026/07/20 01:56:25 INFO Column already exists, skipping table=tickets column=type
2026/07/20 01:56:25 INFO Column already exists, skipping table=tickets column=tags
2026/07/20 01:56:25 INFO Encryption service initialized for tenant API keys
2026/07/20 01:56:25 INFO Verification layer initialized
2026/07/20 01:56:25 INFO RAG pipeline disabled, using no-op vector database
2026/07/20 01:56:25 INFO Observer buffer initialized buffer_tokens_fraction=0.2 activation_fraction=0.8 block_after_fraction=1.2
--- FAIL: TestChatExternalClientMessageIDIsIdempotent (0.07s)
    store.go:87: failed to load .env file, but it's ok
    bridge_foundation_test.go:193: 
        	Error Trace:	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/bridge_foundation_test.go:193
        	Error:      	Received unexpected error:
        	            	failed to generate response: chat service unavailable for tenant 1: no OpenRouter API key configured for tenant 1
        	Test:       	TestChatExternalClientMessageIDIsIdempotent
--- FAIL: TestChatExternalClientMessageIDIsIdempotent_Concurrent (0.22s)
    store.go:87: failed to load .env file, but it's ok
    bridge_foundation_test.go:230: 
        	Error Trace:	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/bridge_foundation_test.go:230
        	Error:      	Received unexpected error:
        	            	failed to generate response: chat service unavailable for tenant 1: no OpenRouter API key configured for tenant 1
        	Test:       	TestChatExternalClientMessageIDIsIdempotent_Concurrent
        	Messages:   	goroutine 0 errored
--- FAIL: TestChatExternalClientMessageIDIsIdempotent_Restart (0.09s)
    store.go:87: failed to load .env file, but it's ok
    bridge_foundation_test.go:254: 
        	Error Trace:	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/bridge_foundation_test.go:254
        	Error:      	Received unexpected error:
        	            	failed to generate response: chat service unavailable for tenant 1: no OpenRouter API key configured for tenant 1
        	Test:       	TestChatExternalClientMessageIDIsIdempotent_Restart
--- FAIL: TestChatExternalClientMessageIDPersistsToDatabase (0.07s)
    store.go:87: failed to load .env file, but it's ok
    bridge_foundation_test.go:274: 
        	Error Trace:	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/bridge_foundation_test.go:274
        	Error:      	Received unexpected error:
        	            	failed to generate response: chat service unavailable for tenant 1: no OpenRouter API key configured for tenant 1
        	Test:       	TestChatExternalClientMessageIDPersistsToDatabase
--- FAIL: TestChatExternalEscalationCreatesLeadAndTicketWithoutHandoff (0.09s)
    store.go:87: failed to load .env file, but it's ok
    bridge_foundation_test.go:303: 
        	Error Trace:	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/bridge_foundation_test.go:303
        	Error:      	Received unexpected error:
        	            	failed to generate response: chat service unavailable for tenant 1: no OpenRouter API key configured for tenant 1
        	Test:       	TestChatExternalEscalationCreatesLeadAndTicketWithoutHandoff
--- FAIL: TestChatExternalEscalationDedupesTicketAcrossServiceRestart (0.09s)
    store.go:87: failed to load .env file, but it's ok
    bridge_foundation_test.go:343: 
        	Error Trace:	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/bridge_foundation_test.go:343
        	Error:      	Received unexpected error:
        	            	failed to generate response: chat service unavailable for tenant 1: no OpenRouter API key configured for tenant 1
        	Test:       	TestChatExternalEscalationDedupesTicketAcrossServiceRestart
--- FAIL: TestChatExternalEscalationWithIncompleteContactAsksForContactInfo (0.09s)
    store.go:87: failed to load .env file, but it's ok
    bridge_foundation_test.go:365: 
        	Error Trace:	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/bridge_foundation_test.go:365
        	Error:      	Received unexpected error:
        	            	failed to generate response: chat service unavailable for tenant 1: no OpenRouter API key configured for tenant 1
        	Test:       	TestChatExternalEscalationWithIncompleteContactAsksForContactInfo
--- FAIL: TestChatExternalClientMessageIDContentMismatch (0.08s)
    store.go:87: failed to load .env file, but it's ok
    bridge_foundation_test.go:402: 
        	Error Trace:	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/bridge_foundation_test.go:402
        	Error:      	Received unexpected error:
        	            	failed to generate response: chat service unavailable for tenant 1: no OpenRouter API key configured for tenant 1
        	Test:       	TestChatExternalClientMessageIDContentMismatch
--- FAIL: TestMaterializationFailureLogsSanitizedWarningOnce (0.09s)
    store.go:87: failed to load .env file, but it's ok
    bridge_foundation_test.go:485: 
        	Error Trace:	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/bridge_foundation_test.go:485
        	Error:      	Received unexpected error:
        	            	failed to generate response: chat service unavailable for tenant 1: no OpenRouter API key configured for tenant 1
        	Test:       	TestMaterializationFailureLogsSanitizedWarningOnce
--- FAIL: TestUnsupportedDBPathCreatesNoWarnings (0.06s)
    store.go:87: failed to load .env file, but it's ok
    bridge_foundation_test.go:578: 
        	Error Trace:	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/bridge_foundation_test.go:578
        	Error:      	Received unexpected error:
        	            	failed to generate response: chat service unavailable for tenant 1: no OpenRouter API key configured for tenant 1
        	Test:       	TestUnsupportedDBPathCreatesNoWarnings
--- FAIL: TestChatExternalAfterReleaseResumesAIBehavior (0.07s)
    store.go:87: failed to load .env file, but it's ok
    bridge_runtime_test.go:91: 
        	Error Trace:	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/bridge_runtime_test.go:91
        	Error:      	Received unexpected error:
        	            	failed to generate response: chat service unavailable for tenant 1: no OpenRouter API key configured for tenant 1
        	Test:       	TestChatExternalAfterReleaseResumesAIBehavior
--- FAIL: TestChatExternalHumanActiveHandoffDoesNotAppendUserOrAIMessage (0.08s)
    store.go:87: failed to load .env file, but it's ok
    bridge_runtime_test.go:105: 
        	Error Trace:	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/bridge_runtime_test.go:105
        	Error:      	Received unexpected error:
        	            	failed to generate response: chat service unavailable for tenant 1: no OpenRouter API key configured for tenant 1
        	Test:       	TestChatExternalHumanActiveHandoffDoesNotAppendUserOrAIMessage
--- FAIL: TestChatExternalUnsupportedBridgeDBDoesNotBreakNormalChat (0.06s)
    store.go:87: failed to load .env file, but it's ok
    bridge_runtime_test.go:143: 
        	Error Trace:	/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/bridge_runtime_test.go:143
        	Error:      	Received unexpected error:
        	            	failed to generate response: chat service unavailable for tenant 1: no OpenRouter API key configured for tenant 1
        	Test:       	TestChatExternalUnsupportedBridgeDBDoesNotBreakNormalChat
FAIL
FAIL	github.com/usememos/memos/server/router/api/v1/agent	7.442s
?   	github.com/usememos/memos/server/router/frontend	[no test files]
?   	github.com/usememos/memos/server/router/rss	[no test files]
?   	github.com/usememos/memos/server/runner/memopayload	[no test files]
?   	github.com/usememos/memos/server/runner/s3presign	[no test files]
?   	github.com/usememos/memos/server/service	[no test files]
?   	github.com/usememos/memos/store	[no test files]
ok  	github.com/usememos/memos/store/cache	(cached)
?   	github.com/usememos/memos/store/db	[no test files]
ok  	github.com/usememos/memos/store/db/mysql	(cached)
ok  	github.com/usememos/memos/store/db/postgres	(cached)
ok  	github.com/usememos/memos/store/db/sqlite	(cached)
ok  	github.com/usememos/memos/store/test	8.707s
FAIL

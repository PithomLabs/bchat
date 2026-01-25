Under build folder
-----

```
export OPENROUTER_API_KEY=
export LLM_MODEL=xiaomi/mimo-v2-flash:free

chaschel@xia:~/Documents/ibm/ai/chat/build$ ./memos --mode dev --driver sqlite --data ./data
```

Under chat folder
-----
chaschel@xia:~/Documents/ibm/ai/chat$ 

Misc
-----

```
lsof -i :8081
kill -9 PID

git tag -a v0.1.0 -m "DOCS_CHAT_PAGE.MD"
git tag -a v0.2.0 -m "chat agent"
git tag -a v0.2.1 -m "DOCS_CHAT_DESIGN_4_IMP_2-PROGRESS.MD"
git tag -a v0.3.0 -m "DOCS_RBAC.MD"
git tag -a v0.3.1 -m "DOCS_RBAC_2.MD"
git tag -a v0.3.2 -m "fixes on DOCS_RBAC_2.MD"
git tag -a v0.3.3 -m "fixes across the board DOCS_RBAC_2.MD"
git tag -a v0.4.0 -m "DOCS_SIMULATION.MD"
git tag -a v0.4.1 -m "simulation fixes"
git tag -a v0.4.2 -m "agent self-improvement"
git tag -a v0.4.3 -m "agent self-improvement fixes"
git tag -a v0.4.4 -m "simplify self-improvement"
git tag -a v0.4.5 -m "sim addendums"
git tag -a v0.4.6 -m "tighten MD files"
git tag -a v0.4.7 -m "DOCS_SIMULATION_2*.MD"
git tag -a v0.5.8 -m "chat widget"
git tag -a v0.5.9 -m "rag stats"
git tag -a v0.5.10 -m "parse MD files"
git tag -a v0.5.11 -m "rag reindex"
git push origin --tags
```


export OPENROUTER_API_KEY=
export LLM_MODEL=xiaomi/mimo-v2-flash:free
export OPENROUTER_API_KEY=
export LLM_MODEL=xiaomi/mimo-v2-flash:free
export LLM_MODEL=openai/gpt-oss-120b:free
export ENCRYPTION_MASTER_KEY=ced26e6e-91ce-4120-b4c0-68ec9ea25701

xiaomi/mimo-v2-flash:free
openai/gpt-oss-120b:free
nvidia/nemotron-3-nano-30b-a3b:free
google/gemma-3-27b-it:free
qwen/qwen3-coder:free
z-ai/glm-4.5-air:free
nousresearch/hermes-3-llama-3.1-405b:free

uuidgen to encrypt OPENROUTER_API_KEY
./memos --mode dev --driver sqlite --data ./data

To build RAG
go build -tags rag -o build/memos ./bin/memos

RAG_PIPELINE_ENABLED=true REINDEX_RAG=true task run:rag

RAG_PIPELINE_ENABLED=true EMBEDDING_PROVIDER=openrouter LANCEDB_STORAGE_PROVIDER=local ./build/memos --mode dev --data /home/chaschel/Documents/ibm/ai/bchat/build/data
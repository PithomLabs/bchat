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
git tag -a v0.5.12 -m "updated docs"
git tag -a v0.5.13 -m "cybertron"
git tag -a v0.5.14 -m "fixed rebuild index/rag"
git tag -a v0.5.15 -m "QA pairs, RAG Search Explorer"
git tag -a v0.5.16 -m "save chat transcript"
git tag -a v0.5.17 -m "CHANGELOG.MD 2026-01-28 2218"
git tag -a v0.5.18 -m "deploy to fly.io"
git tag -a v0.5.19 -m "rebuild index fixes"
git tag -a v0.5.20 -m "CHANGELOG.MD 2026-01-31 0320"
git tag -a v0.5.21 -m "fixed chat widget"
git tag -a v0.5.22 -m "CHANGELOG.MD 2026-02-01 1017"
git tag -a v0.5.23 -m "CHANGELOG.MD 2026-02-01 2011"
git tag -a v0.5.24 -m "DOCS_RAG_OVERVIEW.MD"
git tag -a v0.5.25 -m "fixed 1536 dimensions"
git push origin --tags
```


export OPENROUTER_API_KEY=
export LLM_MODEL=xiaomi/mimo-v2-flash:free
export OPENROUTER_API_KEY=
export LLM_MODEL=xiaomi/mimo-v2-flash:free
export LLM_MODEL=openai/gpt-oss-120b:free

uuidgen to encrypt OPENROUTER_API_KEY
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

To test the server:

```
task build:rag
task build:rag:all

task run:rag:l12
```


To test the widget:              
-----                                           
```                                                                              
  # Terminal 1: Run the backend                                               
  task run:rag                                                                
                                                                              
  # Terminal 2: Serve widget test files                                       
  cd widget/site && python3 -m http.server 8080                                
                                                                              
  Then open http://localhost:8080 in your browser.
```
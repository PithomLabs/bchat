```
fly secrets set EMBEDDING_MODEL=sentence-transformers/all-MiniLM-L12-v2 -a bchat

fly secrets set EMBEDDING_MODEL=openai/text-embedding-3-small -a bchat

fly secrets set EMBEDDING_TIMEOUT=5m

fly apps restart bchat


```
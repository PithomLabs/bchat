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
git push origin --tags
```

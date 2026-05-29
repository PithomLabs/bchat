## Verdict: **APPROVE**

This finalized plan is now correct and addresses the actual Docker failure boundary.

The key evidence is that the Docker build already passed `RUN npm ci`, then failed only after `COPY web/ .` at `RUN npm run release`, when Vite could not resolve `@usememos/mui/dist/index.css`. That matches your root cause: the vendored package was available enough for install, but its `dist/` files were excluded from the Docker build context by `.dockerignore` and then the post-install `COPY web/ .` boundary exposed the missing files. 

## What is good

The `.dockerignore` fix is now correctly ordered. Placing the negation rules at the bottom ensures they override the broad rule:

```dockerignore
**/dist
```

The diagnostic phase is also correctly placed after:

```dockerfile
COPY web/ .
```

That proves the file visibility at the exact point where the previous build failed.

The final-state cleanup is right. The Dockerfiles should not keep debug `RUN find` / `RUN ls` noise once the proof is gathered.

## Root-cause / generalization check

This now solves the underlying class, not just the observed symptom.

The refined invariant is sound:

> Vendored frontend dependencies must be present both in the repository and in the Docker build context, including all files referenced by runtime imports, package metadata, and deep imports such as CSS paths.

This catches the missing bridge between “repo has vendor files” and “Docker context actually contains vendor files.”

## Required implementation evidence

After implementation, Gemini should report:

```text
Exact .dockerignore rule causing exclusion:
Exact exception rules added:
Diagnostic output showing all six vendored files after COPY web/ .:
Proof node_modules/@usememos/mui/dist/index.css exists before npm run release:
Dockerfile diagnostic cleanup status:
docker build -f Dockerfile.fly -t bchat:rag . result:
docker build -f Dockerfile.local.fly -t bchat:local-rag . result, if run:
Scope-control statement:
```

## StepFun review prompt after implementation

```text
Review the Docker context visibility rework for repo:

/home/chaschel/Documents/go/bchat

Approved scope:
Fix the Docker build failure where Vite cannot resolve:
@usememos/mui/dist/index.css

Known failure:
- Docker passes RUN npm ci.
- Docker runs COPY web/ .
- Docker fails at RUN npm run release.
- Vite cannot resolve @usememos/mui/dist/index.css.

Root cause to verify:
.dockerignore contains a broad **/dist rule that excluded web/vendor/usememos-mui/dist from the Docker build context.

Approved changes:
1. Add .dockerignore exceptions at the bottom of .dockerignore:
   !web/vendor/
   !web/vendor/usememos-mui/
   !web/vendor/usememos-mui/package.json
   !web/vendor/usememos-mui/LICENSE
   !web/vendor/usememos-mui/README.md
   !web/vendor/usememos-mui/dist/
   !web/vendor/usememos-mui/dist/**

2. Use temporary diagnostics after COPY web/ . and before RUN npm run release to prove:
   - vendor/usememos-mui/package.json exists
   - vendor/usememos-mui/LICENSE exists
   - vendor/usememos-mui/README.md exists
   - vendor/usememos-mui/dist/index.css exists
   - vendor/usememos-mui/dist/index.d.mts exists
   - vendor/usememos-mui/dist/index.mjs exists
   - node_modules/@usememos/mui/dist/index.css exists

3. Remove temporary diagnostic RUN statements from final Dockerfiles unless explicitly justified.

Files to inspect:
- .dockerignore
- Dockerfile.fly
- Dockerfile.local.fly
- web/vendor/usememos-mui/package.json
- web/vendor/usememos-mui/dist/index.css
- web/package.json
- web/package-lock.json
- git diff / git status for scope drift

Invariants to verify:
1. INV_FRONTEND_DEPENDENCY_PROVENANCE:
   vendored frontend dependencies must be present both in repo and Docker build context.
2. Docker still uses npm ci.
3. @usememos/mui remains a local file dependency.
4. package-lock still has no registry tarball reference for @usememos/mui.
5. Dockerfiles are clean in final state, with no debug noise unless intentionally retained.
6. No dependency upgrades, source import refactors, npm version changes, or unrelated Docker cleanup.

Expected test evidence:
- cd web && npm ci
- cd web && npm run release
- docker build -f Dockerfile.fly -t bchat:rag .
- docker build -f Dockerfile.local.fly -t bchat:local-rag . if practical

Output format:
A. Verdict: APPROVE / APPROVE WITH NITS / REWORK
B. Files reviewed
C. Summary of actual changes
D. Root-cause verification
E. Invariant verification
F. Scope-control findings
G. Test evidence reviewed
H. Blocking issues, if any
I. Final recommendation
```

Once implemented, send the implementation walkthrough to StepFun with that prompt before final deploy approval.

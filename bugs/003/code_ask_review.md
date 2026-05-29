Review the Docker context visibility rework for repo:

/home/chaschel/Documents/go/bchat

Context:
This is the second-stage repair for the frontend dependency provenance fix.

Original issue:
- Fly/Docker failed at RUN npm ci because @usememos/mui@0.1.0-20250601165716 resolved to an unavailable npm registry tarball.

First repair:
- Vendored @usememos/mui into web/vendor/usememos-mui.
- Changed web/package.json to use:
  "@usememos/mui": "file:vendor/usememos-mui"
- Docker now gets past RUN npm ci.

New issue:
- Docker then failed at RUN npm run release.
- Vite could not resolve:
  @usememos/mui/dist/index.css
- Failure occurred after:
  COPY web/ .
- Root cause identified:
  .dockerignore had a broad rule:
  **/dist
  which excluded web/vendor/usememos-mui/dist from the Docker build context.

Approved scope:
Fix only Docker build-context visibility for the vendored @usememos/mui package assets.
Do not upgrade dependencies.
Do not replace npm ci.
Do not refactor frontend imports.
Do not broaden into unrelated Docker cleanup.

Files to inspect:
- .dockerignore
- Dockerfile.fly
- Dockerfile.local.fly
- web/package.json
- web/package-lock.json
- web/vendor/usememos-mui/package.json
- web/vendor/usememos-mui/LICENSE
- web/vendor/usememos-mui/README.md
- web/vendor/usememos-mui/dist/index.css
- web/vendor/usememos-mui/dist/index.d.mts
- web/vendor/usememos-mui/dist/index.mjs
- task.md and walkthrough.md only for scope awareness if modified
- git diff / git status for scope drift

Approved .dockerignore fix:
At the bottom of .dockerignore, after the broad **/dist rule and any other ignore rules, add:

!web/vendor/
!web/vendor/usememos-mui/
!web/vendor/usememos-mui/package.json
!web/vendor/usememos-mui/LICENSE
!web/vendor/usememos-mui/README.md
!web/vendor/usememos-mui/dist/
!web/vendor/usememos-mui/dist/**

Verify:
1. These exceptions are actually after the broad **/dist rule.
2. No later rule re-excludes web/vendor/usememos-mui/dist.
3. The exceptions are specific to the vendored @usememos/mui package and do not broadly re-include all dist directories.

Approved temporary diagnostics:
Temporary diagnostics may be present after:

COPY web/ .

and before:

RUN npm run release

They should prove:
- vendor/usememos-mui/package.json exists
- vendor/usememos-mui/LICENSE exists
- vendor/usememos-mui/README.md exists
- vendor/usememos-mui/dist/index.css exists
- vendor/usememos-mui/dist/index.d.mts exists
- vendor/usememos-mui/dist/index.mjs exists
- node_modules/@usememos/mui/dist/index.css exists

Important final-state check:
The diagnostic RUN find / RUN ls / RUN test lines are acceptable for the proof build, but final production Dockerfiles should remove debug print noise unless the implementation intentionally keeps only a narrow assertion with clear justification.

Invariants to verify:
1. INV_FRONTEND_DEPENDENCY_PROVENANCE:
   Vendored frontend dependencies must be present both in the repository and in the Docker build context, including files referenced by runtime imports, package metadata, and deep imports.
2. Docker still uses npm ci.
3. @usememos/mui remains a local file dependency.
4. package-lock still has no reference to:
   - mui-0.1.0-20250601165716.tgz
   - registry.npmjs.org/@usememos/mui
5. No dependency upgrades, source import refactors, npm version changes, or unrelated Docker changes.
6. The Dockerfile diagnostics are either temporary and marked for removal after proof, or intentionally retained as a narrow assertion.

Evidence to review:
- Local npm ci passed.
- Local npm run release passed.
- Local docker build was NOT completed because the user lacked permission to connect to the Docker socket.
- Therefore, Docker/Fly/Depot build success is still pending and must not be treated as proven unless a new remote build log shows success.

Required test evidence for final approval:
- fly deploy or Docker build reaches past npm run release successfully.
- Diagnostic output, if present, shows all six vendored files after COPY web/ .
- The assertion test -f node_modules/@usememos/mui/dist/index.css passes before npm run release.
- npm run release succeeds inside Docker/Fly/Depot.

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
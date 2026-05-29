## Verdict: **REWORK**

The original provenance repair **partially worked**: Docker now gets past `RUN npm ci`, so the unavailable registry tarball problem is fixed. But the Docker build now fails at:

```text id="ahxhgs"
RUN npm run release
```

because Vite cannot resolve:

```text id="nursnf"
@usememos/mui/dist/index.css
```

from:

```text id="obq13t"
/frontend-build/src/main.tsx
```

So the failure moved from **install-time dependency provenance** to **Docker build-context / vendored-file availability**. 

## Most likely root cause

The vendored package is present enough for `npm ci` to succeed, but inside the Docker image, the actual file:

```text id="g62u5p"
frontend-build/vendor/usememos-mui/dist/index.css
```

or the linked path:

```text id="bfp1fn"
frontend-build/node_modules/@usememos/mui/dist/index.css
```

is missing or not reachable.

The highest-probability cause is `.dockerignore` excluding `dist/` or similar patterns, so `web/vendor/usememos-mui/dist/*` does not actually enter the Docker build context even though it exists locally.

This line is suspicious:

```text id="ztgxka"
[internal] load build context ... transferring context: 2.07GB
```

That suggests the Docker context may also be including large unintended directories, possibly `node_modules`. So this needs a Docker-context audit, not just another package-lock change.

## Recovered invariant refinement

The previous invariant needs a sharper Docker-specific clause:

> **INV_FRONTEND_DEPENDENCY_PROVENANCE:** vendored frontend dependencies must be present both in the repo and in the Docker build context, including all files referenced by runtime imports, package metadata, and deep imports such as CSS paths.

The prior review verified the repo copy. The Docker failure shows we also need to verify the **container-visible copy**.

## Immediate diagnostic commands

Run these from repo root:

```bash id="fsmlpp"
docker build -f Dockerfile.fly -t bchat:rag-debug --target frontend --progress=plain .
```

If the Dockerfile does not expose a named frontend target cleanly, add temporary diagnostics right before `RUN npm run release`:

```dockerfile id="xl2tpa"
RUN find vendor/usememos-mui -maxdepth 3 -type f -print | sort
RUN ls -la vendor/usememos-mui/dist || true
RUN ls -la node_modules/@usememos/mui || true
RUN ls -la node_modules/@usememos/mui/dist || true
RUN node -e "console.log(require.resolve('@usememos/mui/package.json'))"
RUN test -f node_modules/@usememos/mui/dist/index.css
```

Then rebuild.

Also inspect `.dockerignore`:

```bash id="xsv4tk"
rg -n "dist|vendor|node_modules|web/vendor|usememos" .dockerignore
```

## Likely fix

If `.dockerignore` has a broad rule like:

```text id="cz8q67"
dist
**/dist
```

then add an exception for the vendored package:

```dockerignore id="ijq5a9"
!web/vendor/
!web/vendor/usememos-mui/
!web/vendor/usememos-mui/dist/
!web/vendor/usememos-mui/dist/**
```

If `.dockerignore` excludes vendor broadly, also add:

```dockerignore id="gcf6uv"
!web/vendor/**
```

Also consider excluding frontend `node_modules` from Docker context if it is currently being sent:

```dockerignore id="nwyvps"
web/node_modules
```

But do that only after confirming the Docker build still succeeds with clean `npm ci`.

## Gemini rework prompt

```text id="xq1r7a"
We need a narrow rework for the bchat frontend dependency provenance repair.

Repo:
/home/chaschel/Documents/go/bchat

Current state:
- Original Fly/Docker failure at `RUN npm ci` was fixed.
- Docker now passes `[frontend 5/12] RUN npm ci`.
- New failure occurs at `[frontend 7/12] RUN npm run release`.
- Vite error:
  `[vite]: Rollup failed to resolve import "@usememos/mui/dist/index.css" from "/frontend-build/src/main.tsx".`

Important:
This means the package is installed enough for npm ci, but the deep CSS import is not resolvable inside the Docker image.

Approved scope:
Diagnose and fix only the Docker/container visibility of the vendored @usememos/mui package files. Do not upgrade dependencies. Do not replace npm ci. Do not refactor frontend imports unless the actual package metadata proves that the import path is invalid, which is unlikely because local npm run release already passed.

Hypotheses to test:
1. `.dockerignore` excludes `dist/` or `**/dist`, so `web/vendor/usememos-mui/dist/index.css` exists locally but is missing from the Docker build context.
2. `.dockerignore` excludes part of `web/vendor`.
3. `COPY web/ .` after `npm ci` overwrites or changes the previously copied vendor directory.
4. The Docker build context is accidentally including `web/node_modules`, causing host node_modules to overwrite container-installed dependencies. The 2.07GB build context suggests this may be happening.

Required diagnostics:
1. Inspect `.dockerignore` for patterns affecting:
   - web/vendor
   - vendor
   - dist
   - **/dist
   - node_modules
   - web/node_modules
2. Add temporary Dockerfile diagnostics immediately before `RUN npm run release`, or use an equivalent debug build, to prove:
   - `vendor/usememos-mui/package.json` exists
   - `vendor/usememos-mui/dist/index.css` exists
   - `node_modules/@usememos/mui/package.json` resolves
   - `node_modules/@usememos/mui/dist/index.css` exists
3. Determine whether the file is missing from the build context, missing after `npm ci`, or removed/overwritten after `COPY web/ .`.

Expected likely fix:
If `.dockerignore` excludes dist broadly, add explicit exceptions such as:
`!web/vendor/`
`!web/vendor/usememos-mui/`
`!web/vendor/usememos-mui/dist/`
`!web/vendor/usememos-mui/dist/**`

If `web/node_modules` is entering the Docker context, consider adding:
`web/node_modules`
but only if the clean Docker npm ci path still succeeds afterward.

Non-negotiables:
- Keep Docker using npm ci.
- Keep @usememos/mui as `file:vendor/usememos-mui`.
- Preserve the vendored package files.
- Do not upgrade npm, Vite, React, MUI, or @usememos/mui.
- Do not refactor source imports as a workaround.
- Keep the fix limited to Docker context / Dockerfile / .dockerignore unless evidence proves otherwise.

Required validation:
1. `cd web && npm ci`
2. `cd web && npm run release`
3. `docker build -f Dockerfile.fly -t bchat:rag .`
4. `docker build -f Dockerfile.local.fly -t bchat:local-rag .` if practical
5. Confirm Docker build no longer fails on:
   `@usememos/mui/dist/index.css`
6. Confirm package-lock still contains no:
   `mui-0.1.0-20250601165716.tgz`
   `registry.npmjs.org/@usememos/mui`

Implementation summary must include:
- Exact root cause
- Whether `.dockerignore`, Dockerfile copy order, or node_modules context pollution caused the issue
- Files changed
- Proof that `vendor/usememos-mui/dist/index.css` exists inside Docker before release build
- Commands run and results
- Scope-control statement
```

## My read

This is not a reason to abandon vendoring. It is a reason to tighten the Docker-context side of the repair. The key proof needed now is simple: inside the frontend build container, before `npm run release`, does this file exist?

```text id="s6zguz"
node_modules/@usememos/mui/dist/index.css
```

If not, fix the Docker context or copy semantics until it does.

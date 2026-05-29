## Verdict: **APPROVE — Docker proof succeeded**

The local Docker build now completed successfully:

```text
[+] Building 174.8s (45/45) FINISHED
```

The proof build passed the exact previously failing boundary:

```text
[frontend  5/18] RUN npm ci
[frontend  6/18] COPY web/ .
[frontend 12/18] RUN test -f node_modules/@usememos/mui/dist/index.css
[frontend 13/18] RUN npm run release
```

So the original `npm ci` provenance failure is fixed, and the later Vite `@usememos/mui/dist/index.css` resolution failure is also fixed. The Docker context also shrank from **2.07GB** in the earlier failed build to **659.21kB**, confirming the `.dockerignore` state is now much healthier.  

## Final acceptance

**INV_FRONTEND_DEPENDENCY_PROVENANCE is satisfied.**

Confirmed:

* Docker still uses `npm ci`.
* `@usememos/mui` resolves from the vendored local file dependency.
* The vendored `dist/index.css` is visible inside Docker after `COPY web/ .`.
* `npm run release` succeeds inside Docker.
* Backend build succeeds with `-tags rag`.
* Final image exports successfully as `docker.io/library/bchat:rag`.

## Required cleanup before final production commit/deploy

Now remove the temporary diagnostic noise from both Dockerfiles:

```dockerfile
RUN find vendor/usememos-mui -maxdepth 3 -type f -print | sort
RUN ls -la vendor/usememos-mui/dist || true
RUN ls -la node_modules/@usememos/mui || true
RUN ls -la node_modules/@usememos/mui/dist || true
RUN node -e "console.log(require.resolve('@usememos/mui/package.json'))"
```

Keep this one narrow assertion if you want Docker to enforce the recovered invariant:

```dockerfile
RUN test -f node_modules/@usememos/mui/dist/index.css
```

Final preferred frontend stage:

```dockerfile
COPY web/package*.json ./
COPY web/vendor ./vendor
RUN npm ci
COPY web/ .
RUN test -f node_modules/@usememos/mui/dist/index.css
RUN npm run release
```

Or remove the `RUN test -f` too if you want a completely clean Dockerfile.

## Cleanup prompt for Gemini

```text
Clean up the temporary Docker diagnostics after the successful Docker proof build.

Repo:
/home/chaschel/Documents/go/bchat

Proof:
docker build -f Dockerfile.fly -t bchat:rag . completed successfully.
The build passed:
- RUN npm ci
- COPY web/ .
- RUN test -f node_modules/@usememos/mui/dist/index.css
- RUN npm run release
- backend go build -tags rag
- final image export

Task:
Remove debug print noise from both Dockerfiles.

Files:
- Dockerfile.fly
- Dockerfile.local.fly

Remove these temporary diagnostic lines from both Dockerfiles:
- RUN find vendor/usememos-mui -maxdepth 3 -type f -print | sort
- RUN ls -la vendor/usememos-mui/dist || true
- RUN ls -la node_modules/@usememos/mui || true
- RUN ls -la node_modules/@usememos/mui/dist || true
- RUN node -e "console.log(require.resolve('@usememos/mui/package.json'))"

Preferred final state:
Keep exactly one narrow assertion before npm run release:
RUN test -f node_modules/@usememos/mui/dist/index.css

Do not change:
- .dockerignore exceptions
- web/package.json
- web/package-lock.json
- web/vendor/usememos-mui/*
- npm ci
- frontend imports
- dependency versions

Validation:
- Confirm both Dockerfiles still copy web/vendor before npm ci.
- Confirm both Dockerfiles still use npm ci.
- Confirm both Dockerfiles no longer contain RUN find, RUN ls, or require.resolve debug diagnostics.
- Confirm .dockerignore still includes web/vendor/usememos-mui/dist/** at the bottom.
- If available, rerun:
  docker build -f Dockerfile.fly -t bchat:rag .

Summary must include:
- Files changed
- Confirmation debug print diagnostics were removed
- Confirmation whether the single RUN test assertion was retained
- Confirmation no dependency/source/config changes were made outside Dockerfile cleanup
```

After that cleanup, this is deploy-ready.

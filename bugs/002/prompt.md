**Verdict: REWORK / do not deploy yet.**

The Fly deploy is failing at Docker build step **`frontend 4/11 RUN npm ci`**, not at Fly runtime. The blocker is this missing package tarball:

```text
@usememos/mui@https://registry.npmjs.org/@usememos/mui/-/mui-0.1.0-20250601165716.tgz
```

npm returns:

```text
404 Not Found
'@usememos/mui@https://registry.npmjs.org/@usememos/mui/-/mui-0.1.0-20250601165716.tgz' is not in this registry.
```

That means your lockfile or package metadata points to a tarball URL that the npm registry no longer serves, so `npm ci` cannot reproduce the frontend install. The repeated deploy confirms this is deterministic, not a transient Fly/Depot issue. 

The package itself appears to be a real Memos UI package/workspace, but the specific timestamped tarball in your lockfile is not available from npm. The upstream Memos project is active, with recent releases such as `v0.28.0` in April 2026, so this looks like a dependency pin / lockfile provenance problem rather than a Fly problem. ([GitHub][1])

## Root cause

Your Dockerfile uses:

```dockerfile
COPY web/package*.json ./
RUN npm ci
```

`npm ci` is intentionally strict: it installs exactly what `package-lock.json` says. Here, the lockfile requires an exact tarball URL that is unavailable, so Docker cannot build.

## Recovered invariant

**INV_FRONTEND_DEPENDENCY_PROVENANCE:** every frontend dependency referenced by `package-lock.json` must resolve from a durable source during clean Docker builds. A lockfile must not point to unpublished, deleted, private, or timestamped tarballs unless the build also provides a durable vendored fallback.

## Immediate unblock

From repo root:

```bash
cd /home/chaschel/Documents/go/bchat

grep -R "@usememos/mui" -n web/package.json web/package-lock.json
```

Then inspect whether `web/package.json` pins `@usememos/mui` directly or whether only `package-lock.json` contains the bad tarball.

### Case A — `web/package.json` contains the bad tarball URL

Replace it with a durable version or Git source. For example, use the published package version if available:

```bash
cd web
npm view @usememos/mui versions --json
```

Then choose a valid version and update:

```bash
npm install @usememos/mui@<valid-version>
npm ci
```

Commit the changed `web/package.json` and `web/package-lock.json`.

### Case B — only `web/package-lock.json` contains the bad tarball URL

Regenerate the lockfile from the declared package constraints:

```bash
cd web
rm -rf node_modules package-lock.json
npm install
npm ci
```

Then redeploy:

```bash
cd ..
fly deploy
```

## What not to do

Do **not** “fix” this by changing Dockerfile from `npm ci` to `npm install` as the main solution. That hides the reproducibility failure. `npm install` may work locally by rewriting the lockfile, but production builds should remain deterministic.

Do **not** assume updating npm fixes this. The log’s npm version notice is unrelated. The real failure is a registry 404 for a package tarball.

## Gemini prompt

Use this:

```text
We are fixing a Fly deploy failure in repo /home/chaschel/Documents/go/bchat.

Observed failure:
- `fly deploy` fails during Docker build.
- Failing step: `[frontend 4/11] RUN npm ci`
- Error:
  `404 Not Found - GET https://registry.npmjs.org/@usememos/mui/-/mui-0.1.0-20250601165716.tgz`
  `'@usememos/mui@https://registry.npmjs.org/@usememos/mui/-/mui-0.1.0-20250601165716.tgz' is not in this registry.`

Task:
Diagnose and fix the frontend dependency provenance issue without weakening deterministic builds.

Required workflow:
1. Inspect:
   - `Dockerfile.local.fly`
   - `web/package.json`
   - `web/package-lock.json`
   - any `.npmrc`, workspace, vendoring, or patch-package configuration
2. Determine whether the bad `@usememos/mui` tarball URL is declared directly in `web/package.json` or only locked in `web/package-lock.json`.
3. Identify the correct durable dependency source:
   - a valid npm-published `@usememos/mui` version, or
   - a durable Git reference/tag/commit, or
   - a local vendored package if this repo intentionally carries a fork.
4. Preserve `npm ci` in Docker unless there is a clearly justified architecture reason not to.
5. Regenerate only the necessary frontend dependency files.
6. Run:
   - `cd web && npm ci`
   - `cd .. && docker build -f Dockerfile.local.fly .` or the closest local equivalent
   - `fly deploy` only if local build succeeds, or report the exact blocker.

Root-cause requirement:
Do not patch the symptom by switching Docker to `npm install`.
Recover and enforce this invariant:
`INV_FRONTEND_DEPENDENCY_PROVENANCE`: every frontend dependency referenced by the lockfile must resolve from a durable source during clean Docker builds.

Implementation summary must include:
- root cause
- exact files changed
- whether `package.json` or only `package-lock.json` was wrong
- dependency source chosen and why it is durable
- commands run and results
- whether Docker still uses `npm ci`
```

This is likely a small lockfile/dependency-source repair, not a Fly infrastructure issue.

[1]: https://github.com/usememos/mui?utm_source=chatgpt.com "usememos/mui"

## A. Verdict: **APPROVE WITH NITS**

## B. Files Reviewed
- `.dockerignore` (lines 1877-1884)
- `Dockerfile.fly` (lines 11-21)
- `Dockerfile.local.fly` (lines 11-21)
- `web/package.json` (line 21)
- `web/package-lock.json` (lines 18, 4791-4793, 12374-12399)
- `web/vendor/usememos-mui/package.json`
- `web/vendor/usememos-mui/LICENSE`
- `web/vendor/usememos-mui/dist/index.css`
- `web/vendor/usememos-mui/dist/index.mjs`

## C. Summary of Actual Changes
1. **`.dockerignore`**: Added 7 exception rules at the file bottom (lines 1877-1884) to make vendored `@usememos/mui/dist/` visible to Docker context
2. **`Dockerfile.fly` / `Dockerfile.local.fly`**: Added `COPY web/vendor ./vendor` before `npm ci`, plus 6 diagnostic RUN lines after `COPY web/ .`
3. **`web/package.json`**: Changed `@usememos/mui` from `0.1.0-20250601165716` to `file:vendor/usememos-mui`
4. **`web/package-lock.json`**: Updated to reflect local file reference (resolved: `vendor/usememos-mui`)

## D. Root-cause Verification
✓ The `**/dist` rule at line 11 excluded `web/vendor/usememos-mui/dist/`
✓ Exception rules are correctly placed AFTER all `**/dist` rules
✓ The assertion `test -f node_modules/@usememos/mui/dist/index.css` validates Vite can resolve the import

## E. Invariant Verification
1. ✓ Vendored files exist in repository (package.json, LICENSE, README.md, dist/*)
2. ✓ Docker uses `npm ci`
3. ✓ `@usememos/mui` is local file dependency
4. ✓ No `mui-0.1.0-20250601165716.tgz` or `registry.npmjs.org/@usememos/mui` in package-lock.json
5. ✓ No dependency upgrades performed

## F. Scope-control Findings
The diagnostic RUN lines (6 per Dockerfile) are present but not clearly marked as temporary. Per the approved scope: "diagnostic RUN find / RUN ls / RUN test lines are acceptable for the proof build, but final production Dockerfiles should remove debug print noise unless intentionally retained as a narrow assertion."

## G. Test Evidence Reviewed
- Local `npm ci` passed
- Local `npm run release` passed
- Local Docker build NOT completed (no socket access)

## H. Blocking Issues
None. All invariants satisfied.

## I. Final Recommendation
The `.dockerignore` fix is technically correct. However, the 6 diagnostic RUN lines in both Dockerfiles should either:
- Have a clear comment indicating they are temporary and for proof build only, OR
- Be removed and replaced with a single assertion like the `test -f` line

Recommend a Fly/Helm/Docker remote build to verify Docker success before final production deployment.
**A. Verdict: APPROVE**

**B. Files reviewed:**
- web/package.json
- web/package-lock.json  
- web/vendor/usememos-mui/package.json, LICENSE, README.md, dist/index.css, dist/index.d.mts, dist/index.mjs
- Dockerfile.local.fly, Dockerfile.fly
- .dockerignore

**C. Summary of actual changes:**
- web/package.json: Changed `@usememos/mui` from `"0.1.0-20250601165716"` to `"file:vendor/usememos-mui"`
- web/package-lock.json: Removed registry.npmjs.org/@usememos/mui entry, replaced with `"resolved": "vendor/usememos-mui", "link": true`
- Dockerfile.local.fly, Dockerfile.fly: Added `COPY web/vendor ./vendor` before `RUN npm ci`

**D. Invariant verification:**
1. ✓ Docker still uses `npm ci` (not replaced with npm install)
2. ✓ Dockerfiles copy `web/vendor` before `RUN npm ci`
3. ✓ web/package.json declares `"@usememos/mui": "file:vendor/usememos-mui"`
4. ✓ package-lock.json contains neither `mui-0.1.0-20250601165716.tgz` nor `registry.npmjs.org/@usememos/mui` references
5. ✓ package-lock.json points to local file/vendor source with `"resolved": "vendor/usememos-mui", "link": true`
6. ✓ Vendored package has correct identity: name `@usememos/mui`, version `0.1.0-20250601165716`
7. ✓ Vendored package includes all required files (package.json, LICENSE, README.md, dist/*)
8. ✓ No unrelated changes - only the minimal scope needed

**E. Scope-control findings:**
- Only 4 files modified, 1 vendor directory added
- No broad lockfile churn beyond the @usememos/mui entry
- No npm/node version changes
- No other dependency upgrades

**F. Test evidence reviewed:**
- `diff -qr` returned zero differences (vendored package matches original)
- `npm ci` passed successfully
- `npm run release` completed successfully

**G. Blocking issues:** None
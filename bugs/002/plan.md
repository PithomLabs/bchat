# Repair Frontend Dependency Provenance

  ## Summary

  Fix the Fly build failure by vendoring the exact installed @usememos/mui@0.1.0-20250601165716 package from web/node_modules, then point
  web/package.json and web/package-lock.json at that local source. Keep Docker on npm ci.

  Root cause: web/package.json:21 directly pins an npm version whose registry metadata exists, but whose tarball 404s. This is not just a
  stale lockfile.

  ## Key Changes

  - Add web/vendor/usememos-mui/ containing only the runtime package files:
    package.json, LICENSE, README.md, and dist/*.
  - Change @usememos/mui in web/package.json to:
    file:vendor/usememos-mui
  - Regenerate web/package-lock.json so the lock resolves @usememos/mui from file:vendor/usememos-mui, not
    https://registry.npmjs.org/...tgz.
  - Update Dockerfile.local.fly and Dockerfile.fly frontend stages:
      - keep RUN npm ci
      - copy web/vendor before RUN npm ci, because file dependencies must exist before install.
  - Do not change npm ci to npm install.

  ## Test Plan

  - Run cd web && npm ci.
  - Run cd web && npm run release.
  - Run docker build -f Dockerfile.local.fly ..
  - Run fly deploy only after the local Docker build succeeds.
  - Verify rg "@usememos/mui/-/mui-0.1.0-20250601165716.tgz" web/package-lock.json returns no matches.

  ## Assumptions

  - Vendoring is chosen because it preserves the exact current package API, including Textarea and dist/index.css.
  - The currently reachable npm canary has a changed public surface, so upgrading it is higher risk than vendoring for this deploy unblock.
  - .dockerignore excludes *.tgz, so vendoring as a directory is intentional.
  - External references checked: [npm package page](httpsTitle: Repair Frontend Dependency Provenance for Fly Build

  Summary

  - Root cause: @usememos/mui is declared in web/package.json:21 as 0.1.0-20250601165716, and web/package-lock.json:4500 locks an npm
    tarball URL that now returns 404.
  - Use a vendored exact copy of the currently installed package from web/node_modules/@usememos/mui, preserving the existing API and dist/
    index.css.
  - Keep Docker using npm ci; do not switch to npm install.

  Key Changes

  - Add web/vendor/usememos-mui/ containing only the runtime package files:
    package.json, LICENSE, README.md, and dist/*.
  - Change web/package.json dependency to:

    "@usememos/mui": "file:vendor/usememos-mui"

  - Regenerate web/package-lock.json so @usememos/mui resolves from file:vendor/usememos-mui, not https://registry.npmjs.org/....
  - Update Dockerfile.local.fly:11 before RUN npm ci:

    COPY web/package*.json ./
    COPY web/vendor ./vendor
    RUN npm ci

  - Apply the same vendor-copy fix to Dockerfile.fly:9 so the alternate Fly Dockerfile does not retain the same failure mode.

  Test Plan

  - Run:

    cd web && npm install --package-lock-only --ignore-scripts
    npm ci

  - Confirm:

    rg -n "mui-0.1.0-20250601165716.tgz|registry.npmjs.org/@usememos/mui" package-lock.json
    returns no matches.

  - From repo root, run:

    docker build -f Dockerfile.local.fly .

  - Run fly deploy only after the local Docker build succeeds.

  Assumptions

  - Vendoring is preferred over upgrading because the reachable 2026 npm canary has a changed package surface, while the local package
    exports the currently used Button, Checkbox, Input, Switch, Textarea, and dist/index.css.
  - .dockerignore excludes **/*.tgz, so vendoring as a directory is intentional.
  - Source references checked: npm package registry (https://www.npmjs.com/package/@usememos/mui) and usememos/mui GitHub package
    (https://github.com/usememos/mui).

# Implementation Plan - Frontend Dependency Provenance Repair

This plan details the steps to repair the frontend dependency provenance issue unblocking Fly deployment. The registry tarball for `@usememos/mui@0.1.0-20250601165716` is returning 404, causing `npm ci` to fail.

## Proposed Strategy

We will vendor the exact currently-installed `@usememos/mui` dependency from `web/node_modules/@usememos/mui` under `web/vendor/usememos-mui`, configure `web/package.json` to install it as a `file:` dependency, and update both Fly Dockerfiles to copy this vendor directory before running `npm ci`. This ensures deterministic builds via `npm ci` without switching to `npm install` or upgrading unrelated frontend dependencies.

---

## Provenance Verification

We successfully verified the identity of the locally installed package:
```bash
$ node -p "require('./web/node_modules/@usememos/mui/package.json').name"
@usememos/mui
$ node -p "require('./web/node_modules/@usememos/mui/package.json').version"
0.1.0-20250601165716
```

We also inspected `web/node_modules/@usememos/mui/package.json` and mapped all of its referenced metadata files to verify completeness. The package relies on the following files:
* `package.json`
* `LICENSE`
* `README.md`
* `dist/index.css`
* `dist/index.d.mts`
* `dist/index.mjs`

Every one of these files exists locally and will be completely copied into the vendored destination to prevent any typecheck or runtime import failures.

---

## Proposed Changes

### Component: Frontend Dependency

#### [NEW] [usememos-mui](file:///home/chaschel/Documents/go/bchat/web/vendor/usememos-mui)
Create directory `web/vendor/usememos-mui` and copy the verified package files:
* `web/vendor/usememos-mui/package.json`
* `web/vendor/usememos-mui/LICENSE`
* `web/vendor/usememos-mui/README.md`
* `web/vendor/usememos-mui/dist/index.css`
* `web/vendor/usememos-mui/dist/index.d.mts`
* `web/vendor/usememos-mui/dist/index.mjs`

#### [MODIFY] [package.json](file:///home/chaschel/Documents/go/bchat/web/package.json)
Update `@usememos/mui` dependency version reference:
```json
-    "@usememos/mui": "0.1.0-20250601165716",
+    "@usememos/mui": "file:vendor/usememos-mui",
```

---

### Component: Fly Dockerfiles

#### [MODIFY] [Dockerfile.local.fly](file:///home/chaschel/Documents/go/bchat/Dockerfile.local.fly)
Update the frontend build stage so `web/vendor` is copied before `npm ci` is executed, keeping the `npm ci` build contract:
```dockerfile
 FROM node:20-alpine AS frontend
 WORKDIR /frontend-build
 COPY web/package*.json ./
+COPY web/vendor ./vendor
 RUN npm ci
```

#### [MODIFY] [Dockerfile.fly](file:///home/chaschel/Documents/go/bchat/Dockerfile.fly)
Apply the identical vendoring copy statement to the alternate Fly Dockerfile:
```dockerfile
 FROM node:20-alpine AS frontend
 WORKDIR /frontend-build
 COPY web/package*.json ./
+COPY web/vendor ./vendor
 RUN npm ci
```

---

## Verification Plan

### Automated Verification

From `web/` directory:
1. **Regenerate lockfile:**
   ```bash
   npm install --package-lock-only --ignore-scripts
   ```
2. **Deterministic Install:**
   ```bash
   npm ci
   ```
3. **Frontend compilation / packaging check:**
   ```bash
   npm run release
   ```
4. **Lockfile search validation:**
   ```bash
   rg -n "mui-0.1.0-20250601165716.tgz|registry.npmjs.org/@usememos/mui" package-lock.json
   ```
   *Expected:* No matches returned.
5. **Package declaration validation:**
   ```bash
   rg -n '"@usememos/mui": "file:vendor/usememos-mui"' package.json
   ```
   *Expected:* Exactly one match.

From repo root:
6. **Docker Build Validation (local fly):**
   ```bash
   docker build -f Dockerfile.local.fly .
   ```
   *Expected:* Complete and successful Docker image build.
7. **Docker Build Validation (production fly):**
   ```bash
   docker build -f Dockerfile.fly .
   ```
   *Expected:* Complete and successful Docker image build.

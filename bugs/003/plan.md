# Rework Implementation Plan - Frontend Dependency Provenance Repair (Docker Context Visibility)

Fly deployment now installs dependencies successfully during `npm ci`, but fails at `npm run release` because Vite cannot resolve `@usememos/mui/dist/index.css`. This plan diagnoses and resolves the Docker context visibility of the vendored `@usememos/mui` assets.

## Identified Root Cause

* **`.dockerignore` Exclusion:** In `.dockerignore` on line 11, there is a global exclusion rule:
  ```text
  **/dist
  ```
  This recursively ignores all folders named `dist` in the repository from entering the Docker build context. As a result, `web/vendor/usememos-mui/dist/` is completely excluded from the Docker build container, leaving the vendored package empty of its required assets (like `dist/index.css` and `dist/index.mjs`) inside Docker, even though they exist locally.

* **Build Context size:** The large Docker build context transfer size (2.07GB) indicates that some large untracked local directories (e.g. `build/data` or massive directories not in the workspace) may be incorrectly entering the Docker context. However, `**/node_modules` is already correctly excluded, and our focus is to narrowly resolve the missing `dist` files for the vendored package.

---

## Recovered Invariant Refinement

> **INV_FRONTEND_DEPENDENCY_PROVENANCE:** Vendored frontend dependencies must be present both in the repository and in the Docker build context, including all files referenced by runtime imports, package metadata, and deep imports such as CSS paths.

---

## Proposed Changes

### 1. Component: Docker Build Context Configuration

#### [MODIFY] [.dockerignore](file:///home/chaschel/Documents/go/bchat/.dockerignore)
Add explicit negation rules in `.dockerignore` to allow the vendored package's `dist/` directory to bypass the global `**/dist` exclusion:

```dockerignore
# Add exceptions to allow vendored usememos-mui files to enter the build context
!web/vendor/
!web/vendor/usememos-mui/
!web/vendor/usememos-mui/dist/
!web/vendor/usememos-mui/dist/**
```

---

### 2. Component: Fly Dockerfiles (Diagnostic Phase)

#### [MODIFY] [Dockerfile.local.fly](file:///home/chaschel/Documents/go/bchat/Dockerfile.local.fly)
Add temporary diagnostic print statements in the frontend stage right before `RUN npm run release` to prove the files exist and log their exact structure in the container build log:

```dockerfile
 COPY web/vendor ./vendor
 RUN npm ci
+RUN find vendor/usememos-mui -maxdepth 3 -type f -print | sort
+RUN ls -la vendor/usememos-mui/dist || true
+RUN ls -la node_modules/@usememos/mui/dist || true
 COPY web/ .
 RUN npm run release
```

#### [MODIFY] [Dockerfile.fly](file:///home/chaschel/Documents/go/bchat/Dockerfile.fly)
Apply the identical temporary diagnostic print statements to `Dockerfile.fly`:

```dockerfile
 COPY web/vendor ./vendor
 RUN npm ci
+RUN find vendor/usememos-mui -maxdepth 3 -type f -print | sort
+RUN ls -la vendor/usememos-mui/dist || true
+RUN ls -la node_modules/@usememos/mui/dist || true
 COPY web/ .
 RUN npm run release
```

---

## Verification Plan

### Automated Validation

From `web/` directory:
1. **Deterministic local install:**
   ```bash
   npm ci
   ```
2. **Deterministic local release build:**
   ```bash
   npm run release
   ```
3. **Lockfile validation:**
   Ensure no registry URLs or unavailable `.tgz` archives exist in `web/package-lock.json`.

From repo root:
4. **Deploy Validation:**
   Run Fly deploy (or local docker build if the docker daemon/CLI is available on host) to verify that:
   - The diagnostic `RUN find` output prints all 6 vendored package files.
   - `RUN npm run release` finishes with 100% success.

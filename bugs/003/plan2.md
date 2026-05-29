# Revised Rework Implementation Plan - Frontend Dependency Provenance Repair (Docker Context Visibility)

Vite compilation inside Docker fails at `RUN npm run release` because `COPY web/ .` overwrites the good container `vendor` folder with a version from the Docker build context that is missing its `dist` directory due to a `.dockerignore` ignore rule (`**/dist`).

This plan implements the precise exceptions in `.dockerignore` to allow our vendored assets through, verifies it using temporary diagnostics placed at the exact failure boundary (after `COPY web/ .`), and ensures clean production Dockerfiles for the final state.

---

## Recovered Invariant Refinement

> **INV_FRONTEND_DEPENDENCY_PROVENANCE:** Vendored frontend dependencies must be present both in the repository and in the Docker build context, including all files referenced by runtime imports, package metadata, and deep imports such as CSS paths.

---

## Proposed Changes

### 1. Component: Docker Build Context Configuration

#### [MODIFY] [.dockerignore](file:///home/chaschel/Documents/go/bchat/.dockerignore)
Add exact exceptions to the top of `.dockerignore` so all files and deep import assets for our vendored `@usememos/mui` package can safely bypass the global `**/dist` ignore rule and enter the build context:

```dockerignore
# Add exceptions to allow vendored usememos-mui files to enter the build context
!web/vendor/
!web/vendor/usememos-mui/
!web/vendor/usememos-mui/package.json
!web/vendor/usememos-mui/LICENSE
!web/vendor/usememos-mui/README.md
!web/vendor/usememos-mui/dist/
!web/vendor/usememos-mui/dist/**
```

---

### 2. Component: Fly Dockerfiles (Diagnostic & Final State)

#### Step A: Diagnostic Phase
Add temporary diagnostic print and test statements in both `Dockerfile.local.fly` and `Dockerfile.fly` **after** `COPY web/ .` and **before** `RUN npm run release`. This is the exact failure boundary, ensuring we prove the file exists in the container even after the workspace files are copied.

* **[MODIFY] [Dockerfile.local.fly](file:///home/chaschel/Documents/go/bchat/Dockerfile.local.fly):**
  ```dockerfile
   COPY web/ .
  +RUN find vendor/usememos-mui -maxdepth 3 -type f -print | sort
  +RUN ls -la vendor/usememos-mui/dist || true
  +RUN ls -la node_modules/@usememos/mui || true
  +RUN ls -la node_modules/@usememos/mui/dist || true
  +RUN node -e "console.log(require.resolve('@usememos/mui/package.json'))"
  +RUN test -f node_modules/@usememos/mui/dist/index.css
   RUN npm run release
  ```

* **[MODIFY] [Dockerfile.fly](file:///home/chaschel/Documents/go/bchat/Dockerfile.fly):**
  ```dockerfile
   COPY web/ .
  +RUN find vendor/usememos-mui -maxdepth 3 -type f -print | sort
  +RUN ls -la vendor/usememos-mui/dist || true
  +RUN ls -la node_modules/@usememos/mui || true
  +RUN ls -la node_modules/@usememos/mui/dist || true
  +RUN node -e "console.log(require.resolve('@usememos/mui/package.json'))"
  +RUN test -f node_modules/@usememos/mui/dist/index.css
   RUN npm run release
  ```

#### Step B: Final State (Clean Up)
Once the diagnostics prove the file visibility in Docker, we will clean up the Dockerfiles to remove these temporary assertions, ensuring our final configuration has no debug noise:

```dockerfile
COPY web/package*.json ./
COPY web/vendor ./vendor
RUN npm ci
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

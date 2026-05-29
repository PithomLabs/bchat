## Verdict: **APPROVE**

This revised plan now satisfies the dependency-provenance invariant. The earlier Fly failure was deterministic at `npm ci`, caused by the unavailable `@usememos/mui` registry tarball, not by Fly runtime or Depot itself. 

## What is good

The plan now closes the main gap I flagged earlier: it verifies that the local installed package is exactly:

```text
@usememos/mui
0.1.0-20250601165716
```

That makes vendoring defensible instead of blindly copying whatever happened to be in `node_modules`.

The metadata completeness check is also good. Since the package only requires:

```text
package.json
LICENSE
README.md
dist/index.css
dist/index.d.mts
dist/index.mjs
```

copying those exact files is enough to preserve runtime imports, CSS imports, and TypeScript declarations.

Keeping `npm ci` is correct. This remains a reproducibility repair, not a build-contract downgrade.

Updating both `Dockerfile.local.fly` and `Dockerfile.fly` is also correct. Otherwise the alternate Fly build path would retain the same failure mode.

## Root-cause / generalization check

This does solve the underlying class for this dependency: the build no longer depends on a registry tarball that can 404. It restores a durable install source that is included in the repo and available before `npm ci`.

The recovered invariant is satisfied:

> Frontend dependencies used by clean Docker builds must resolve from durable, reviewable sources under `npm ci`; if an upstream package tarball is unavailable, the repair must either pin a durable upstream source or vendor a verified exact package copy.

This is not a symptom patch like replacing `npm ci` with `npm install`.

## Nits before implementation

Add one small validation after vendoring:

```bash
diff -qr web/node_modules/@usememos/mui web/vendor/usememos-mui
```

Because you intentionally copy only runtime package files, it is okay if extra irrelevant files differ or are omitted, but any difference among the six required files should be investigated.

Also, after regenerating the lockfile, check that the lockfile records the package as a local file dependency, not as a registry package:

```bash
rg -n '"node_modules/@usememos/mui"|"resolved": "file:vendor/usememos-mui"|"link": true' web/package-lock.json
```

The exact lockfile shape depends on npm’s lockfile version, but the important property is: no registry tarball for `@usememos/mui`.

## Required implementation summary

After implementation, ask Gemini to report:

```text
Root cause:
Vendored package identity:
Vendored package file list:
Files changed:
Dockerfile npm-ci status:
Lockfile provenance result:
Commands run and results:
Remaining follow-up:
```

## StepFun review prompt after Gemini implements

```text
Review the implemented frontend dependency provenance repair in repo /home/chaschel/Documents/go/bchat.

Approved change:
- Vendor exact local @usememos/mui@0.1.0-20250601165716 from web/node_modules/@usememos/mui into web/vendor/usememos-mui.
- Change web/package.json dependency to "file:vendor/usememos-mui".
- Regenerate web/package-lock.json so @usememos/mui no longer resolves from registry.npmjs.org.
- Update both Dockerfile.local.fly and Dockerfile.fly so web/vendor is copied before RUN npm ci.
- Preserve npm ci. Do not replace it with npm install in Docker.

Files to inspect:
- web/package.json
- web/package-lock.json
- web/vendor/usememos-mui/package.json
- web/vendor/usememos-mui/LICENSE
- web/vendor/usememos-mui/README.md
- web/vendor/usememos-mui/dist/index.css
- web/vendor/usememos-mui/dist/index.d.mts
- web/vendor/usememos-mui/dist/index.mjs
- Dockerfile.local.fly
- Dockerfile.fly
- .dockerignore, only to confirm no rule excludes web/vendor

Invariants to verify:
1. INV_FRONTEND_DEPENDENCY_PROVENANCE:
   Clean Docker frontend builds must not depend on the unavailable @usememos/mui registry tarball.
2. Docker must still use npm ci.
3. The vendored package must identify as:
   - name: @usememos/mui
   - version: 0.1.0-20250601165716
4. The vendored package must include all files referenced by its package metadata:
   - package.json
   - LICENSE
   - README.md
   - dist/index.css
   - dist/index.d.mts
   - dist/index.mjs
5. No unrelated frontend dependency upgrades, import refactors, or Docker strategy changes.

Required checks:
- Confirm web/package.json contains:
  "@usememos/mui": "file:vendor/usememos-mui"
- Confirm web/package-lock.json does not contain:
  "mui-0.1.0-20250601165716.tgz"
  "registry.npmjs.org/@usememos/mui"
- Confirm both Dockerfiles copy web/vendor before RUN npm ci.
- Confirm Dockerfiles did not replace npm ci with npm install.
- Confirm package-lock points @usememos/mui to a local file/vendor source.
- Review git diff for scope drift.

Expected test evidence:
- cd web && npm ci
- cd web && npm run release
- docker build -f Dockerfile.local.fly .
- docker build -f Dockerfile.fly . if practical

Output format:
A. Verdict: APPROVE / APPROVE WITH NITS / REWORK
B. Files reviewed
C. What changed
D. Invariant verification
E. Scope-control findings
F. Test evidence reviewed
G. Blocking issues, if any
H. Final recommendation
```

Once Gemini implements, do **not** treat the implementation summary alone as final approval. Send it through StepFun with the prompt above.

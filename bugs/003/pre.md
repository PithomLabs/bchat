chaschel@linux:~/Documents/go/bchat$ sudo docker build -f Dockerfile.fly -t bchat:rag .
[sudo] password for chaschel: 
sudo: a password is required
chaschel@linux:~/Documents/go/bchat$ sudo docker build -f Dockerfile.fly -t bchat:rag .
[sudo] password for chaschel:        
[+] Building 91.7s (18/38)                                                                                                                   docker:default
 => [internal] load build definition from Dockerfile.fly                                                                                               0.0s
 => => transferring dockerfile: 2.77kB                                                                                                                 0.0s
 => [internal] load metadata for docker.io/library/debian:bookworm-slim                                                                                2.1s
 => [internal] load metadata for docker.io/library/golang:1.24                                                                                         2.5s
 => [internal] load metadata for docker.io/library/node:20-alpine                                                                                      2.5s
 => [internal] load .dockerignore                                                                                                                      0.0s
 => => transferring context: 74.07kB                                                                                                                   0.0s
 => [internal] load build context                                                                                                                      8.6s
 => => transferring context: 2.07GB                                                                                                                    8.5s
 => [stage-2  1/10] FROM docker.io/library/debian:bookworm-slim@sha256:0104b334637a5f19aa9c983a91b54c89887c0984081f2068983107a6f6c21eeb               25.0s
 => => resolve docker.io/library/debian:bookworm-slim@sha256:0104b334637a5f19aa9c983a91b54c89887c0984081f2068983107a6f6c21eeb                          0.0s
 => => sha256:068fedd6b0f109b8186d00d49327b6fc6747c428fd3c9a8739424ff5f38d7531 28.23MB / 28.23MB                                                      24.4s
 => => extracting sha256:068fedd6b0f109b8186d00d49327b6fc6747c428fd3c9a8739424ff5f38d7531                                                              0.6s
 => [frontend  1/12] FROM docker.io/library/node:20-alpine@sha256:fb4cd12c85ee03686f6af5362a0b0d56d50c58a04632e6c0fb8363f609372293                    45.8s
 => => resolve docker.io/library/node:20-alpine@sha256:fb4cd12c85ee03686f6af5362a0b0d56d50c58a04632e6c0fb8363f609372293                                0.0s
 => => sha256:fff4e2c1b189bf87d63ad8bd07f7f4eb288d6f2b6a07a8bb44c60e8c075d2096 445B / 445B                                                             0.3s
 => => sha256:b2cbbfe903b0821005780971ddc5892edcc4ce74c5a48d82e1d2b382edac3122 1.26MB / 1.26MB                                                         2.3s
 => => sha256:4feea04c154301db6f4a496efa397b3db96603b1c009c797cfdde77bea8b3287 43.23MB / 43.23MB                                                      44.8s
 => => sha256:6a0ac1617861a677b045b7ff88545213ec31c0ff08763195a70a4a5adda577bb 3.86MB / 3.86MB                                                         9.0s
 => => extracting sha256:6a0ac1617861a677b045b7ff88545213ec31c0ff08763195a70a4a5adda577bb                                                              0.1s
 => => extracting sha256:4feea04c154301db6f4a496efa397b3db96603b1c009c797cfdde77bea8b3287                                                              0.7s
 => => extracting sha256:b2cbbfe903b0821005780971ddc5892edcc4ce74c5a48d82e1d2b382edac3122                                                              0.0s
 => => extracting sha256:fff4e2c1b189bf87d63ad8bd07f7f4eb288d6f2b6a07a8bb44c60e8c075d2096                                                              0.0s
 => [backend  1/10] FROM docker.io/library/golang:1.24@sha256:d2d2bc1c84f7e60d7d2438a3836ae7d0c847f4888464e7ec9ba3a1339a1ee804                        89.0s
 => => resolve docker.io/library/golang:1.24@sha256:d2d2bc1c84f7e60d7d2438a3836ae7d0c847f4888464e7ec9ba3a1339a1ee804                                   0.0s
 => => sha256:4f4fb700ef54461cfa02571ae0db9a0dc1e0cdb5577484a6d75e68dc38e8acc1 32B / 32B                                                               0.5s
 => => sha256:50a27cd32f8983e7e43777ec65195fb6594ea877f31993fd555513c261ffc054 126B / 126B                                                             0.4s
 => => sha256:f7bdfd728ac2ad72d43b82689890dc698260d3a1049845f48fb3fb942df6c581 55.57MB / 79.13MB                                                      85.7s
 => => sha256:b2b04fcbed4bf6e5373e2607d2705704ec5b220f1d1306e06ab8fe9471b2f86a 73.40MB / 102.14MB                                                     79.5s
 => => sha256:b5e2021c4c8bd1a46b34d9608a9381afdc333600ee1ef3c94306ecf7373e1956 33.55MB / 67.79MB                                                      64.7s 
 => => sha256:954d6059ca7bdbb9ceb566ca2239e01ef312165659d656753d7dbace7771a591 24.12MB / 25.61MB                                                      44.1s 
 => [stage-2  2/10] WORKDIR /usr/local/memos                                                                                                           0.3s 
 => [stage-2  3/10] RUN apt-get update && apt-get install -y     ca-certificates     tzdata     && rm -rf /var/lib/apt/lists/*                        11.9s
 => [frontend  2/12] WORKDIR /frontend-build                                                                                                           0.1s
 => [frontend  3/12] COPY web/package*.json ./                                                                                                         0.0s
 => [frontend  4/12] COPY web/vendor ./vendor                                                                                                          0.0s
 => [frontend  5/12] RUN npm ci                                                                                                                       41.2s
 => [frontend  6/12] COPY web/ .                                                                                                                       0.4s
 => ERROR [frontend  7/12] RUN npm run release                                                                                                         1.4s
 => CANCELED [backend  2/10] WORKDIR /backend-build                                                                                                    0.0s
------
 > [frontend  7/12] RUN npm run release:
0.300 
0.300 > release
0.300 > vite build --mode release --outDir=../server/router/frontend/dist --emptyOutDir
0.300 
1.017 vite v6.4.1 building for release...
1.293 transforming...
1.318 ✓ 4 modules transformed.
1.321 ✗ Build failed in 278ms
1.321 error during build:
1.321 [vite]: Rollup failed to resolve import "@usememos/mui/dist/index.css" from "/frontend-build/src/main.tsx".
1.321 This is most likely unintended because it can break your application at runtime.
1.321 If you do want to externalize this module explicitly add it to
1.321 `build.rollupOptions.external`
1.321     at viteLog (file:///frontend-build/node_modules/vite/dist/node/chunks/dep-D4NMHUTW.js:46374:15)
1.321     at file:///frontend-build/node_modules/vite/dist/node/chunks/dep-D4NMHUTW.js:46432:18
1.321     at onwarn (/frontend-build/node_modules/@vitejs/plugin-react/dist/index.cjs:112:7)
1.321     at file:///frontend-build/node_modules/vite/dist/node/chunks/dep-D4NMHUTW.js:46430:7
1.321     at onRollupLog (file:///frontend-build/node_modules/vite/dist/node/chunks/dep-D4NMHUTW.js:46422:5)
1.321     at onLog (file:///frontend-build/node_modules/vite/dist/node/chunks/dep-D4NMHUTW.js:46072:7)
1.321     at file:///frontend-build/node_modules/rollup/dist/es/shared/node-entry.js:21004:32
1.321     at Object.logger [as onLog] (file:///frontend-build/node_modules/rollup/dist/es/shared/node-entry.js:22891:9)
1.321     at ModuleLoader.handleInvalidResolvedId (file:///frontend-build/node_modules/rollup/dist/es/shared/node-entry.js:21635:26)
1.321     at file:///frontend-build/node_modules/rollup/dist/es/shared/node-entry.js:21593:26
------
WARNING: current commit information was not captured by the build: failed to read current commit information with git rev-parse --is-inside-work-tree
Dockerfile.fly:14
--------------------
  12 |     RUN npm ci
  13 |     COPY web/ .
  14 | >>> RUN npm run release
  15 |     
  16 |     # Build widget
--------------------
ERROR: failed to build: failed to solve: process "/bin/sh -c npm run release" did not complete successfully: exit code: 1

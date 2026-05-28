chaschel@linux:~/Documents/go/bchat$ fly deploy
==> Verifying app config
Validating /home/chaschel/Documents/go/bchat/fly.toml
✓ Configuration is valid
--> Verified app config
==> Building image
==> Building image with Depot
--> build:  (​)
[+] Building 57.6s (21/37)                                                      
 => [internal] load build definition from Dockerfile.local.fly             0.2s
 => => transferring dockerfile: 3.17kB                                     0.2s
 => [internal] load metadata for docker.io/library/ubuntu:24.04            1.8s
 => [internal] load metadata for docker.io/library/node:20-alpine          1.7s
 => [internal] load metadata for docker.io/library/golang:1.24             2.0s
 => [internal] load .dockerignore                                          0.3s
 => => transferring context: 74.07kB                                       0.2s
 => [internal] load build context                                          1.7s
 => => transferring context: 440.17kB                                      1.7s
 => [backend  1/10] FROM docker.io/library/golang:1.24@sha256:d2d2bc1c84f  0.0s
 => => resolve docker.io/library/golang:1.24@sha256:d2d2bc1c84f7e60d7d243  0.0s
 => [frontend  1/11] FROM docker.io/library/node:20-alpine@sha256:fb4cd12  0.0s
 => => resolve docker.io/library/node:20-alpine@sha256:fb4cd12c85ee03686f  0.0s
 => [stage-2  1/10] FROM docker.io/library/ubuntu:24.04@sha256:c4a8d5503d  0.0s
 => => resolve docker.io/library/ubuntu:24.04@sha256:c4a8d5503dfb2a3eb8ab  0.0s
 => CACHED [stage-2  2/10] WORKDIR /usr/local/memos                        0.0s
 => CACHED [stage-2  3/10] RUN apt-get update && apt-get install -y     c  0.0s
 => CACHED [frontend  2/11] WORKDIR /frontend-build                        0.0s
 => CACHED [frontend  3/11] COPY web/package*.json ./                      0.0s
 => CACHED [backend  2/10] WORKDIR /backend-build                          0.0s
 => CACHED [backend  3/10] RUN apt-get update && apt-get install -y     g  0.0s
 => CACHED [backend  4/10] COPY lib/linux_amd64/ /usr/local/lib/lancedb/   0.0s
 => CACHED [backend  5/10] COPY include/ /usr/local/include/lancedb/       0.0s
 => CACHED [backend  6/10] COPY go.mod go.sum ./                           0.0s
 => CACHED [backend  7/10] RUN go mod download                             0.0s
 => ERROR [frontend  4/11] RUN npm ci                                     53.4s
 => [backend  8/10] COPY . .                                              53.4s
------
 > [frontend  4/11] RUN npm ci:
14.47 npm warn deprecated @mui/base@5.0.0-beta.40-0: This package has been replaced by @base-ui-components/react
20.08 npm error code E404
20.08 npm error 404 Not Found - GET https://registry.npmjs.org/@usememos/mui/-/mui-0.1.0-20250601165716.tgz - Not found
20.08 npm error 404
20.08 npm error 404  '@usememos/mui@https://registry.npmjs.org/@usememos/mui/-/mui-0.1.0-20250601165716.tgz' is not in this registry.
20.08 npm error 404
20.08 npm error 404 Note that you can also install from a
20.08 npm error 404 tarball, folder, http url, or git url.
20.09 npm notice
20.09 npm notice New major version of npm available! 10.8.2 -> 11.16.0
20.09 npm notice Changelog: https://github.com/npm/cli/releases/tag/v11.16.0
20.09 npm notice To update run: npm install -g npm@11.16.0
20.09 npm notice
20.09 npm error A complete log of this run can be found in: /root/.npm/_logs/2026-05-28T23_18_39_031Z-debug-0.log
------
==> Building image
==> Building image with Depot
--> build:  (​)
[+] Building 20.9s (21/37)                                                      
 => [internal] load build definition from Dockerfile.local.fly             0.2s
 => => transferring dockerfile: 3.17kB                                     0.2s
 => [internal] load metadata for docker.io/library/ubuntu:24.04            0.6s
 => [internal] load metadata for docker.io/library/golang:1.24             0.7s
 => [internal] load metadata for docker.io/library/node:20-alpine          1.1s
 => [internal] load .dockerignore                                          0.3s
 => => transferring context: 74.07kB                                       0.3s
 => [internal] load build context                                          1.6s
 => => transferring context: 440.17kB                                      1.6s
 => [backend  1/10] FROM docker.io/library/golang:1.24@sha256:d2d2bc1c84f  0.0s
 => => resolve docker.io/library/golang:1.24@sha256:d2d2bc1c84f7e60d7d243  0.0s
 => [stage-2  1/10] FROM docker.io/library/ubuntu:24.04@sha256:c4a8d5503d  0.0s
 => => resolve docker.io/library/ubuntu:24.04@sha256:c4a8d5503dfb2a3eb8ab  0.0s
 => [frontend  1/11] FROM docker.io/library/node:20-alpine@sha256:fb4cd12  0.0s
 => => resolve docker.io/library/node:20-alpine@sha256:fb4cd12c85ee03686f  0.0s
 => CACHED [stage-2  2/10] WORKDIR /usr/local/memos                        0.0s
 => CACHED [stage-2  3/10] RUN apt-get update && apt-get install -y     c  0.0s
 => CACHED [frontend  2/11] WORKDIR /frontend-build                        0.0s
 => CACHED [frontend  3/11] COPY web/package*.json ./                      0.0s
 => CACHED [backend  2/10] WORKDIR /backend-build                          0.0s
 => CACHED [backend  3/10] RUN apt-get update && apt-get install -y     g  0.0s
 => CACHED [backend  4/10] COPY lib/linux_amd64/ /usr/local/lib/lancedb/   0.0s
 => CACHED [backend  5/10] COPY include/ /usr/local/include/lancedb/       0.0s
 => CACHED [backend  6/10] COPY go.mod go.sum ./                           0.0s
 => CACHED [backend  7/10] RUN go mod download                             0.0s
 => CACHED [backend  8/10] COPY . .                                        0.0s
 => ERROR [frontend  4/11] RUN npm ci                                     17.6s
------
 > [frontend  4/11] RUN npm ci:
13.10 npm warn deprecated @mui/base@5.0.0-beta.40-0: This package has been replaced by @base-ui-components/react
17.25 npm error code E404
17.25 npm error 404 Not Found - GET https://registry.npmjs.org/@usememos/mui/-/mui-0.1.0-20250601165716.tgz - Not found
17.25 npm error 404
17.25 npm error 404  '@usememos/mui@https://registry.npmjs.org/@usememos/mui/-/mui-0.1.0-20250601165716.tgz' is not in this registry.
17.25 npm error 404
17.25 npm error 404 Note that you can also install from a
17.25 npm error 404 tarball, folder, http url, or git url.
17.25 npm notice
17.25 npm notice New major version of npm available! 10.8.2 -> 11.16.0
17.25 npm notice Changelog: https://github.com/npm/cli/releases/tag/v11.16.0
17.25 npm notice To update run: npm install -g npm@11.16.0
17.25 npm notice
17.25 npm error A complete log of this run can be found in: /root/.npm/_logs/2026-05-28T23_19_38_483Z-debug-0.log
------
Error: failed to fetch an image or build from source: error building: failed to solve: process "/bin/sh -c npm ci" did not complete successfully: exit code: 1

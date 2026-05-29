chaschel@linux:~/Documents/go/bchat$ sudo docker build -f Dockerfile.fly -t bchat:rag .
[sudo] password for chaschel:        
[+] Building 174.8s (45/45) FINISHED                                                                                                         docker:default
 => [internal] load build definition from Dockerfile.fly                                                                                               0.0s
 => => transferring dockerfile: 3.10kB                                                                                                                 0.0s
 => [internal] load metadata for docker.io/library/node:20-alpine                                                                                      1.9s
 => [internal] load metadata for docker.io/library/debian:bookworm-slim                                                                                1.4s
 => [internal] load metadata for docker.io/library/golang:1.24                                                                                         1.9s
 => [internal] load .dockerignore                                                                                                                      0.0s
 => => transferring context: 74.35kB                                                                                                                   0.0s
 => [stage-2  1/10] FROM docker.io/library/debian:bookworm-slim@sha256:0104b334637a5f19aa9c983a91b54c89887c0984081f2068983107a6f6c21eeb                0.0s
 => => resolve docker.io/library/debian:bookworm-slim@sha256:0104b334637a5f19aa9c983a91b54c89887c0984081f2068983107a6f6c21eeb                          0.0s
 => [backend  1/10] FROM docker.io/library/golang:1.24@sha256:d2d2bc1c84f7e60d7d2438a3836ae7d0c847f4888464e7ec9ba3a1339a1ee804                        65.2s
 => => resolve docker.io/library/golang:1.24@sha256:d2d2bc1c84f7e60d7d2438a3836ae7d0c847f4888464e7ec9ba3a1339a1ee804                                   0.0s
 => => sha256:f7bdfd728ac2ad72d43b82689890dc698260d3a1049845f48fb3fb942df6c581 79.13MB / 79.13MB                                                      53.8s
 => => sha256:b2b04fcbed4bf6e5373e2607d2705704ec5b220f1d1306e06ab8fe9471b2f86a 102.14MB / 102.14MB                                                    55.9s
 => => sha256:b5e2021c4c8bd1a46b34d9608a9381afdc333600ee1ef3c94306ecf7373e1956 67.79MB / 67.79MB                                                      59.0s
 => => sha256:954d6059ca7bdbb9ceb566ca2239e01ef312165659d656753d7dbace7771a591 25.61MB / 25.61MB                                                       3.6s
 => => sha256:ef235bf1a09a237b896b69935c8c8d917c9c6a78b538724911414afc0a96763c 49.29MB / 49.29MB                                                      54.3s
 => => extracting sha256:ef235bf1a09a237b896b69935c8c8d917c9c6a78b538724911414afc0a96763c                                                              1.0s
 => => extracting sha256:954d6059ca7bdbb9ceb566ca2239e01ef312165659d656753d7dbace7771a591                                                              0.4s
 => => extracting sha256:b5e2021c4c8bd1a46b34d9608a9381afdc333600ee1ef3c94306ecf7373e1956                                                              1.4s
 => => extracting sha256:b2b04fcbed4bf6e5373e2607d2705704ec5b220f1d1306e06ab8fe9471b2f86a                                                              1.5s
 => => extracting sha256:f7bdfd728ac2ad72d43b82689890dc698260d3a1049845f48fb3fb942df6c581                                                              2.7s
 => => extracting sha256:50a27cd32f8983e7e43777ec65195fb6594ea877f31993fd555513c261ffc054                                                              0.0s
 => => extracting sha256:4f4fb700ef54461cfa02571ae0db9a0dc1e0cdb5577484a6d75e68dc38e8acc1                                                              0.0s
 => [internal] load build context                                                                                                                      1.7s
 => => transferring context: 659.21kB                                                                                                                  1.7s
 => [frontend  1/18] FROM docker.io/library/node:20-alpine@sha256:fb4cd12c85ee03686f6af5362a0b0d56d50c58a04632e6c0fb8363f609372293                     0.0s
 => => resolve docker.io/library/node:20-alpine@sha256:fb4cd12c85ee03686f6af5362a0b0d56d50c58a04632e6c0fb8363f609372293                                0.0s 
 => CACHED [stage-2  2/10] WORKDIR /usr/local/memos                                                                                                    0.0s 
 => CACHED [stage-2  3/10] RUN apt-get update && apt-get install -y     ca-certificates     tzdata     && rm -rf /var/lib/apt/lists/*                  0.0s 
 => CACHED [frontend  2/18] WORKDIR /frontend-build                                                                                                    0.0s 
 => CACHED [frontend  3/18] COPY web/package*.json ./                                                                                                  0.0s 
 => [frontend  4/18] COPY web/vendor ./vendor                                                                                                          0.0s 
 => [frontend  5/18] RUN npm ci                                                                                                                       51.8s
 => [frontend  6/18] COPY web/ .                                                                                                                       0.4s
 => [frontend  7/18] RUN find vendor/usememos-mui -maxdepth 3 -type f -print | sort                                                                    0.2s
 => [frontend  8/18] RUN ls -la vendor/usememos-mui/dist || true                                                                                       0.2s
 => [frontend  9/18] RUN ls -la node_modules/@usememos/mui || true                                                                                     0.2s
 => [frontend 10/18] RUN ls -la node_modules/@usememos/mui/dist || true                                                                                0.2s
 => [frontend 11/18] RUN node -e "console.log(require.resolve('@usememos/mui/package.json'))"                                                          0.3s
 => [frontend 12/18] RUN test -f node_modules/@usememos/mui/dist/index.css                                                                             0.2s
 => [frontend 13/18] RUN npm run release                                                                                                              53.0s
 => [backend  2/10] WORKDIR /backend-build                                                                                                             0.4s
 => [backend  3/10] RUN apt-get update && apt-get install -y     gcc     libc-dev     && rm -rf /var/lib/apt/lists/*                                   9.5s
 => [backend  4/10] COPY lib/linux_amd64/ /usr/local/lib/lancedb/                                                                                      1.0s
 => [backend  5/10] COPY include/ /usr/local/include/lancedb/                                                                                          0.0s
 => [backend  6/10] COPY go.mod go.sum ./                                                                                                              0.0s
 => [backend  7/10] RUN go mod download                                                                                                               53.9s
 => [frontend 14/18] WORKDIR /widget-build                                                                                                             0.1s
 => [frontend 15/18] COPY widget/package*.json ./                                                                                                      0.1s
 => [frontend 16/18] RUN npm ci                                                                                                                        2.0s
 => [frontend 17/18] COPY widget/ .                                                                                                                    0.1s
 => [frontend 18/18] RUN npm run build                                                                                                                 0.7s
 => [backend  8/10] COPY . .                                                                                                                           4.6s
 => [backend  9/10] COPY --from=frontend /server/router/frontend/dist ./server/router/frontend/dist                                                    0.2s
 => [backend 10/10] RUN go build -tags rag -ldflags="-s -w" -o memos ./bin/memos/main.go                                                              30.4s
 => [stage-2  4/10] COPY --from=backend /usr/local/lib/lancedb/liblancedb_go.so /usr/local/lib/                                                        0.3s
 => [stage-2  5/10] RUN ldconfig                                                                                                                       0.2s
 => [stage-2  6/10] COPY --from=backend /backend-build/memos .                                                                                         0.1s
 => [stage-2  7/10] COPY scripts/entrypoint.sh .                                                                                                       0.0s
 => [stage-2  8/10] RUN chmod +x entrypoint.sh                                                                                                         0.2s
 => [stage-2  9/10] COPY --from=frontend /widget-build/dist ./widget/dist                                                                              0.0s
 => [stage-2 10/10] RUN mkdir -p /var/opt/memos                                                                                                        0.2s
 => exporting to image                                                                                                                                 6.2s
 => => exporting layers                                                                                                                                5.2s
 => => exporting manifest sha256:fe3ccc04788bd2c94cac3e2a1fabfdb962319b0f3c6ceab43c224c8fbd9f8458                                                      0.0s
 => => exporting config sha256:3c633c03389205aa04bd66103cca67bedefc4304fa3059b48ff843c684ae0446                                                        0.0s
 => => exporting attestation manifest sha256:de5818800d7c870088fdfa7ae9d12dc618068af27a3a7decbd41c1bab1729ee2                                          0.0s
 => => exporting manifest list sha256:0682437cb79702e535dcd726907c195e10ae0ff7f7ae62507cfab21b21a7cbac                                                 0.0s
 => => naming to docker.io/library/bchat:rag                                                                                                           0.0s
 => => unpacking to docker.io/library/bchat:rag                                                                                                        0.8s
WARNING: current commit information was not captured by the build: failed to read current commit information with git rev-parse --is-inside-work-tree
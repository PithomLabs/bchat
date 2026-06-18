bash: /bin/brew: No such file or directory
bash: /bin/brew: No such file or directory
chaschel@xia:~/Desktop$ ls
48.pdf              LiteIDE.desktop           scratch
adp                 Obsidian-1.10.3.AppImage  your-source-to-prompt.html
bchat-fly-setup.sh  playlist.m3u
chaschel@xia:~/Desktop$ ls
48.pdf  b.sh             Obsidian-1.10.3.AppImage  scratch
adp     LiteIDE.desktop  playlist.m3u              your-source-to-prompt.html
chaschel@xia:~/Desktop$ ./b.sh
=========================================
  bchat fly.io Deployment Setup
=========================================

[Step 1/7] Checking flyctl installation...
Warning: Metrics token unavailable: failed to run query($slug: String!) { organization(slug: $slug) { id internalNumericId slug rawSlug name type billable limitedAccessTokens { nodes { id name expiresAt revokedAt user { email } } } } }: context canceled
✓ flyctl is installed: flyctl v0.4.6 linux/amd64 Commit: 4fd60abf9c13af3f0055008a6b1da8dba398ed69 BuildDate: 2026-01-27T13:58:35Z

[Step 2/7] Checking fly.io authentication...
Not logged in. Opening browser for authentication...
Opening https://fly.io/app/auth/cli/6f7973653266367a6a7a70323532693737796e636e676c36367a626e34326b61 ...

Waiting for session...⣷^CError: Not Found
chaschel@xia:~/Desktop$ ^C
chaschel@xia:~/Desktop$ ^C
chaschel@xia:~/Desktop$ ./b.sh
=========================================
  bchat fly.io Deployment Setup
=========================================

[Step 1/7] Checking flyctl installation...
Warning: Metrics token unavailable: failed to run query($slug: String!) { organization(slug: $slug) { id internalNumericId slug rawSlug name type billable limitedAccessTokens { nodes { id name expiresAt revokedAt user { email } } } } }: context canceled
✓ flyctl is installed: flyctl v0.4.6 linux/amd64 Commit: 4fd60abf9c13af3f0055008a6b1da8dba398ed69 BuildDate: 2026-01-27T13:58:35Z

[Step 2/7] Checking fly.io authentication...
Not logged in. Opening browser for authentication...
Opening https://fly.io/app/auth/cli/6e733336736a7a6469336e6272786564366e646b7579617a6c6c756435333576 ...

Waiting for session... Done
successfully logged in as gani.mendoza@gmail.com
Warning: Metrics token unavailable: failed to run query($slug: String!) { organization(slug: $slug) { id internalNumericId slug rawSlug name type billable limitedAccessTokens { nodes { id name expiresAt revokedAt user { email } } } } }: context canceled
✓ Logged in as: gani.mendoza@gmail.com

[Step 3/7] Choose storage type for LanceDB:

  1) LOCAL  - Store LanceDB on fly volume (simpler, single volume)
  2) S3     - Store LanceDB on Tigrisdata S3 (recommended for production)

Enter choice (1 or 2): 1
✓ Selected: local storage

[Step 4/7] Creating fly.io app...
Copying fly.local.toml to fly.toml...
Creating new fly app...
An existing fly.toml file was found
Using build strategies '[the "Dockerfile.local.fly" dockerfile]'. Remove [build] from fly.toml to force a rescan
Creating app in /home/chaschel/Documents/ibm/ai/bchat

════════════════════════════════════════════════════════════════════════════════




                       %%%%%%%
                  %#+++=.  .:+*+*#%%
               %#+=====:       :++=+#%
             %#+======-          .++==#%
            %*=======+.            .+==+%
           %#========+               *==+%
           #+========+              = *==*%
           #=========+           .. : .*=+#
           #=========*    ==      .:   *==*%
           #+========+-             :. +==*%
           #*=========+              :-*==*%         %@
           %#**+======+-           .-. #==*%      @%%%%@
           %%#++=======+:      .::.   :*=+%       @%
           %%%#*++=====++.            #=+#%      @%@
          %%  %%#*++++++*+.          **+#@       %@
         %%     @%#***+++*+         =*+#%%      %%
        @@        @%%##*****       :*+%@%%%    %%
        @@        @@@@%#***##     .**%%   %% %%%
        @      @@@@  @@@%%#**%   .+#%@     @%%
        @@@@@@@           @%#*#.:*%%
                            @%#**%@
                             @@%@@
                            %%#%%%
                          @#***+**%
                          %**+***#%
                          @@%#+*#%


We're about to launch your app on Fly.io. Here's what you're getting:

Organization: Isagani Mendoza          (fly launch defaults to the personal org)
Name:         bchat                    (specified on the command line)
Region:       Chicago, Illinois (US)   (specified on the command line)
App Machines: shared-cpu-1x, 512MB RAM (from your fly.toml)
Postgres:     <none>                   (not requested)
Redis:        <none>                   (not requested)
Tigris:       <none>                   (not requested)

════════════════════════════════════════════════════════════════════════════════



? Do you want to tweak these settings before proceeding? No

Created app 'bchat' in organization 'personal'

Admin URL: https://fly.io/apps/bchat
Hostname: bchat.fly.dev
? Create .dockerignore from 3 .gitignore files? (y/N) 
Wrote config file fly.toml
Validating /home/chaschel/Documents/ibm/ai/bchat/fly.toml
✓ Configuration is valid
Your app is ready! Deploy with `flyctl deploy`
^C
[Step 5/7] Skipping S3 storage (using local)...
✓ Skipped (local storage selected)

[Step 6/7] Setting required secrets...

OpenRouter API Key is required for LLM and embeddings.
Get your key at: https://openrouter.ai/keys
Enter your OPENROUTER_API_KEY (sk-or-v1-...): ^C
chaschel@xia:~/Desktop$ ^C
chaschel@xia:~/Desktop$ ^C
chaschel@xia:~/Desktop$ ^C
chaschel@xia:~/Desktop$ ^C
chaschel@xia:~/Desktop$ ^C
chaschel@xia:~/Desktop$ ./b.sh
=========================================
  bchat fly.io Deployment Setup
=========================================

[Step 1/7] Checking flyctl installation...
✓ flyctl is installed: flyctl v0.4.6 linux/amd64 Commit: 4fd60abf9c13af3f0055008a6b1da8dba398ed69 BuildDate: 2026-01-27T13:58:35Z

[Step 2/7] Checking fly.io authentication...
✓ Logged in as: gani.mendoza@gmail.com

[Step 3/7] Choose storage type for LanceDB:

  1) LOCAL  - Store LanceDB on fly volume (simpler, single volume)
  2) S3     - Store LanceDB on Tigrisdata S3 (recommended for production)

Enter choice (1 or 2): 1
✓ Selected: local storage

[Step 4/7] Creating fly.io app...
Copying fly.local.toml to fly.toml...
✓ App 'bchat' already exists

[Step 5/7] Skipping S3 storage (using local)...
✓ Skipped (local storage selected)

[Step 6/7] Setting required secrets...

OpenRouter API Key is required for LLM and embeddings.
Get your key at: https://openrouter.ai/keys
Enter your OPENROUTER_API_KEY (sk-or-v1-...): 
Secrets are staged for the first deployment
✓ OPENROUTER_API_KEY set

Generating ENCRYPTION_MASTER_KEY for tenant API key encryption...
Secrets are staged for the first deployment
✓ ENCRYPTION_MASTER_KEY set

Current secrets:
NAME                   	DIGEST          	STATUS 
* OPENROUTER_API_KEY   	90c2f2f34ef7f586	Staged	
* ENCRYPTION_MASTER_KEY	78f7f950b54c3729	Staged	

There are 2 secrets not deployed. Deploy with `fly secrets deploy` to make them available.

[Step 7/7] Creating fly volume for SQLite database...
Creating 1GB volume in sjc region...
Warning! Every volume is pinned to a specific physical host. You should create two or more volumes per application to avoid downtime. Learn more at https://fly.io/docs/volumes/overview/
? Do you still want to use the volumes feature? Yes
                  ID: vol_re8jx0wpe1q9811r
                Name: memos_data
                 App: bchat
              Region: sjc
                Zone: 1854
             Size GB: 1
           Encrypted: true
          Created at: 30 Jan 26 04:20 UTC
  Snapshot retention: 5
 Scheduled snapshots: true
✓ Volume created

=========================================
  Setup Complete!
=========================================

Configuration Summary:
  Storage Type:  local
  Dockerfile:    Dockerfile.local.fly
  fly.toml:      copied from fly.local.toml

Next Steps:
  1. cd /home/chaschel/Documents/ibm/ai/bchat
  2. fly deploy

Useful Commands:
  fly status      - Check app status
  fly logs        - View application logs
  fly open        - Open app in browser
  fly ssh console - SSH into container

Documentation: /home/chaschel/Documents/ibm/ai/bchat/docs/DOCS_DEPLOYMENT.MD
chaschel@xia:~/Desktop$ ./b.sh
=========================================
  bchat fly.io Deployment Setup
=========================================

[Step 1/7] Checking flyctl installation...
✓ flyctl is installed: flyctl v0.4.6 linux/amd64 Commit: 4fd60abf9c13af3f0055008a6b1da8dba398ed69 BuildDate: 2026-01-27T13:58:35Z

[Step 2/7] Checking fly.io authentication...
✓ Logged in as: gani.mendoza@gmail.com

[Step 3/7] Choose storage type for LanceDB:

  1) LOCAL  - Store LanceDB on fly volume (simpler, single volume)
  2) S3     - Store LanceDB on Tigrisdata S3 (recommended for production)

Enter choice (1 or 2): 1
✓ Selected: local storage

[Step 4/7] Creating fly.io app...
Copying fly.local.toml to fly.toml...
✓ App 'bchat' already exists

[Step 5/7] Skipping S3 storage (using local)...
✓ Skipped (local storage selected)

[Step 6/7] Setting required secrets...

OpenRouter API Key is required for LLM and embeddings.
Get your key at: https://openrouter.ai/keys
Enter your OPENROUTER_API_KEY (sk-or-v1-...): 
Secrets are staged for the first deployment
✓ OPENROUTER_API_KEY set

ENCRYPTION_MASTER_KEY is used for tenant API key encryption.
You can generate one with: uuidgen
Enter your ENCRYPTION_MASTER_KEY: ced26e6e-91ce-4120-b4c0-68ec9ea25701
Secrets are staged for the first deployment
✓ ENCRYPTION_MASTER_KEY set

Current secrets:
NAME                   	DIGEST          	STATUS 
* OPENROUTER_API_KEY   	90c2f2f34ef7f586	Staged	
* ENCRYPTION_MASTER_KEY	3827f65b4bc99faa	Staged	

There are 2 secrets not deployed. Deploy with `fly secrets deploy` to make them available.

[Step 7/7] Creating fly volume for SQLite database...
✓ Volume 'memos_data' already exists

=========================================
  Setup Complete!
=========================================

Configuration Summary:
  Storage Type:  local
  Dockerfile:    Dockerfile.local.fly
  fly.toml:      copied from fly.local.toml

Next Steps:
  1. cd /home/chaschel/Documents/ibm/ai/bchat
  2. fly deploy

Useful Commands:
  fly status      - Check app status
  fly logs        - View application logs
  fly open        - Open app in browser
  fly ssh console - SSH into container

Documentation: /home/chaschel/Documents/ibm/ai/bchat/docs/DOCS_DEPLOYMENT.MD
chaschel@xia:~/Desktop$ fly secrets deploy
Error: the config for your app is missing an app name, add an app field to the fly.toml file or specify with the -a flag
chaschel@xia:~/Desktop$ 



-------------------
FLY.IO Deployment
-------------------

bash: /bin/brew: No such file or directory
bash: /bin/brew: No such file or directory
chaschel@xia:~/Documents/ibm/ai/bchat$ fly secrets deploy
Error: no machines available to deploy
'fly secrets deploy' will only work if the app has been deployed and there are machines available

Try 'fly deploy' first

chaschel@xia:~/Documents/ibm/ai/bchat$ fly deploy
==> Verifying app config
Validating /home/chaschel/Documents/ibm/ai/bchat/fly.toml
✓ Configuration is valid
--> Verified app config
==> Building image
==> Building image with Depot
--> build:  (​)
[+] Building 272.1s (22/31)                                                                                  
 => [internal] load build definition from Dockerfile.local.fly                                          0.2s
 => => transferring dockerfile: 2.51kB                                                                  0.2s
 => [internal] load metadata for docker.io/library/ubuntu:24.04                                         2.5s
 => [internal] load metadata for docker.io/library/golang:1.24                                          3.8s
 => [internal] load metadata for docker.io/library/node:20-alpine                                       2.5s
 => [internal] load .dockerignore                                                                       0.1s
 => => transferring context: 2B                                                                         0.1s
 => [backend  1/10] FROM docker.io/library/golang:1.24@sha256:c2131140c7c29ff277b1c412d524b7f56289513  24.2s
 => => resolve docker.io/library/golang:1.24@sha256:c2131140c7c29ff277b1c412d524b7f56289513f49672c57a3  0.0s
 => => sha256:4f4fb700ef54461cfa02571ae0db9a0dc1e0cdb5577484a6d75e68dc38e8acc1 32B / 32B                0.7s
 => => sha256:fb9d4bc08b092267b3083b826637a46f93f553cd35428eac603f9a907e0aee7e 79.13MB / 79.13MB        3.4s
 => => sha256:8ccc0040e4177cd0b70dc94c7c29efde5111b371ef23865bd0573cb3ac3b43ed 127B / 127B              0.8s
 => => sha256:6db3b1fb3a5eb008e28839b9929b132276aaa7ad54a8d33c560c29577b26ee85 102.14MB / 102.14MB      6.7s
 => => sha256:be442a7e0d6f290b909f8da51840566e06ab51bfbea277c70fbda26c44c8259d 67.79MB / 67.79MB        5.1s
 => => sha256:82e18c5e1c15ff34b31f1443e9327b69daaa0c1bd65a23846328fc3738c7f8f1 25.61MB / 25.61MB        1.5s
 => => sha256:2ca1bfae7ba8b9e2a56c1c19a2d14036cae96bf868ca154ca88bc078eaf7c376 49.29MB / 49.29MB        3.8s
 => => extracting sha256:2ca1bfae7ba8b9e2a56c1c19a2d14036cae96bf868ca154ca88bc078eaf7c376               2.3s
 => => extracting sha256:82e18c5e1c15ff34b31f1443e9327b69daaa0c1bd65a23846328fc3738c7f8f1               1.3s
 => => extracting sha256:be442a7e0d6f290b909f8da51840566e06ab51bfbea277c70fbda26c44c8259d               3.7s
 => => extracting sha256:6db3b1fb3a5eb008e28839b9929b132276aaa7ad54a8d33c560c29577b26ee85               3.3s
 => => extracting sha256:fb9d4bc08b092267b3083b826637a46f93f553cd35428eac603f9a907e0aee7e               6.7s
 => => extracting sha256:8ccc0040e4177cd0b70dc94c7c29efde5111b371ef23865bd0573cb3ac3b43ed               0.0s
 => => extracting sha256:4f4fb700ef54461cfa02571ae0db9a0dc1e0cdb5577484a6d75e68dc38e8acc1               0.0s
 => [stage-2 1/9] FROM docker.io/library/ubuntu:24.04@sha256:cd1dba651b3080c3686ecf4e3c4220f026b521fb7  7.8s
 => => resolve docker.io/library/ubuntu:24.04@sha256:cd1dba651b3080c3686ecf4e3c4220f026b521fb769788817  0.0s
 => => sha256:a3629ac5b9f4680dc2032439ff2354e73b06aecc2e68f0035a2d7c001c8b4114 29.73MB / 29.73MB        1.6s
 => => extracting sha256:a3629ac5b9f4680dc2032439ff2354e73b06aecc2e68f0035a2d7c001c8b4114               6.2s
 => [internal] load build context                                                                     267.9s
 => => transferring context: 2.00GB                                                                   169.3s
 => [frontend 1/6] FROM docker.io/library/node:20-alpine@sha256:09e2b3d9726018aecf269bd35325f46bf75046  9.3s
 => => resolve docker.io/library/node:20-alpine@sha256:09e2b3d9726018aecf269bd35325f46bf75046a643a66d2  0.0s
 => => sha256:e31b2016552274339ed88ed4a438d78bf37e0f6bdf328d02207b2a598c1ef86d 445B / 445B              0.1s
 => => sha256:eb87f4721c91769ed5206f34a9ab6ec98fc1d5235c12c2fc956665b1155e9ecb 1.26MB / 1.26MB          0.2s
 => => sha256:ad6d96c196e3198e14ea37df8bba4f54bf92fb525eb65e49fa4027c7dee13f80 42.78MB / 42.78MB        0.8s
 => => sha256:589002ba0eaed121a1dbf42f6648f29e5be55d5c8a6ee0f8eaa0285cc21ac153 3.86MB / 3.86MB          0.5s
 => => extracting sha256:589002ba0eaed121a1dbf42f6648f29e5be55d5c8a6ee0f8eaa0285cc21ac153               0.2s
 => => extracting sha256:ad6d96c196e3198e14ea37df8bba4f54bf92fb525eb65e49fa4027c7dee13f80               6.7s
 => => extracting sha256:eb87f4721c91769ed5206f34a9ab6ec98fc1d5235c12c2fc956665b1155e9ecb               1.5s
 => => extracting sha256:e31b2016552274339ed88ed4a438d78bf37e0f6bdf328d02207b2a598c1ef86d               0.2s
 => [stage-2 2/9] WORKDIR /usr/local/memos                                                              9.5s
 => [frontend 2/6] WORKDIR /frontend-build                                                             11.3s
 => [stage-2 3/9] RUN apt-get update && apt-get install -y     ca-certificates     tzdata     && rm -  16.0s
 => [backend  2/10] WORKDIR /backend-build                                                              3.8s
 => [backend  3/10] RUN apt-get update && apt-get install -y     gcc     libc-dev     && rm -rf /var/l  4.4s
 => [frontend 3/6] COPY web/package*.json ./                                                            7.8s
 => [backend  4/10] COPY lib/linux_amd64/ /usr/local/lib/lancedb/                                       7.8s
 => [backend  5/10] COPY include/ /usr/local/include/lancedb/                                           0.0s
 => [frontend 4/6] RUN npm ci                                                                          25.0s
 => [backend  6/10] COPY go.mod go.sum ./                                                               0.0s
 => [backend  7/10] RUN go mod download                                                                25.0s
 => [frontend 5/6] COPY web/ .                                                                         61.7s
 => [backend  8/10] COPY . .                                                                           61.7s
 => ERROR [frontend 6/6] RUN npm run release                                                            2.9s
------                                                                                                       
 > [frontend 6/6] RUN npm run release:                                                                       
0.753 
0.753 > release
0.753 > vite build --mode release --outDir=../server/router/frontend/dist --emptyOutDir
0.753 
1.767 vite v6.4.1 building for release...
2.212 transforming...
2.808 ✓ 16 modules transformed.
2.812 ✗ Build failed in 1.00s
2.812 error during build:
2.812 [vite]: Rollup failed to resolve import "axios" from "/frontend-build/src/store/v2/user.ts".
2.812 This is most likely unintended because it can break your application at runtime.
2.812 If you do want to externalize this module explicitly add it to
2.812 `build.rollupOptions.external`
2.812     at viteLog (file:///frontend-build/node_modules/vite/dist/node/chunks/dep-D4NMHUTW.js:46374:15)
2.812     at file:///frontend-build/node_modules/vite/dist/node/chunks/dep-D4NMHUTW.js:46432:18
2.812     at onwarn (/frontend-build/node_modules/@vitejs/plugin-react/dist/index.cjs:112:7)
2.812     at file:///frontend-build/node_modules/vite/dist/node/chunks/dep-D4NMHUTW.js:46430:7
2.812     at onRollupLog (file:///frontend-build/node_modules/vite/dist/node/chunks/dep-D4NMHUTW.js:46422:5)
2.812     at onLog (file:///frontend-build/node_modules/vite/dist/node/chunks/dep-D4NMHUTW.js:46072:7)
2.812     at file:///frontend-build/node_modules/rollup/dist/es/shared/node-entry.js:21004:32
2.812     at Object.logger [as onLog] (file:///frontend-build/node_modules/rollup/dist/es/shared/node-entry.js:22891:9)
2.812     at ModuleLoader.handleInvalidResolvedId (file:///frontend-build/node_modules/rollup/dist/es/shared/node-entry.js:21635:26)
2.812     at file:///frontend-build/node_modules/rollup/dist/es/shared/node-entry.js:21593:26
------
2026/01/30 12:34:20 error releasing builder: deadline_exceeded: context deadline exceeded
==> Building image
==> Building image with Depot
--> build:  (​)
[+] Building 6.4s (22/31)                                                                                    
 => [internal] load build definition from Dockerfile.local.fly                                          0.2s
 => => transferring dockerfile: 2.51kB                                                                  0.2s
 => [internal] load metadata for docker.io/library/ubuntu:24.04                                         0.7s
 => [internal] load metadata for docker.io/library/node:20-alpine                                       1.7s
 => [internal] load metadata for docker.io/library/golang:1.24                                          0.7s
 => [internal] load .dockerignore                                                                       0.2s
 => => transferring context: 2B                                                                         0.2s
 => [backend  1/10] FROM docker.io/library/golang:1.24@sha256:c2131140c7c29ff277b1c412d524b7f56289513f  0.0s
 => => resolve docker.io/library/golang:1.24@sha256:c2131140c7c29ff277b1c412d524b7f56289513f49672c57a3  0.0s
 => [internal] load build context                                                                       4.3s
 => => transferring context: 3.40MB                                                                     1.7s
 => [stage-2 1/9] FROM docker.io/library/ubuntu:24.04@sha256:cd1dba651b3080c3686ecf4e3c4220f026b521fb7  0.0s
 => => resolve docker.io/library/ubuntu:24.04@sha256:cd1dba651b3080c3686ecf4e3c4220f026b521fb769788817  0.0s
 => [frontend 1/6] FROM docker.io/library/node:20-alpine@sha256:09e2b3d9726018aecf269bd35325f46bf75046  0.0s
 => => resolve docker.io/library/node:20-alpine@sha256:09e2b3d9726018aecf269bd35325f46bf75046a643a66d2  0.0s
 => CACHED [stage-2 2/9] WORKDIR /usr/local/memos                                                       0.0s
 => CACHED [stage-2 3/9] RUN apt-get update && apt-get install -y     ca-certificates     tzdata     &  0.0s
 => CACHED [backend  2/10] WORKDIR /backend-build                                                       0.0s
 => CACHED [backend  3/10] RUN apt-get update && apt-get install -y     gcc     libc-dev     && rm -rf  0.0s
 => CACHED [backend  4/10] COPY lib/linux_amd64/ /usr/local/lib/lancedb/                                0.0s
 => CACHED [backend  5/10] COPY include/ /usr/local/include/lancedb/                                    0.0s
 => CACHED [backend  6/10] COPY go.mod go.sum ./                                                        0.0s
 => CACHED [backend  7/10] RUN go mod download                                                          0.0s
 => CACHED [backend  8/10] COPY . .                                                                     0.0s
 => CACHED [frontend 2/6] WORKDIR /frontend-build                                                       0.0s
 => CACHED [frontend 3/6] COPY web/package*.json ./                                                     0.0s
 => CACHED [frontend 4/6] RUN npm ci                                                                    0.0s
 => CACHED [frontend 5/6] COPY web/ .                                                                   0.0s
 => ERROR [frontend 6/6] RUN npm run release                                                            2.3s
------                                                                                                       
 > [frontend 6/6] RUN npm run release:                                                                       
0.290                                                                                                        
0.290 > release                                                                                              
0.290 > vite build --mode release --outDir=../server/router/frontend/dist --emptyOutDir                      
0.290 
1.216 vite v6.4.1 building for release...
1.629 transforming...
2.239 ✓ 107 modules transformed.
2.241 ✗ Build failed in 980ms
2.242 error during build:
2.242 [vite]: Rollup failed to resolve import "axios" from "/frontend-build/src/store/v2/user.ts".
2.242 This is most likely unintended because it can break your application at runtime.
2.242 If you do want to externalize this module explicitly add it to
2.242 `build.rollupOptions.external`
2.242     at viteLog (file:///frontend-build/node_modules/vite/dist/node/chunks/dep-D4NMHUTW.js:46374:15)
2.242     at file:///frontend-build/node_modules/vite/dist/node/chunks/dep-D4NMHUTW.js:46432:18
2.242     at onwarn (/frontend-build/node_modules/@vitejs/plugin-react/dist/index.cjs:112:7)
2.242     at file:///frontend-build/node_modules/vite/dist/node/chunks/dep-D4NMHUTW.js:46430:7
2.242     at onRollupLog (file:///frontend-build/node_modules/vite/dist/node/chunks/dep-D4NMHUTW.js:46422:5)
2.242     at onLog (file:///frontend-build/node_modules/vite/dist/node/chunks/dep-D4NMHUTW.js:46072:7)
2.242     at file:///frontend-build/node_modules/rollup/dist/es/shared/node-entry.js:21004:32
2.242     at Object.logger [as onLog] (file:///frontend-build/node_modules/rollup/dist/es/shared/node-entry.js:22891:9)
2.242     at ModuleLoader.handleInvalidResolvedId (file:///frontend-build/node_modules/rollup/dist/es/shared/node-entry.js:21635:26)
2.242     at file:///frontend-build/node_modules/rollup/dist/es/shared/node-entry.js:21593:26
------
Error: failed to fetch an image or build from source: error building: failed to solve: process "/bin/sh -c npm run release" did not complete successfully: exit code: 1
chaschel@xia:~/Documents/ibm/ai/bchat$ fly deploy
==> Verifying app config
Validating /home/chaschel/Documents/ibm/ai/bchat/fly.toml
✓ Configuration is valid
--> Verified app config
==> Building image
==> Building image with Depot
--> build:  (​)
[+] Building 235.2s (31/32)                                                                                  
 => [internal] load build definition from Dockerfile.local.fly                                          0.2s
 => => transferring dockerfile: 2.51kB                                                                  0.2s
 => [internal] load metadata for docker.io/library/ubuntu:24.04                                         1.8s
 => [internal] load metadata for docker.io/library/node:20-alpine                                       0.7s
 => [internal] load metadata for docker.io/library/golang:1.24                                          0.7s
 => [internal] load .dockerignore                                                                       0.2s
 => => transferring context: 2B                                                                         0.2s
 => [backend  1/10] FROM docker.io/library/golang:1.24@sha256:c2131140c7c29ff277b1c412d524b7f56289513f  0.0s
 => => resolve docker.io/library/golang:1.24@sha256:c2131140c7c29ff277b1c412d524b7f56289513f49672c57a3  0.0s
 => [stage-2 1/9] FROM docker.io/library/ubuntu:24.04@sha256:cd1dba651b3080c3686ecf4e3c4220f026b521fb7  0.0s
 => => resolve docker.io/library/ubuntu:24.04@sha256:cd1dba651b3080c3686ecf4e3c4220f026b521fb769788817  0.0s
 => [internal] load build context                                                                     233.0s
 => => transferring context: 6.99MB                                                                     2.3s
 => [frontend 1/6] FROM docker.io/library/node:20-alpine@sha256:09e2b3d9726018aecf269bd35325f46bf75046  0.0s
 => => resolve docker.io/library/node:20-alpine@sha256:09e2b3d9726018aecf269bd35325f46bf75046a643a66d2  0.0s
 => CACHED [stage-2 2/9] WORKDIR /usr/local/memos                                                       0.0s
 => CACHED [stage-2 3/9] RUN apt-get update && apt-get install -y     ca-certificates     tzdata     &  0.0s
 => CACHED [frontend 2/6] WORKDIR /frontend-build                                                       0.0s
 => [frontend 3/6] COPY web/package*.json ./                                                            2.6s
 => CACHED [backend  2/10] WORKDIR /backend-build                                                       0.0s
 => CACHED [backend  3/10] RUN apt-get update && apt-get install -y     gcc     libc-dev     && rm -rf  0.0s
 => CACHED [backend  4/10] COPY lib/linux_amd64/ /usr/local/lib/lancedb/                                0.0s
 => CACHED [backend  5/10] COPY include/ /usr/local/include/lancedb/                                    0.0s
 => CACHED [backend  6/10] COPY go.mod go.sum ./                                                        0.0s
 => CACHED [backend  7/10] RUN go mod download                                                          0.0s
 => [backend  8/10] COPY . .                                                                           62.0s
 => [frontend 4/6] RUN npm ci                                                                          42.4s
 => [frontend 5/6] COPY web/ .                                                                         19.7s 
 => [frontend 6/6] RUN npm run release                                                                 91.1s 
 => [backend  9/10] COPY --from=frontend /frontend-build/dist ./web/dist                                0.8s 
 => [backend 10/10] RUN go build -tags rag -ldflags="-s -w" -o memos ./bin/memos/main.go               60.0s 
 => [stage-2 4/9] COPY --from=backend /usr/local/lib/lancedb/liblancedb_go.so /usr/local/lib/           2.2s 
 => [stage-2 5/9] RUN ldconfig                                                                          0.1s 
 => [stage-2 6/9] COPY --from=backend /backend-build/memos .                                            0.6s 
 => [stage-2 7/9] COPY scripts/entrypoint.sh .                                                          0.0s 
 => [stage-2 8/9] RUN chmod +x entrypoint.sh                                                            0.1s
 => [stage-2 9/9] RUN mkdir -p /var/opt/memos/lancedb                                                   0.1s
 => exporting to image                                                                                  7.5s
 => => exporting layers                                                                                 6.0s
 => => exporting manifest sha256:e149f8d8b1b24e4179d33789fcce2be9e91afc7221dac1d22ee3634c65245fbb       0.0s
 => => exporting config sha256:f8ddadfcc455cb2562c5e015d42da07151df767f72464d203b1fb39b4b68b20b         0.0s
 => => pushing layers for registry.fly.io/bchat:deployment-01KG6K258B8CZ3HRNRHQJP56YJ@sha256:e149f8d8b  1.3s
 => => pushing layer sha256:d7e2615203d4dc2832201a4b75d64925806790981293a492c696f378db702e5c            0.7s
 => => pushing layer sha256:af02388f00275adb8ecc6b1026c8b42e84111c19cb8bc525b9019eda33abc87f            1.3s
 => => pushing layer sha256:320885d507974429b97b08a0d395ea2353a7bd1f81167319d42126712cee1609            0.7s
 => => pushing layer sha256:074677e330bf9ce537158317a8da0f7c16dd43b665b24c0d8cdfcc0f78ab2069            0.7s
 => => pushing layer sha256:4f606f693ee25549575bb04e63c08f6e4a6a1c2cc00b1f50b437de77d65d73da            0.8s
 => => pushing layer sha256:2c1ce468d9f3d941396801f6e3afc8921466650dd05430fe644cd3537713d27f            0.3s
 => => pushing layer sha256:f8ddadfcc455cb2562c5e015d42da07151df767f72464d203b1fb39b4b68b20b            0.7s
 => => pushing layer sha256:120422edc1b67bdfee59bb02873471c8eb843711eb7df2010f2b8f782962a0ef            0.7s
 => => pushing layer sha256:17a8e3f97b43b831472a9c9280c9b6030a7545f65aae41725efbe6c331582cc6            1.1s
 => => pushing layer sha256:90cffd5394ffd411390fc952966bbd5743084aabc8df2a9e3c70284442c7d448            0.7s
 => => pushing manifest for registry.fly.io/bchat:deployment-01KG6K258B8CZ3HRNRHQJP56YJ@sha256:e149f8d  0.1s
--> Build Summary:  (​)
--> Building image done
image: registry.fly.io/bchat:deployment-01KG6K258B8CZ3HRNRHQJP56YJ
image size: 88 MB

Watch your deployment at https://fly.io/apps/bchat/monitoring

Provisioning ips for bchat
  Dedicated ipv6: 2a09:8280:1::cd:5d53:0
  Shared ipv4: 66.241.125.147
  Add a dedicated ipv4 with: fly ips allocate-v4

This deployment will:
 * create 1 "app" machine

No machines in group app, launching a new machine
Finished launching new machines
-------
 ✔ Machine 8e4765f7602608 [app] update finished: success
NOTE: The machines for [app] have services with 'auto_stop_machines = "stop"' that will be stopped when idling

-------
Checking DNS configuration for bchat.fly.dev
✓ DNS configuration verified

Visit your newly deployed app at https://bchat.fly.dev/

chaschel@xia:~/Documents/ibm/ai/bchat$ 

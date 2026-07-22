task run:rag
task: [validate:migrations] ./scripts/validate-migrations.sh
task: [build:frontend] cd web && nub run release
task: [test:scripts] ./scripts/test/run-tests.sh
task: [validate:parity] ./scripts/validate-parity.sh
task: Task "setup:lancedb" is up to date
=== Script Tests ===

Test 1: bump-version.sh detects version mismatch
=== Cross-Driver Parity Validator ===

Check 1: File-list parity
Validating LATEST.sql against migrations...
Using migration directory: /home/chaschel/Documents/go/bchat/store/migration/sqlite/0.33

✓ LATEST.sql is in sync with all migrations
$ vite build --mode release --outDir=../server/router/frontend/dist --emptyOutDir
  PASS: bump-version.sh exits 1 (version mismatch) (exit=1)
  PASS: shows computed version
  PASS: shows current version.go
  PASS: shows MISMATCH

Test 2: create-migration.sh --dry-run
  PASS: File lists are in sync

Check 2: Schema parity (best-effort lint)
  PASS: Schema structure matches (best-effort check)

=== Summary ===
PASS: All checks passed
  PASS: exits 0 (exit=0)
  PASS: shows migration name

Test 3: create-migration.sh validates input
  PASS: rejects invalid name (exit=1)
  PASS: rejects missing name (exit=1)
  PASS: rejects uppercase (exit=1)

Test 4: validate-parity.sh passes on known-good pair
  PASS: validate-parity.sh passes on known-good pair (exit=0)

Test 5: validate-parity.sh detects schema drift
  PASS: validate-parity.sh detects drift (exit=2)

Test 6: validate-parity.sh detects file-list mismatch
  PASS: validate-parity.sh detects file-list mismatch (exit=1)

=== Results ===
  Passed: 12
  Failed: 0
  STATUS: PASS
vite v6.4.1 building for release...
transforming (1) src/main.tsxBrowserslist: browsers data (caniuse-lite) is 7 months old. Please run:
  npx update-browserslist-db@latest
  Why you should do it regularly: https://github.com/browserslist/update-db#readme
✓ 5122 modules transformed.
[plugin vite:reporter] 
(!) /home/chaschel/Documents/go/bchat/web/src/locales/en.json is dynamically imported by /home/chaschel/Documents/go/bchat/web/src/i18n.ts, /home/chaschel/Documents/go/bchat/web/src/i18n.ts but also statically imported by /home/chaschel/Documents/go/bchat/web/src/i18n.ts, dynamic import will not move module into another chunk.

../server/router/frontend/dist/assets/en-GB-legacy.Cf_KHwaR.js                           0.09 kB │ gzip:   0.10 kB
../server/router/frontend/dist/assets/chunk-QZHKN3VN-legacy.CQH1e7UJ.js                  0.27 kB │ gzip:   0.21 kB
../server/router/frontend/dist/assets/MemoDetailRedirect-legacy.C3bc4gRy.js              0.28 kB │ gzip:   0.24 kB
../server/router/frontend/dist/assets/chunk-55IACEB6-legacy.D3CL_OZZ.js                  0.31 kB │ gzip:   0.25 kB
../server/router/frontend/dist/assets/chunk-4BX2VUAB-legacy.cXcuIiZB.js                  0.31 kB │ gzip:   0.23 kB
../server/router/frontend/dist/assets/check-legacy.CdB3Cobg.js                           0.38 kB │ gzip:   0.30 kB
../server/router/frontend/dist/assets/play-legacy.Bmx6e9LS.js                            0.39 kB │ gzip:   0.30 kB
../server/router/frontend/dist/assets/message-circle-legacy.DlsoSMQZ.js                  0.42 kB │ gzip:   0.31 kB
../server/router/frontend/dist/assets/arrow-left-legacy.DZOW3CoZ.js                      0.44 kB │ gzip:   0.32 kB
../server/router/frontend/dist/assets/message-square-legacy.B4fR-4Lr.js                  0.45 kB │ gzip:   0.32 kB
../server/router/frontend/dist/assets/chunk-FMBD7UC4-legacy.DrLx2Zom.js                  0.46 kB │ gzip:   0.32 kB
../server/router/frontend/dist/assets/user-legacy.kRfbahq9.js                            0.47 kB │ gzip:   0.34 kB
../server/router/frontend/dist/assets/info-legacy.fx1BPDlO.js                            0.48 kB │ gzip:   0.33 kB
../server/router/frontend/dist/assets/external-link-legacy.DrNRaCU3.js                   0.53 kB │ gzip:   0.35 kB
../server/router/frontend/dist/assets/workflow-legacy.pBfkNu03.js                        0.54 kB │ gzip:   0.36 kB
../server/router/frontend/dist/assets/stateDiagram-v2-4FDKWEC3-legacy.CFQ4gYfF.js        0.55 kB │ gzip:   0.36 kB
../server/router/frontend/dist/assets/AuthFooter-legacy.B-WWkZKT.js                      0.58 kB │ gzip:   0.37 kB
../server/router/frontend/dist/assets/classDiagram-2ON5EDUG-legacy.DljdwVSN.js           0.59 kB │ gzip:   0.39 kB
../server/router/frontend/dist/assets/classDiagram-v2-WZHVMYZB-legacy.DljdwVSN.js        0.59 kB │ gzip:   0.39 kB
../server/router/frontend/dist/assets/chunk-QN33PNHL-legacy.CWq63dMZ.js                  0.59 kB │ gzip:   0.41 kB
../server/router/frontend/dist/assets/refresh-cw-legacy.BF2XqVIM.js                      0.60 kB │ gzip:   0.38 kB
../server/router/frontend/dist/assets/trash-2-legacy.BmFDQbWN.js                         0.63 kB │ gzip:   0.41 kB
../server/router/frontend/dist/assets/infoDiagram-WHAUD3N6-legacy.BPIhSOLZ.js            0.73 kB │ gzip:   0.48 kB
../server/router/frontend/dist/assets/sparkles-legacy.BT5V4cLC.js                        0.78 kB │ gzip:   0.46 kB
../server/router/frontend/dist/assets/PermissionDenied-legacy.CTjUt823.js                0.87 kB │ gzip:   0.48 kB
../server/router/frontend/dist/assets/NotFound-legacy.CJBk8PfW.js                        0.89 kB │ gzip:   0.50 kB
../server/router/frontend/dist/assets/Explore-legacy.C9v2mbf4.js                         1.09 kB │ gzip:   0.63 kB
../server/router/frontend/dist/assets/Archived-legacy.3ycdk4gT.js                        1.22 kB │ gzip:   0.66 kB
../server/router/frontend/dist/assets/AdminSignIn-legacy.DB4EVHhj.js                     1.33 kB │ gzip:   0.67 kB
../server/router/frontend/dist/assets/AuthCallback-legacy.BiOEXpwF.js                    1.39 kB │ gzip:   0.76 kB
../server/router/frontend/dist/assets/chunk-TZMSLE5B-legacy.DHe0dTb1.js                  1.51 kB │ gzip:   0.68 kB
../server/router/frontend/dist/assets/ca-legacy.Do233O3E.js                              1.52 kB │ gzip:   0.87 kB
../server/router/frontend/dist/assets/LocaleSelect-legacy.BmJZr4CP.js                    2.22 kB │ gzip:   1.02 kB
../server/router/frontend/dist/assets/TicketDetail-legacy.41rjNn_h.js                    2.49 kB │ gzip:   0.99 kB
../server/router/frontend/dist/assets/Notifications-legacy.Byetud34.js                   2.52 kB │ gzip:   1.10 kB
../server/router/frontend/dist/assets/SignIn-legacy.BAPij5uZ.js                          2.70 kB │ gzip:   1.34 kB
../server/router/frontend/dist/assets/UserProfile-legacy.D2jAAfGO.js                     2.84 kB │ gzip:   1.41 kB
../server/router/frontend/dist/assets/SignUp-legacy.dAZOo8JY.js                          3.47 kB │ gzip:   1.38 kB
../server/router/frontend/dist/assets/sv-legacy.Bg3xxBqU.js                              3.65 kB │ gzip:   1.71 kB
../server/router/frontend/dist/assets/pieDiagram-ADFJNKIX-legacy.UztfEhbp.js             3.95 kB │ gzip:   1.79 kB
../server/router/frontend/dist/assets/PasswordSignInForm-legacy.HXYytFaD.js              4.12 kB │ gzip:   1.46 kB
../server/router/frontend/dist/assets/diagram-S2PKOQOG-legacy.BajrWItp.js                4.42 kB │ gzip:   1.91 kB
../server/router/frontend/dist/assets/Inboxes-legacy.By5oV_83.js                         4.76 kB │ gzip:   2.03 kB
../server/router/frontend/dist/assets/ar-legacy.DXXcc3y9.js                              4.89 kB │ gzip:   2.14 kB
../server/router/frontend/dist/assets/Resources-legacy.CDsF9RYG.js                       5.35 kB │ gzip:   1.78 kB
../server/router/frontend/dist/assets/graph-legacy.BAa9xAh4.js                           5.89 kB │ gzip:   1.85 kB
../server/router/frontend/dist/assets/diagram-QEK2KX5R-legacy.CQv-nJM-.js                6.02 kB │ gzip:   2.46 kB
../server/router/frontend/dist/assets/InternalAgent-legacy.uSAMgjvv.js                   6.04 kB │ gzip:   2.14 kB
../server/router/frontend/dist/assets/Chat-legacy.CoiPhG2U.js                            6.76 kB │ gzip:   2.33 kB
../server/router/frontend/dist/assets/nl-legacy.HzdYkiP1.js                              7.51 kB │ gzip:   3.08 kB
../server/router/frontend/dist/assets/hr-legacy.DFG1x8UV.js                              8.40 kB │ gzip:   3.66 kB
../server/router/frontend/dist/assets/it-legacy.Cks-hhXH.js                              8.43 kB │ gzip:   3.34 kB
../server/router/frontend/dist/assets/es-legacy.k9lPoDUc.js                              9.40 kB │ gzip:   3.69 kB
../server/router/frontend/dist/assets/hi-legacy.DBnS6Hti.js                              9.42 kB │ gzip:   2.88 kB
../server/router/frontend/dist/assets/ko-legacy.BZnXZ0Eg.js                              9.76 kB │ gzip:   4.09 kB
../server/router/frontend/dist/assets/hu-legacy.DqqbsCFV.js                             10.30 kB │ gzip:   4.15 kB
../server/router/frontend/dist/assets/stateDiagram-FKZM4ZOC-legacy.TWYZ1lD3.js          10.46 kB │ gzip:   3.62 kB
../server/router/frontend/dist/assets/pl-legacy.iRc3pGdk.js                             10.50 kB │ gzip:   4.42 kB
../server/router/frontend/dist/assets/diagram-PSM6KHXK-legacy.DAF_fc90.js               10.87 kB │ gzip:   3.74 kB
../server/router/frontend/dist/assets/id-legacy.XtMC4CKK.js                             10.92 kB │ gzip:   4.28 kB
../server/router/frontend/dist/assets/dagre-6UL2VRFP-legacy.IFzbfnOR.js                 11.03 kB │ gzip:   4.07 kB
../server/router/frontend/dist/assets/pt-PT-legacy.B7E9PuQY.js                          11.24 kB │ gzip:   4.40 kB
../server/router/frontend/dist/assets/nb-legacy.BL23we-n.js                             11.25 kB │ gzip:   4.56 kB
../server/router/frontend/dist/assets/ja-legacy.Dii3uIO-.js                             12.00 kB │ gzip:   4.63 kB
../server/router/frontend/dist/assets/de-legacy.BfkzKxwz.js                             12.07 kB │ gzip:   4.76 kB
../server/router/frontend/dist/assets/zh-Hant-legacy.BxiF6Oed.js                        12.24 kB │ gzip:   5.60 kB
../server/router/frontend/dist/assets/ru-legacy.BmyRgvFs.js                             12.35 kB │ gzip:   4.34 kB
../server/router/frontend/dist/assets/zh-Hans-legacy.C23T_3Xa.js                        12.55 kB │ gzip:   5.54 kB
../server/router/frontend/dist/assets/sl-legacy.C6iGhg95.js                             12.59 kB │ gzip:   5.11 kB
../server/router/frontend/dist/assets/vi-legacy.DS75qhhp.js                             12.95 kB │ gzip:   4.83 kB
../server/router/frontend/dist/assets/tr-legacy.yS6f94sI.js                             13.32 kB │ gzip:   5.23 kB
../server/router/frontend/dist/assets/fr-legacy.BgtSTWby.js                             13.71 kB │ gzip:   5.25 kB
../server/router/frontend/dist/assets/Playground-legacy.CbhdOZq2.js                     13.73 kB │ gzip:   4.27 kB
../server/router/frontend/dist/assets/pt-BR-legacy.B4OVTvfs.js                          14.15 kB │ gzip:   5.51 kB
../server/router/frontend/dist/assets/cs-legacy.BTjZYoCi.js                             14.51 kB │ gzip:   5.87 kB
../server/router/frontend/dist/assets/RagStats-legacy.CyzStbKd.js                       14.92 kB │ gzip:   4.07 kB
../server/router/frontend/dist/assets/mr-legacy.CCKPcQ86.js                             16.88 kB │ gzip:   4.80 kB
../server/router/frontend/dist/assets/fa-legacy.BqHolvk4.js                             16.88 kB │ gzip:   5.63 kB
../server/router/frontend/dist/assets/th-legacy.DprzZxoV.js                             17.78 kB │ gzip:   4.81 kB
../server/router/frontend/dist/assets/uk-legacy.CuhpMcDi.js                             18.19 kB │ gzip:   6.08 kB
../server/router/frontend/dist/assets/ka-GE-legacy.5ddekcpY.js                          18.52 kB │ gzip:   4.65 kB
../server/router/frontend/dist/assets/Tickets-legacy.CNlDkMSv.js                        19.38 kB │ gzip:   6.30 kB
../server/router/frontend/dist/assets/kanban-definition-3W4ZIXB7-legacy.CbctbKSY.js     19.75 kB │ gzip:   6.97 kB
../server/router/frontend/dist/assets/mindmap-definition-VGOIOE7T-legacy.DDBNPby2.js    19.85 kB │ gzip:   6.87 kB
../server/router/frontend/dist/assets/AgentSimulation-legacy.Bbp7-kHs.js                20.91 kB │ gzip:   5.61 kB
../server/router/frontend/dist/assets/sankeyDiagram-TZEHDZUN-legacy.BVIfgqU3.js         21.72 kB │ gzip:   7.76 kB
../server/router/frontend/dist/assets/timeline-definition-IT6M3QCI-legacy.Cr7BH3e1.js   23.40 kB │ gzip:   8.00 kB
../server/router/frontend/dist/assets/journeyDiagram-XKPGCS4Q-legacy.D37h-2rf.js        23.48 kB │ gzip:   8.18 kB
../server/router/frontend/dist/assets/gitGraphDiagram-NY62KEGX-legacy.CP9a59zd.js       24.21 kB │ gzip:   7.34 kB
../server/router/frontend/dist/assets/erDiagram-Q2GNP2WA-legacy.DlpwtMOu.js             24.71 kB │ gzip:   8.68 kB
../server/router/frontend/dist/assets/layout-legacy.DF1i1mjk.js                         24.77 kB │ gzip:   8.50 kB
../server/router/frontend/dist/assets/requirementDiagram-UZGBJVZJ-legacy.DT2HPWxk.js    29.89 kB │ gzip:   9.28 kB
../server/router/frontend/dist/assets/quadrantDiagram-AYHSOK5B-legacy.C1OQV8Dq.js       33.37 kB │ gzip:   9.71 kB
../server/router/frontend/dist/assets/chunk-DI55MBZ5-legacy.Bz4jKPGi.js                 35.36 kB │ gzip:  11.42 kB
../server/router/frontend/dist/assets/xychartDiagram-PRI3JC2R-legacy.C7D2jHb-.js        38.63 kB │ gzip:  10.54 kB
../server/router/frontend/dist/assets/chunk-B4BG7PRW-legacy.BPZqwC1m.js                 44.79 kB │ gzip:  14.45 kB
../server/router/frontend/dist/assets/ganttDiagram-JELNMOA3-legacy.WdzJOFTv.js          47.94 kB │ gzip:  15.85 kB
../server/router/frontend/dist/assets/flowDiagram-NV44I4VS-legacy.Cu0DHP6y.js           59.45 kB │ gzip:  19.13 kB
../server/router/frontend/dist/assets/Setting-legacy.CKhN4wb8.js                        59.59 kB │ gzip:  11.90 kB
../server/router/frontend/dist/assets/c4Diagram-YG6GDRKO-legacy.D3_GorMI.js             69.42 kB │ gzip:  19.30 kB
../server/router/frontend/dist/assets/blockDiagram-VD42YOAC-legacy.Dd_jhmjZ.js          70.18 kB │ gzip:  19.84 kB
../server/router/frontend/dist/assets/app-legacy.C2MZQSKI.js                            77.14 kB │ gzip:  28.44 kB
../server/router/frontend/dist/assets/AgentAdmin-legacy.9FRbVUCI.js                     80.52 kB │ gzip:  16.98 kB
../server/router/frontend/dist/assets/cose-bilkent-S5V4N54A-legacy.iVZnLLIQ.js          80.77 kB │ gzip:  21.40 kB
../server/router/frontend/dist/assets/sequenceDiagram-WL72ISMW-legacy.BBwVxp8U.js       96.89 kB │ gzip:  26.42 kB
../server/router/frontend/dist/assets/utils-vendor-legacy.BxaQyz1v.js                   98.91 kB │ gzip:  30.59 kB
../server/router/frontend/dist/assets/MemoDetail-legacy.DSswLF21.js                    137.13 kB │ gzip:  43.24 kB
../server/router/frontend/dist/assets/architectureDiagram-VXUJARFQ-legacy.D7ezUF2y.js  145.40 kB │ gzip:  39.82 kB
../server/router/frontend/dist/assets/leaflet-vendor-legacy.CslRHpxP.js                152.79 kB │ gzip:  44.38 kB
../server/router/frontend/dist/assets/react-vendor-legacy.BqH4zW9D.js                  223.14 kB │ gzip:  72.98 kB
../server/router/frontend/dist/assets/katex-vendor-legacy.Dzn61udv.js                  267.79 kB │ gzip:  77.24 kB
../server/router/frontend/dist/assets/treemap-KMMF4GRG-legacy.BuDcQNCA.js              324.89 kB │ gzip:  76.47 kB
../server/router/frontend/dist/assets/mui-vendor-legacy.l2ERqyJS.js                    402.39 kB │ gzip: 111.13 kB
../server/router/frontend/dist/assets/cytoscape.esm-legacy.B7uj18Wu.js                 431.87 kB │ gzip: 135.83 kB
../server/router/frontend/dist/assets/mermaid-vendor-legacy.C-J_l5VQ.js                523.12 kB │ gzip: 144.55 kB
../server/router/frontend/dist/assets/app-legacy.BTRtdgNs.js                           881.79 kB │ gzip: 237.65 kB
../server/router/frontend/dist/assets/highlight-vendor-legacy.Cnre7F_-.js              962.44 kB │ gzip: 308.07 kB

(!) Some chunks are larger than 500 kB after minification. Consider:
- Using dynamic import() to code-split the application
- Use build.rollupOptions.output.manualChunks to improve chunking: https://rollupjs.org/configuration-options/#output-manualchunks
- Adjust chunk size limit for this warning via build.chunkSizeWarningLimit.
[plugin vite:reporter] 
(!) /home/chaschel/Documents/go/bchat/web/src/locales/en.json is dynamically imported by /home/chaschel/Documents/go/bchat/web/src/i18n.ts, /home/chaschel/Documents/go/bchat/web/src/i18n.ts but also statically imported by /home/chaschel/Documents/go/bchat/web/src/i18n.ts, dynamic import will not move module into another chunk.

../server/router/frontend/dist/assets/KaTeX_Size3-Regular.CTq5MqoE.woff           4.42 kB
../server/router/frontend/dist/assets/KaTeX_Size4-Regular.Dl5lxZxV.woff2          4.93 kB
../server/router/frontend/dist/assets/KaTeX_Size2-Regular.Dy4dx90m.woff2          5.21 kB
../server/router/frontend/dist/assets/KaTeX_Size1-Regular.mCD8mA8B.woff2          5.47 kB
../server/router/frontend/dist/index.html                                         5.76 kB │ gzip:   2.02 kB
../server/router/frontend/dist/assets/KaTeX_Size4-Regular.BF-4gkZK.woff           5.98 kB
../server/router/frontend/dist/assets/KaTeX_Size2-Regular.oD1tc_U0.woff           6.19 kB
../server/router/frontend/dist/assets/KaTeX_Size1-Regular.C195tn64.woff           6.50 kB
../server/router/frontend/dist/assets/KaTeX_Caligraphic-Regular.Di6jR-x-.woff2    6.91 kB
../server/router/frontend/dist/assets/KaTeX_Caligraphic-Bold.Dq_IR9rO.woff2       6.91 kB
../server/router/frontend/dist/assets/KaTeX_Size3-Regular.DgpXs0kz.ttf            7.59 kB
../server/router/frontend/dist/assets/KaTeX_Caligraphic-Regular.CTRA-rTL.woff     7.66 kB
../server/router/frontend/dist/assets/KaTeX_Caligraphic-Bold.BEiXGLvX.woff        7.72 kB
../server/router/frontend/dist/assets/KaTeX_Script-Regular.D3wIWfF6.woff2         9.64 kB
../server/router/frontend/dist/assets/KaTeX_SansSerif-Regular.DDBCnlJ7.woff2     10.34 kB
../server/router/frontend/dist/assets/KaTeX_Size4-Regular.DWFBv043.ttf           10.36 kB
../server/router/frontend/dist/assets/KaTeX_Script-Regular.D5yQViql.woff         10.59 kB
../server/router/frontend/dist/assets/KaTeX_Fraktur-Regular.CTYiF6lA.woff2       11.32 kB
../server/router/frontend/dist/assets/KaTeX_Fraktur-Bold.CL6g_b3V.woff2          11.35 kB
../server/router/frontend/dist/assets/KaTeX_Size2-Regular.B7gKUWhC.ttf           11.51 kB
../server/router/frontend/dist/assets/KaTeX_SansSerif-Italic.C3H0VqGB.woff2      12.03 kB
../server/router/frontend/dist/assets/KaTeX_SansSerif-Bold.D1sUS0GD.woff2        12.22 kB
../server/router/frontend/dist/assets/KaTeX_Size1-Regular.Dbsnue_I.ttf           12.23 kB
../server/router/frontend/dist/assets/KaTeX_SansSerif-Regular.CS6fqUqJ.woff      12.32 kB
../server/router/frontend/dist/assets/KaTeX_Caligraphic-Regular.wX97UBjC.ttf     12.34 kB
../server/router/frontend/dist/assets/KaTeX_Caligraphic-Bold.ATXxdsX0.ttf        12.37 kB
../server/router/frontend/dist/assets/KaTeX_Fraktur-Regular.Dxdc4cR9.woff        13.21 kB
../server/router/frontend/dist/assets/KaTeX_Fraktur-Bold.BsDP51OF.woff           13.30 kB
../server/router/frontend/dist/assets/KaTeX_Typewriter-Regular.CO6r4hn1.woff2    13.57 kB
../server/router/frontend/dist/assets/KaTeX_SansSerif-Italic.DN2j7dab.woff       14.11 kB
../server/router/frontend/dist/assets/KaTeX_SansSerif-Bold.DbIhKOiC.woff         14.41 kB
../server/router/frontend/dist/assets/KaTeX_Typewriter-Regular.C0xS9mPB.woff     16.03 kB
../server/router/frontend/dist/assets/KaTeX_Math-BoldItalic.CZnvNsCZ.woff2       16.40 kB
../server/router/frontend/dist/assets/KaTeX_Math-Italic.t53AETM-.woff2           16.44 kB
../server/router/frontend/dist/assets/KaTeX_Script-Regular.C5JkGWo-.ttf          16.65 kB
../server/router/frontend/dist/assets/KaTeX_Main-BoldItalic.DxDJ3AOS.woff2       16.78 kB
../server/router/frontend/dist/assets/KaTeX_Main-Italic.NWA7e6Wa.woff2           16.99 kB
../server/router/frontend/dist/assets/KaTeX_Math-BoldItalic.iY-2wyZ7.woff        18.67 kB
../server/router/frontend/dist/assets/KaTeX_Math-Italic.DA0__PXp.woff            18.75 kB
../server/router/frontend/dist/assets/KaTeX_Main-BoldItalic.SpSLRI95.woff        19.41 kB
../server/router/frontend/dist/assets/KaTeX_SansSerif-Regular.BNo7hRIc.ttf       19.44 kB
../server/router/frontend/dist/assets/KaTeX_Fraktur-Regular.CB_wures.ttf         19.57 kB
../server/router/frontend/dist/assets/KaTeX_Fraktur-Bold.BdnERNNW.ttf            19.58 kB
../server/router/frontend/dist/assets/KaTeX_Main-Italic.BMLOBm91.woff            19.68 kB
../server/router/frontend/dist/assets/KaTeX_SansSerif-Italic.YYjJ1zSn.ttf        22.36 kB
../server/router/frontend/dist/assets/KaTeX_SansSerif-Bold.CFMepnvq.ttf          24.50 kB
../server/router/frontend/dist/assets/KaTeX_Main-Bold.Cx986IdX.woff2             25.32 kB
../server/router/frontend/dist/assets/KaTeX_Main-Regular.B22Nviop.woff2          26.27 kB
../server/router/frontend/dist/assets/KaTeX_Typewriter-Regular.D3Ib7_Hf.ttf      27.56 kB
../server/router/frontend/dist/assets/KaTeX_AMS-Regular.BQhdFMY1.woff2           28.08 kB
../server/router/frontend/dist/assets/KaTeX_Main-Bold.Jm3AIy58.woff              29.91 kB
../server/router/frontend/dist/assets/KaTeX_Main-Regular.Dr94JaBh.woff           30.77 kB
../server/router/frontend/dist/assets/KaTeX_Math-BoldItalic.B3XSjfu4.ttf         31.20 kB
../server/router/frontend/dist/assets/KaTeX_Math-Italic.flOr_0UB.ttf             31.31 kB
../server/router/frontend/dist/assets/KaTeX_Main-BoldItalic.DzxPMmG6.ttf         32.97 kB
../server/router/frontend/dist/assets/KaTeX_AMS-Regular.DMm9YOAa.woff            33.52 kB
../server/router/frontend/dist/assets/KaTeX_Main-Italic.3WenGoN9.ttf             33.58 kB
../server/router/frontend/dist/assets/KaTeX_Main-Bold.waoOVXN0.ttf               51.34 kB
../server/router/frontend/dist/assets/KaTeX_Main-Regular.ypZvNtVU.ttf            53.58 kB
../server/router/frontend/dist/assets/KaTeX_AMS-Regular.DRggAlZN.ttf             63.63 kB
../server/router/frontend/dist/assets/index.BIuklCrv.css                        121.15 kB │ gzip:  27.94 kB
../server/router/frontend/dist/assets/en-GB.BIHI7g3E.js                           0.03 kB │ gzip:   0.05 kB
../server/router/frontend/dist/assets/chunk-QZHKN3VN.DXUt0PK-.js                  0.20 kB │ gzip:   0.16 kB
../server/router/frontend/dist/assets/MemoDetailRedirect.DJAWOYbE.js              0.20 kB │ gzip:   0.18 kB
../server/router/frontend/dist/assets/chunk-55IACEB6.D0kYb5ZX.js                  0.24 kB │ gzip:   0.21 kB
../server/router/frontend/dist/assets/check.2UpgvWz0.js                           0.30 kB │ gzip:   0.25 kB
../server/router/frontend/dist/assets/chunk-4BX2VUAB.D3cDc8zo.js                  0.30 kB │ gzip:   0.21 kB
../server/router/frontend/dist/assets/play.Bidrrc8W.js                            0.31 kB │ gzip:   0.25 kB
../server/router/frontend/dist/assets/message-circle.dI9QOJgj.js                  0.32 kB │ gzip:   0.26 kB
../server/router/frontend/dist/assets/arrow-left.Djc_Bmco.js                      0.34 kB │ gzip:   0.27 kB
../server/router/frontend/dist/assets/message-square.D3Da6t_r.js                  0.35 kB │ gzip:   0.27 kB
../server/router/frontend/dist/assets/user.D0ksnbvL.js                            0.37 kB │ gzip:   0.29 kB
../server/router/frontend/dist/assets/info.B-pSq3Tt.js                            0.38 kB │ gzip:   0.28 kB
../server/router/frontend/dist/assets/chunk-FMBD7UC4.CcFM_1ZL.js                  0.38 kB │ gzip:   0.27 kB
../server/router/frontend/dist/assets/external-link.B6j-o6oz.js                   0.42 kB │ gzip:   0.30 kB
../server/router/frontend/dist/assets/stateDiagram-v2-4FDKWEC3.CGj37JgJ.js        0.44 kB │ gzip:   0.30 kB
../server/router/frontend/dist/assets/workflow.CNCG6O9M.js                        0.44 kB │ gzip:   0.31 kB
../server/router/frontend/dist/assets/classDiagram-2ON5EDUG.I_5RpPGS.js           0.47 kB │ gzip:   0.32 kB
../server/router/frontend/dist/assets/classDiagram-v2-WZHVMYZB.I_5RpPGS.js        0.47 kB │ gzip:   0.32 kB
../server/router/frontend/dist/assets/refresh-cw.SmJm7YH0.js                      0.49 kB │ gzip:   0.33 kB
../server/router/frontend/dist/assets/AuthFooter.D_WCh3b3.js                      0.52 kB │ gzip:   0.33 kB
../server/router/frontend/dist/assets/trash-2.D2wA_lKr.js                         0.53 kB │ gzip:   0.35 kB
../server/router/frontend/dist/assets/chunk-QN33PNHL.0v3zN4MA.js                  0.58 kB │ gzip:   0.38 kB
../server/router/frontend/dist/assets/infoDiagram-WHAUD3N6.CYDsljf9.js            0.65 kB │ gzip:   0.43 kB
../server/router/frontend/dist/assets/sparkles.H10cGNdf.js                        0.68 kB │ gzip:   0.40 kB
../server/router/frontend/dist/assets/PermissionDenied.DBv8evRx.js                0.75 kB │ gzip:   0.42 kB
../server/router/frontend/dist/assets/NotFound.KWJgTPfa.js                        0.78 kB │ gzip:   0.44 kB
../server/router/frontend/dist/assets/Explore.BZlzaT7n.js                         1.00 kB │ gzip:   0.57 kB
../server/router/frontend/dist/assets/Archived.D7Y0oNG-.js                        1.16 kB │ gzip:   0.61 kB
../server/router/frontend/dist/assets/AdminSignIn.CVVAnOes.js                     1.25 kB │ gzip:   0.64 kB
../server/router/frontend/dist/assets/AuthCallback.ZzPMzFwj.js                    1.27 kB │ gzip:   0.69 kB
../server/router/frontend/dist/assets/chunk-TZMSLE5B.D2rW58w6.js                  1.44 kB │ gzip:   0.64 kB
../server/router/frontend/dist/assets/ca.Cxw4Eakv.js                              1.45 kB │ gzip:   0.82 kB
../server/router/frontend/dist/assets/Notifications.DrlbAdmf.js                   2.40 kB │ gzip:   1.03 kB
../server/router/frontend/dist/assets/TicketDetail.mM7TNun0.js                    2.44 kB │ gzip:   0.95 kB
../server/router/frontend/dist/assets/LocaleSelect.DF_lBkSI.js                    2.69 kB │ gzip:   0.99 kB
../server/router/frontend/dist/assets/SignIn.CHMq5im3.js                          2.71 kB │ gzip:   1.30 kB
../server/router/frontend/dist/assets/UserProfile.kdiVTdkb.js                     2.81 kB │ gzip:   1.36 kB
../server/router/frontend/dist/assets/SignUp.BE6PIFgY.js                          3.40 kB │ gzip:   1.34 kB
../server/router/frontend/dist/assets/sv.P9ev13w9.js                              3.58 kB │ gzip:   1.67 kB
../server/router/frontend/dist/assets/PasswordSignInForm.CB-3NL1u.js              4.06 kB │ gzip:   1.43 kB
../server/router/frontend/dist/assets/pieDiagram-ADFJNKIX.CpBd_LOH.js             4.22 kB │ gzip:   1.86 kB
../server/router/frontend/dist/assets/diagram-S2PKOQOG.9pP8Fy4D.js                4.54 kB │ gzip:   1.89 kB
../server/router/frontend/dist/assets/Inboxes.BlrsnpG9.js                         4.72 kB │ gzip:   1.99 kB
../server/router/frontend/dist/assets/ar.Ba0UDAcK.js                              4.83 kB │ gzip:   2.09 kB
../server/router/frontend/dist/assets/Resources.BDEjH_Lc.js                       5.26 kB │ gzip:   1.74 kB
../server/router/frontend/dist/assets/graph.D7i4E-Op.js                           5.82 kB │ gzip:   1.81 kB
../server/router/frontend/dist/assets/InternalAgent.B9ynE8mq.js                   5.96 kB │ gzip:   2.10 kB
../server/router/frontend/dist/assets/diagram-QEK2KX5R.Bjea-mJI.js                6.55 kB │ gzip:   2.58 kB
../server/router/frontend/dist/assets/Chat.DdGYLFcy.js                            6.67 kB │ gzip:   2.27 kB
../server/router/frontend/dist/assets/nl.BoqyFvSh.js                              7.44 kB │ gzip:   3.04 kB
../server/router/frontend/dist/assets/hr.DhJ-GBnp.js                              8.33 kB │ gzip:   3.62 kB
../server/router/frontend/dist/assets/it.CdCRGiek.js                              8.37 kB │ gzip:   3.29 kB
../server/router/frontend/dist/assets/es.C-GYWrxW.js                              9.34 kB │ gzip:   3.65 kB
../server/router/frontend/dist/assets/hi.RUR_QiMw.js                              9.35 kB │ gzip:   2.83 kB
../server/router/frontend/dist/assets/ko.B3Hs5i-D.js                              9.69 kB │ gzip:   4.04 kB
../server/router/frontend/dist/assets/hu.Dlyxu5vl.js                             10.23 kB │ gzip:   4.11 kB
../server/router/frontend/dist/assets/stateDiagram-FKZM4ZOC.3UG2p0_5.js          10.39 kB │ gzip:   3.63 kB
../server/router/frontend/dist/assets/pl.CaG0E2JH.js                             10.44 kB │ gzip:   4.37 kB
../server/router/frontend/dist/assets/id.r3OaGgKB.js                             10.86 kB │ gzip:   4.24 kB
../server/router/frontend/dist/assets/pt-PT.DSaN3Dge.js                          11.17 kB │ gzip:   4.37 kB
../server/router/frontend/dist/assets/nb.DfF3RrOi.js                             11.19 kB │ gzip:   4.52 kB
../server/router/frontend/dist/assets/dagre-6UL2VRFP.Bbvg-gn4.js                 11.21 kB │ gzip:   4.16 kB
../server/router/frontend/dist/assets/diagram-PSM6KHXK.DXIJEFK8.js               11.60 kB │ gzip:   3.99 kB
../server/router/frontend/dist/assets/ja.DFHw3lTa.js                             11.94 kB │ gzip:   4.59 kB
../server/router/frontend/dist/assets/de.CULTuRiT.js                             12.01 kB │ gzip:   4.72 kB
../server/router/frontend/dist/assets/zh-Hant.Bh71jKTR.js                        12.18 kB │ gzip:   5.56 kB
../server/router/frontend/dist/assets/ru.BOZQNLbr.js                             12.29 kB │ gzip:   4.30 kB
../server/router/frontend/dist/assets/zh-Hans.BRa1KvrZ.js                        12.48 kB │ gzip:   5.50 kB
../server/router/frontend/dist/assets/sl.BTGhII0l.js                             12.52 kB │ gzip:   5.07 kB
../server/router/frontend/dist/assets/vi.BDfr5yuZ.js                             12.88 kB │ gzip:   4.79 kB
../server/router/frontend/dist/assets/tr.fPJ4_kLZ.js                             13.26 kB │ gzip:   5.18 kB
../server/router/frontend/dist/assets/fr.CSfo9w74.js                             13.64 kB │ gzip:   5.20 kB
../server/router/frontend/dist/assets/pt-BR.DWjekWM2.js                          14.09 kB │ gzip:   5.48 kB
../server/router/frontend/dist/assets/Playground.NTwi3g02.js                     14.32 kB │ gzip:   4.27 kB
../server/router/frontend/dist/assets/cs.ChLu2VkD.js                             14.44 kB │ gzip:   5.83 kB
../server/router/frontend/dist/assets/RagStats.CtxzFveQ.js                       14.87 kB │ gzip:   3.99 kB
../server/router/frontend/dist/assets/mr.BUIh3YIi.js                             16.81 kB │ gzip:   4.75 kB
../server/router/frontend/dist/assets/fa.CA7lG8uH.js                             16.82 kB │ gzip:   5.59 kB
../server/router/frontend/dist/assets/th.D8xeV0S1.js                             17.71 kB │ gzip:   4.76 kB
../server/router/frontend/dist/assets/uk.BtkON5VC.js                             18.13 kB │ gzip:   6.04 kB
../server/router/frontend/dist/assets/ka-GE.Bbn_B4S4.js                          18.46 kB │ gzip:   4.60 kB
../server/router/frontend/dist/assets/Tickets.Zl4aSB2y.js                        20.18 kB │ gzip:   6.43 kB
../server/router/frontend/dist/assets/mindmap-definition-VGOIOE7T.LrYB7k2i.js    20.73 kB │ gzip:   7.08 kB
../server/router/frontend/dist/assets/kanban-definition-3W4ZIXB7.BpToPeWE.js     21.07 kB │ gzip:   7.31 kB
../server/router/frontend/dist/assets/AgentSimulation.B5_RLFO6.js                21.88 kB │ gzip:   5.60 kB
../server/router/frontend/dist/assets/sankeyDiagram-TZEHDZUN.BjaaD5A2.js         22.23 kB │ gzip:   8.07 kB
../server/router/frontend/dist/assets/timeline-definition-IT6M3QCI.BO1nQ31a.js   24.08 kB │ gzip:   8.31 kB
../server/router/frontend/dist/assets/journeyDiagram-XKPGCS4Q.B9hiYO0r.js        24.13 kB │ gzip:   8.36 kB
../server/router/frontend/dist/assets/layout.SJ_o9Gl0.js                         25.22 kB │ gzip:   8.86 kB
../server/router/frontend/dist/assets/erDiagram-Q2GNP2WA.CjSGum7j.js             25.63 kB │ gzip:   8.91 kB
../server/router/frontend/dist/assets/gitGraphDiagram-NY62KEGX.rC6uMoxq.js       28.07 kB │ gzip:   7.73 kB
../server/router/frontend/dist/assets/requirementDiagram-UZGBJVZJ.BR3n1h-D.js    30.68 kB │ gzip:   9.57 kB
../server/router/frontend/dist/assets/quadrantDiagram-AYHSOK5B.DfSbP6lo.js       34.44 kB │ gzip:  10.12 kB
../server/router/frontend/dist/assets/chunk-DI55MBZ5.WxjaL0eD.js                 37.82 kB │ gzip:  12.03 kB
../server/router/frontend/dist/assets/xychartDiagram-PRI3JC2R.CIAqguEC.js        39.17 kB │ gzip:  10.95 kB
../server/router/frontend/dist/assets/chunk-B4BG7PRW.Bf1m9IZV.js                 46.08 kB │ gzip:  14.78 kB
../server/router/frontend/dist/assets/ganttDiagram-JELNMOA3.ClrccS0l.js          49.22 kB │ gzip:  16.65 kB
../server/router/frontend/dist/assets/Setting.DmQ2tVxO.js                        60.92 kB │ gzip:  12.34 kB
../server/router/frontend/dist/assets/flowDiagram-NV44I4VS.Cb1XJ0UO.js           61.82 kB │ gzip:  19.69 kB
../server/router/frontend/dist/assets/c4Diagram-YG6GDRKO.7prPg7sl.js             70.26 kB │ gzip:  19.72 kB
../server/router/frontend/dist/assets/blockDiagram-VD42YOAC.QjQMcrFp.js          74.60 kB │ gzip:  20.93 kB
../server/router/frontend/dist/assets/cose-bilkent-S5V4N54A.1uvchc-U.js          81.92 kB │ gzip:  22.55 kB
../server/router/frontend/dist/assets/AgentAdmin.BQUMHndh.js                     83.17 kB │ gzip:  17.33 kB
../server/router/frontend/dist/assets/sequenceDiagram-WL72ISMW.CAPZVjak.js       99.43 kB │ gzip:  27.06 kB
../server/router/frontend/dist/assets/utils-vendor.CGdFPbZN.js                  102.32 kB │ gzip:  32.15 kB
../server/router/frontend/dist/assets/MemoDetail.DnVHybN2.js                    138.38 kB │ gzip:  44.76 kB
../server/router/frontend/dist/assets/architectureDiagram-VXUJARFQ.DNgD0zla.js  149.77 kB │ gzip:  42.25 kB
../server/router/frontend/dist/assets/leaflet-vendor.DiwMhoWM.js                153.53 kB │ gzip:  44.78 kB
../server/router/frontend/dist/assets/react-vendor.DhcPx5gP.js                  229.69 kB │ gzip:  75.16 kB
../server/router/frontend/dist/assets/katex-vendor.BheORXjY.js                  265.70 kB │ gzip:  77.48 kB
../server/router/frontend/dist/assets/treemap-KMMF4GRG.DMOZCPKg.js              330.37 kB │ gzip:  79.66 kB
../server/router/frontend/dist/assets/mui-vendor.DBGrrmLh.js                    414.38 kB │ gzip: 114.78 kB
../server/router/frontend/dist/assets/cytoscape.esm.DXpMYzf1.js                 442.86 kB │ gzip: 141.93 kB
../server/router/frontend/dist/assets/mermaid-vendor.ByX0sv4o.js                550.50 kB │ gzip: 155.62 kB
../server/router/frontend/dist/assets/app.CrFr5mJW.js                           801.83 kB │ gzip: 223.06 kB
../server/router/frontend/dist/assets/highlight-vendor.B0a3fjPT.js              970.35 kB │ gzip: 311.90 kB

(!) Some chunks are larger than 500 kB after minification. Consider:
- Using dynamic import() to code-split the application
- Use build.rollupOptions.output.manualChunks to improve chunking: https://rollupjs.org/configuration-options/#output-manualchunks
- Adjust chunk size limit for this warning via build.chunkSizeWarningLimit.
✓ built in 38.40s
task: [build:backend:rag] mkdir -p build
task: [build:backend:rag] go build -tags rag -o build/memos ./bin/memos/main.go
task: [run:rag] if [ -f .env ]; then
  echo "Loading environment from .env file..."
  set -a && . .env && set +a
fi
FORCE_REINDEX_ON_STARTUP=false RAG_PIPELINE_ENABLED=true EMBEDDING_MODEL=openai/text-embedding-3-small LANCEDB_STORAGE_PROVIDER=local ./build/memos --mode dev --data /home/chaschel/Documents/go/bchat/build/data

Loading environment from .env file...
2026/07/23 05:57:15 INFO OpenRouter API key loaded prefix=sk-or-v1-2...
2026/07/23 05:57:15 INFO Column already exists, skipping table=tickets column=type
2026/07/23 05:57:15 INFO Column already exists, skipping table=tickets column=tags
2026/07/23 05:57:15 INFO start migration currentSchemaVersion=0.31.3 targetSchemaVersion=0.33.2
2026/07/23 05:57:15 WARN migration: column already exists, skipping error="SQL logic error: table activity already exists (1)"
2026/07/23 05:57:15 WARN migration: column already exists, skipping error="SQL logic error: duplicate column name: avatar_url (1)"
2026/07/23 05:57:15 WARN migration: column already exists, skipping error="SQL logic error: table idp already exists (1)"
2026/07/23 05:57:15 WARN migration: column already exists, skipping error="SQL logic error: table memo_relation already exists (1)"
2026/07/23 05:57:15 ERROR failed to migrate error="SQL logic error: no such column: external_link (1)\nfailed to execute statement\ngithub.com/usememos/memos/store.(*Store).execute\n\t/home/chaschel/Documents/go/bchat/store/migrator.go:337\ngithub.com/usememos/memos/store.(*Store).Migrate\n\t/home/chaschel/Documents/go/bchat/store/migrator.go:135\nmain.init.func1\n\t/home/chaschel/Documents/go/bchat/bin/memos/main.go:98\ngithub.com/spf13/cobra.(*Command).execute\n\t/home/chaschel/go/pkg/mod/github.com/spf13/cobra@v0.0.3/command.go:766\ngithub.com/spf13/cobra.(*Command).ExecuteC\n\t/home/chaschel/go/pkg/mod/github.com/spf13/cobra@v0.0.3/command.go:852\ngithub.com/spf13/cobra.(*Command).Execute\n\t/home/chaschel/go/pkg/mod/github.com/spf13/cobra@v0.0.3/command.go:800\nmain.main\n\t/home/chaschel/Documents/go/bchat/bin/memos/main.go:312\nruntime.main\n\t/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/runtime/proc.go:290\nruntime.goexit\n\t/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/runtime/asm_amd64.s:1771\nmigrate error: DROP TABLE IF EXISTS resource_temp;\n\nCREATE TABLE resource_temp (\n  id INTEGER PRIMARY KEY AUTOINCREMENT,\n  creator_id INTEGER NOT NULL,\n  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),\n  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),\n  filename TEXT NOT NULL DEFAULT '',\n  blob BLOB DEFAULT NULL,\n  external_link TEXT NOT NULL DEFAULT '',\n  type TEXT NOT NULL DEFAULT '',\n  size INTEGER NOT NULL DEFAULT 0,\n  internal_path TEXT NOT NULL DEFAULT ''\n);\n\nINSERT INTO\n  resource_temp (id, creator_id, created_ts, updated_ts, filename, blob, external_link, type, size, internal_path)\nSELECT\n  id, creator_id, created_ts, updated_ts, filename, blob, external_link, type, size, internal_path\nFROM\n  resource;\n\nDROP TABLE resource;\n\nALTER TABLE resource_temp RENAME TO resource;\n\ngithub.com/usememos/memos/store.(*Store).Migrate\n\t/home/chaschel/Documents/go/bchat/store/migrator.go:136\nmain.init.func1\n\t/home/chaschel/Documents/go/bchat/bin/memos/main.go:98\ngithub.com/spf13/cobra.(*Command).execute\n\t/home/chaschel/go/pkg/mod/github.com/spf13/cobra@v0.0.3/command.go:766\ngithub.com/spf13/cobra.(*Command).ExecuteC\n\t/home/chaschel/go/pkg/mod/github.com/spf13/cobra@v0.0.3/command.go:852\ngithub.com/spf13/cobra.(*Command).Execute\n\t/home/chaschel/go/pkg/mod/github.com/spf13/cobra@v0.0.3/command.go:800\nmain.main\n\t/home/chaschel/Documents/go/bchat/bin/memos/main.go:312\nruntime.main\n\t/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/runtime/proc.go:290\nruntime.goexit\n\t/home/chaschel/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/runtime/asm_amd64.s:1771"

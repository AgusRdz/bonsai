# Changelog

All notable changes to bonsai are documented here.

## [0.58.1] - 2026-05-17

### Bug Fixes
- PR panel stuck on loading when no PRs exist or fetch fails
([1c0d11b](https://github.com/AgusRdz/bonsai/commit/1c0d11b1f622aef7f6893a0a15549373f8685dea))

### Documentation
- Document line-level staging in hunk panel
([73958c6](https://github.com/AgusRdz/bonsai/commit/73958c6e6160d002eb6badc047fea8188a909f19))
## [0.58.0] - 2026-05-17

### Features
- Add line-level staging within hunk mode
([c6d16c6](https://github.com/AgusRdz/bonsai/commit/c6d16c6d1c33a77b9ebf83a3d0340724d452d330))
## [0.57.1] - 2026-05-17

### Documentation
- Document MCP commands and AI structured output commands
([a9a8504](https://github.com/AgusRdz/bonsai/commit/a9a8504bfb2624e6ee16f1ed298349259967b9be))
## [0.57.0] - 2026-05-17

### Features
- Add MCP instructions for code review tool preference
([5ad7ee8](https://github.com/AgusRdz/bonsai/commit/5ad7ee8898698be3c8c199b25f1b691d8ef5ad6f))
## [0.56.3] - 2026-05-17

### Bug Fixes
- Write user-scope MCP to ~/.claude.json with correct stdio format
([a2bd359](https://github.com/AgusRdz/bonsai/commit/a2bd359efd67319112c00af570480722db20a6df))
## [0.56.2] - 2026-05-17

### Bug Fixes
- Use absolute binary path in MCP config for GUI app compatibility
([54af93a](https://github.com/AgusRdz/bonsai/commit/54af93ae629ccbb4ada320ac43f888e44a79f473))
- Update existing bonsai MCP entry if command path changed
([b671a70](https://github.com/AgusRdz/bonsai/commit/b671a702d5dd8bd5ead5dceeed8282d2a7b2f618))
## [0.56.1] - 2026-05-17

### Bug Fixes
- Use -- prefix for mcp subcommands and add uninstall to help
([6e104e7](https://github.com/AgusRdz/bonsai/commit/6e104e71ada1517614e7a84c0cb8a90f8b64166e))

### Miscellaneous
- Remove .mcp.json from tracking and add to .gitignore
([fedec71](https://github.com/AgusRdz/bonsai/commit/fedec71e01e3030854890641921253b38bddb3ef))
## [0.56.0] - 2026-05-17

### Features
- Add bonsai mcp uninstall and fix install feedback
([34d5be2](https://github.com/AgusRdz/bonsai/commit/34d5be21de3e74bb03b6f0f59c618027d6149f25))
## [0.55.16] - 2026-05-17

### Features
- Add bonsai mcp install command
([f8c5faf](https://github.com/AgusRdz/bonsai/commit/f8c5faf8001ac47f5de99f14dadad446c1901338))
## [0.55.15] - 2026-05-17

### Features
- Add MCP stdio server (bonsai mcp)
([954059f](https://github.com/AgusRdz/bonsai/commit/954059fa3ec588c3ba73afadadc129910572c026))
## [0.55.14] - 2026-05-17

### Features
- Add bonsai context command for single-call repo snapshot
([4af8c02](https://github.com/AgusRdz/bonsai/commit/4af8c02a1419db197ec597a7bcde707d947712d5))
## [0.55.13] - 2026-05-17

### Features
- Accept --format=md as alias for --format=markdown
([b837336](https://github.com/AgusRdz/bonsai/commit/b837336c2ad7333e79d2d711b60f84740b6f323b))
## [0.55.12] - 2026-05-17

### Features
- Add markdown and xml output formatters for agent commands
([cca68f9](https://github.com/AgusRdz/bonsai/commit/cca68f92ce0f45806009c0573344a89b028b6ff7))
## [0.55.11] - 2026-05-17

### Bug Fixes
- Two-step agent config - ask use/no then format choice
([2619933](https://github.com/AgusRdz/bonsai/commit/261993350ebd2cc8716f81843f22b7092c428875))
## [0.55.10] - 2026-05-17

### Bug Fixes
- Use global config as baseline for --local first run
([76f3201](https://github.com/AgusRdz/bonsai/commit/76f3201adaa34e6a28e96e43eeca00f30c5dd07a))
## [0.55.9] - 2026-05-17

### Features
- Progressive disclosure for diff/review/show; add bonsai show command
([b5c039d](https://github.com/AgusRdz/bonsai/commit/b5c039d7338073bee90a5625ba26dfa2b4e20266))
## [0.55.8] - 2026-05-17

### Features
- Add numstat, show, and log-opts methods for agent progressive disclosure
([15e2998](https://github.com/AgusRdz/bonsai/commit/15e2998df1575ea2b45a1ca9772898c2f3759090))
## [0.55.7] - 2026-05-17

### Features
- Add [agent] section with default_format and setup wizard prompt
([01b57a2](https://github.com/AgusRdz/bonsai/commit/01b57a20a0a7309b3e351c759b49a405116d35fb))
## [0.55.6] - 2026-05-17

### Refactoring
- Remove context command, rename review-context to review
([17b95d0](https://github.com/AgusRdz/bonsai/commit/17b95d06b994059660cd416462b381b4b9a7858c))
## [0.55.5] - 2026-05-17

### Bug Fixes
- Sanitize remote URL input in remote add panel
([4e2b8d0](https://github.com/AgusRdz/bonsai/commit/4e2b8d0d32cd370f737a3523a48570aefb5bee61))

### Features
- Add structured JSON output commands for agent consumption
([016ba31](https://github.com/AgusRdz/bonsai/commit/016ba31cacdf0f1a166b64d745f6d6ed3ca136b9))
## [0.55.3] - 2026-05-17

### Features
- Add [+] stage all to command bar, simplify default bar items
([4ec533f](https://github.com/AgusRdz/bonsai/commit/4ec533fbc82ec72696b015b61d1bc67ea4fceb8a))
## [0.55.2] - 2026-05-17

### Bug Fixes
- Initialize textinput models before focus in init panel remote add
([74c4c75](https://github.com/AgusRdz/bonsai/commit/74c4c7538349257d2f9f2bfb63dc5b524e5d9d51))

### Documentation
- Document init panel for non-git directories
([bd18da2](https://github.com/AgusRdz/bonsai/commit/bd18da2869f492a6f32fa8a06e7da4f4fac4951e))
## [0.55.1] - 2026-05-17

### Features
- Init panel when bonsai is opened outside a git repository
([dc4cb32](https://github.com/AgusRdz/bonsai/commit/dc4cb32b945a2f15c7b77a97c93c131a6933f9e7))
## [0.55.0] - 2026-05-17

### Documentation
- Add bonsai logo to README
([d297de1](https://github.com/AgusRdz/bonsai/commit/d297de1529471cf504eaf2730d37ed27ec8dc947))
- Update tagline to match website
([8af115a](https://github.com/AgusRdz/bonsai/commit/8af115aa5aa9a840cf16adf3b2513d3d166e2efe))

### Features
- Stage all, fix branch delete, update tagline
([51506cc](https://github.com/AgusRdz/bonsai/commit/51506ccd90a5ee31e022c6011c235153b5b4e087))
## [0.54.1] - 2026-05-17

### Documentation
- Sync README and docs with current feature set
([df1a090](https://github.com/AgusRdz/bonsai/commit/df1a09040974861629839f177483a216b3bf52d2))
## [0.54.0] - 2026-05-17

### Features
- Inline PR diff comments and complete interactive rebase
([810f7df](https://github.com/AgusRdz/bonsai/commit/810f7dfb1f12b8c213112c793930521c2caa2332))
## [0.53.2] - 2026-05-17

### Bug Fixes
- Use -- prefix for all subcommands (patch, bundle, ssh, lfs, repo)
([a5da3a9](https://github.com/AgusRdz/bonsai/commit/a5da3a9ef153b9ca8dec90742bf7c60725ee0429))
## [0.53.1] - 2026-05-17

### Miscellaneous
- Untrack docs/oauth-architecture.md (planning doc, not for repo)
([80c3d30](https://github.com/AgusRdz/bonsai/commit/80c3d30fbe5445e6c927920fc58afdf0f4a4b15d))
- Remove auth package - delegate credential management to gh/glab
([12668f3](https://github.com/AgusRdz/bonsai/commit/12668f3a9da4084917aa96917fc4cef38a33088b))
## [0.53.0] - 2026-05-17

### Features
- OAuth token storage and device flow for bonsai auth CLI
([0dbdd33](https://github.com/AgusRdz/bonsai/commit/0dbdd331950da752490a1a3ae001aafb9641770c))
## [0.52.0] - 2026-05-16

### Features
- Conflict editor with live result preview and manual edit mode
([3d525f7](https://github.com/AgusRdz/bonsai/commit/3d525f7d9884f0451779d4feda8760c6fbf167cc))
## [0.51.0] - 2026-05-16

### Features
- LFS UI with pull, push, track, untrack, and install support
([4cb715d](https://github.com/AgusRdz/bonsai/commit/4cb715d481cb51d49e270c2be537f8e7f6ce0969))
## [0.50.0] - 2026-05-16

### Features
- Multi-repo dashboard with config-driven repo list
([fb5d0b0](https://github.com/AgusRdz/bonsai/commit/fb5d0b0623dc0b489d5d8ef4d5138fab456c7742))
## [0.49.0] - 2026-05-16

### Features
- Diff syntax highlighting via chroma
([888bf08](https://github.com/AgusRdz/bonsai/commit/888bf08e1c346bb2e258796df120c5d0a670cd5c))
## [0.48.0] - 2026-05-16

### Features
- LFS support, SSH key manager TUI, and LFS pointer detection in diffs
([38ed52e](https://github.com/AgusRdz/bonsai/commit/38ed52ecfd4e35ee6f50f858efe539e3389233e5))
## [0.47.0] - 2026-05-16

### Features
- Colored branch graph and Bitbucket PR review workflow
([a17d3b7](https://github.com/AgusRdz/bonsai/commit/a17d3b74431021e84ec93872555e75cd12a996e9))
## [0.46.0] - 2026-05-16

### Features
- Issues panel, bonsai repo create/open
([55f44ae](https://github.com/AgusRdz/bonsai/commit/55f44ae8a7244bc81bcf7eb29e4b83cd9f84076a))
- Three-way merge editor with base (common ancestor) in conflict view
([51a8f63](https://github.com/AgusRdz/bonsai/commit/51a8f63b063f4e78b2254e86d18c7388f92d8941))
## [0.45.0] - 2026-05-16

### Features
- PR review workflow, draft/metadata badges, CI in list
([e58da6b](https://github.com/AgusRdz/bonsai/commit/e58da6bae5f03e87bd57dcc6eaa8cf8b52c8eded))
## [0.44.0] - 2026-05-16

### Features
- Branch protection awareness and gitflow finish automation
([4d5035a](https://github.com/AgusRdz/bonsai/commit/4d5035a1a63d0afec3a752226d5b8f009b3fb7ad))
## [0.43.0] - 2026-05-16

### Features
- Search filter in all list panels, sig verification badge, binary diff highlight
([f230896](https://github.com/AgusRdz/bonsai/commit/f230896082f6a55eff0f907c1930f0454f2bf4c1))
## [0.42.2] - 2026-05-16

### Bug Fixes
- Standup null-byte crash and setup re-run shows current values
([200a632](https://github.com/AgusRdz/bonsai/commit/200a6321b64d257be54132b2ae6875c1964ef95f))
## [0.42.1] - 2026-05-16

### Documentation
- Reorganize help text - groups, consistent lfs subcommands, cleaner alignment
([40033fc](https://github.com/AgusRdz/bonsai/commit/40033fceabdf05a1eab6de899b4fb959e1a07cbc))
## [0.42.0] - 2026-05-16

### Features
- Auto-refresh every 2s and bonsai standup command
([1f1781f](https://github.com/AgusRdz/bonsai/commit/1f1781f61526e26ef89635a175bbc8c72d8eca83))
## [0.41.0] - 2026-05-16

### Features
- GPG signing, LFS CLI, one-click undo, PR diff, fork, metrics CLI
([efb5cf5](https://github.com/AgusRdz/bonsai/commit/efb5cf5771e61aef614242acf08a55191bf7d486))
## [0.40.0] - 2026-05-16

### Features
- General plugin system for git event hooks
([acc76c0](https://github.com/AgusRdz/bonsai/commit/acc76c052f3eed116bbe323f8e584167c54eabe6))
## [0.39.0] - 2026-05-16

### Features
- Suggest PR after push, inline conflict editor, local metrics
([4254396](https://github.com/AgusRdz/bonsai/commit/4254396dd0b5f4de7a77a3773486ce081a53a270))
## [0.38.0] - 2026-05-16

### Features
- Multi-provider PR integration layer with plugin support
([55d3f8b](https://github.com/AgusRdz/bonsai/commit/55d3f8bf081ce1bdc16b55c327e9458351e5c891))
## [0.37.4] - 2026-05-16

### Bug Fixes
- Show only non-obvious actions in file hint (space/diff already in command bar)
([b04794d](https://github.com/AgusRdz/bonsai/commit/b04794d66543a60591e2e9740768995f3ec04d27))
## [0.37.3] - 2026-05-16

### Bug Fixes
- Always show file action hint as dedicated footer line, drop 'hunks' jargon
([d383d97](https://github.com/AgusRdz/bonsai/commit/d383d97db67673538c66b6bb7065b1e810531a6a))
## [0.37.2] - 2026-05-16

### Features
- Show contextual file action hints in footer when file is selected
([1360c68](https://github.com/AgusRdz/bonsai/commit/1360c681073925fce43190180f88959521e1e0ed))
## [0.37.1] - 2026-05-16

### Features
- Delete untracked files from disk and git rm --cached from TUI
([8ef5cdf](https://github.com/AgusRdz/bonsai/commit/8ef5cdf5b08dadce50f1797b43d055c711312e87))
## [0.37.0] - 2026-05-16

### Features
- Human-readable feedback for all major git operations
([25c03cf](https://github.com/AgusRdz/bonsai/commit/25c03cfd8be8d7eab2f9e90e0ce1bb63f9e1c890))
## [0.36.3] - 2026-05-16

### Features
- Show human-readable feedback after pull/rebase/merge
([4a17172](https://github.com/AgusRdz/bonsai/commit/4a1717263e5997dcb8d091d663c995e38aa0f4d2))
## [0.36.2] - 2026-05-16

### Features
- Show rebase/merge choice when pull would diverge
([e6e6a4e](https://github.com/AgusRdz/bonsai/commit/e6e6a4ee87ea2e6fb1f9774a436ed8feab56144b))
## [0.36.1] - 2026-05-16

### Performance
- Replace O(n²) sorts with sort.Slice, cap unbounded git log in Stats
([cbb07bf](https://github.com/AgusRdz/bonsai/commit/cbb07bf4646a111079584b69eec0e6aa0f94c48a))
## [0.36.0] - 2026-05-16

### Features
- Configurable command bar - toggle shortcuts via [C] config or command_bar.items in TOML
([1596a32](https://github.com/AgusRdz/bonsai/commit/1596a320b174524d4014ab2402af1f93a9d79c95))
## [0.35.9] - 2026-05-16

### Features
- Simplify command bar to day-to-day shortcuts, move rest to [?] help
([f238f8f](https://github.com/AgusRdz/bonsai/commit/f238f8fbebf11a8c5864eb29437a2224e4598103))
## [0.35.8] - 2026-05-16

### Refactoring
- Extract isGitVersionSupported/parseSizePack, add masteryThreshold/isProComplex/ResolveEditor/hashFile/replaceBinary/setup/contextTip tests
([40e813c](https://github.com/AgusRdz/bonsai/commit/40e813cddaaf1e7e585cc11de07361454ee0d0f7))
## [0.35.7] - 2026-05-16

### Refactoring
- Extract parseBlamePorcelain, add isAllHex/fileCode/commitDetail/render tests
([d5594d5](https://github.com/AgusRdz/bonsai/commit/d5594d590d9c6528e7690a095ef4c121e1861230))
## [0.35.6] - 2026-05-16

### Refactoring
- Extract parseDiffOutput, add commandKey/build*/format tests
([f40ac6c](https://github.com/AgusRdz/bonsai/commit/f40ac6c574ab552ecff61858f5f2713c87d36bb4))
## [0.35.5] - 2026-05-16

### Refactoring
- Extract inline parsers, add reflog/remote/flow tests
([8cc8263](https://github.com/AgusRdz/bonsai/commit/8cc8263944707d32ca111ba0416166d787ef5d30))
## [0.35.4] - 2026-05-16

### Bug Fixes
- Correct log panel docs, add parser tests, parseLogFilter tests
([5d9bb57](https://github.com/AgusRdz/bonsai/commit/5d9bb579dada7b62ea0bb34cfc52fc80711cb9e4))
## [0.35.3] - 2026-05-16

### Bug Fixes
- Ordering bugs in explain(), new tests, tui-guide hunk/history/graph
([027dbd1](https://github.com/AgusRdz/bonsai/commit/027dbd1a98d81f52443a2959ac82f3032d4cbc82))
## [0.35.2] - 2026-05-16

### Bug Fixes
- Stash footer, help panel, command bar, usage tests, README
([4efdb05](https://github.com/AgusRdz/bonsai/commit/4efdb05b947518609a36482b0fb0d241706f1926))
## [0.35.1] - 2026-05-16

### Bug Fixes
- Test failures, education gaps, and doc refresh
([b8eda30](https://github.com/AgusRdz/bonsai/commit/b8eda307653864e35ec41e2e96e0b66f61d4daae))
## [0.35.0] - 2026-05-16

### Features
- Remote branch delete, upstream in branch list, fix dead panelClean, docs refresh
([03ac1a7](https://github.com/AgusRdz/bonsai/commit/03ac1a7feb377313a5d84cd353cd5f681f542d25))
## [0.34.0] - 2026-05-16

### Features
- Detect SSH remote host dynamically for connectivity checks
([bee3639](https://github.com/AgusRdz/bonsai/commit/bee36392590ec559a07a1cc4d8f029b2e0be319d))
## [0.33.0] - 2026-05-16

### Features
- Branch delete/rename, stash apply/drop, tag push, SSH checks, bonsai ssh CLI
([a829800](https://github.com/AgusRdz/bonsai/commit/a829800ccf39b435e12c83c5407f70a6444962a6))
## [0.32.0] - 2026-05-16

### Features
- File history, branch graph, stash message, commit diff, clone, education manager
([12cd31b](https://github.com/AgusRdz/bonsai/commit/12cd31b79cff41bb351a93f82cb502184ed7038e))
## [0.31.1] - 2026-05-16

### Bug Fixes
- Limit pro mode education override to complex commands only
([e9b9c51](https://github.com/AgusRdz/bonsai/commit/e9b9c51c4a2e1b4d5712639e9e50f92cc653b31c))
## [0.31.0] - 2026-05-16

### Features
- Adaptive education panel with per-command usage tracking
([979d290](https://github.com/AgusRdz/bonsai/commit/979d29050e35a083c8b12cab5a1c05223cd89b4c))
## [0.30.0] - 2026-05-16

### Features
- Hunk-level staging and push options menu
([870e711](https://github.com/AgusRdz/bonsai/commit/870e711f7001008c64c81ecd6a21f91b40ebfb69))
## [0.29.2] - 2026-05-16

### Bug Fixes
- Group stats contributors by email to collapse duplicate identities
([d35bfb0](https://github.com/AgusRdz/bonsai/commit/d35bfb041ac2d715419834cace3193e2e62612a6))
## [0.29.1] - 2026-05-16

### Documentation
- Rewrite README and add full wiki in docs/
([ebbdf60](https://github.com/AgusRdz/bonsai/commit/ebbdf6030272195b0212b37ef5cde1d08e1c69de))
## [0.29.0] - 2026-05-16

### Features
- Complete git coverage - restore, reflog, fetch, clean, remotes, submodules, notes, patch, archive, bundle, stats
([08bd1a1](https://github.com/AgusRdz/bonsai/commit/08bd1a10d2d17fba84e83741111c31f1da04ca69))
## [0.28.3] - 2026-05-16

### Refactoring
- Standardize CLI subcommands to use flags
([c8cd9ac](https://github.com/AgusRdz/bonsai/commit/c8cd9ac47ae2d1406b5c397dfef6bf15d66b2f4b))
## [0.28.2] - 2026-05-16

### Features
- Add --verbose flag to bonsai doctor
([0e1ab91](https://github.com/AgusRdz/bonsai/commit/0e1ab91ebf702ca8dff99642e7bd206e0de5acb5))
## [0.28.1] - 2026-05-16

### Features
- Add bonsai doctor health check command
([9b69e33](https://github.com/AgusRdz/bonsai/commit/9b69e33fcf6fd1931c0066053f2243ff11cd8119))
## [0.28.0] - 2026-05-16

### Features
- Configuration manager panel (block 7)
([7c3ea9d](https://github.com/AgusRdz/bonsai/commit/7c3ea9d1b0191e9bdeb48043133ed6a678a45688))
## [0.27.0] - 2026-05-16

### Features
- Amend last commit (message, author, date, no-edit)
([f7c9255](https://github.com/AgusRdz/bonsai/commit/f7c925562ec4ecf4a571a546658318202c8692e7))
## [0.26.0] - 2026-05-16

### Features
- Interactive rebase panel (block 6)
([b11b4d6](https://github.com/AgusRdz/bonsai/commit/b11b4d6d17233fa36f2f007b821d5c0b9ce7fe9a))
## [0.25.0] - 2026-05-16

### Features
- Git bisect panel (block 5)
([7d02fb8](https://github.com/AgusRdz/bonsai/commit/7d02fb8ef025ea6764f0c8c015ef1d4223294e28))
## [0.24.0] - 2026-05-16

### Features
- Git blame panel (block 4)
([e8c114c](https://github.com/AgusRdz/bonsai/commit/e8c114cd0db18097962c58fc6182c343040e2045))
## [0.23.0] - 2026-05-16

### Features
- Worktree list, add, and remove (block 3)
([9c2cfef](https://github.com/AgusRdz/bonsai/commit/9c2cfef3cf4ac77900fe8eeb6cc231dd7b2e2ddd))
## [0.22.0] - 2026-05-16

### Features
- Linear rebase with continue and abort (block 2)
([3ed8365](https://github.com/AgusRdz/bonsai/commit/3ed83655f35c18381a07eee1d154e0e450066bb2))
## [0.21.1] - 2026-05-16

### Bug Fixes
- Hide scroll position indicator when content fits on screen
([ef360bf](https://github.com/AgusRdz/bonsai/commit/ef360bf5a3c1583d3af502673ccc5d26a41c5935))
## [0.21.0] - 2026-05-16

### Features
- Reset, merge, cherry-pick, and tags (block 1)
([5ad3837](https://github.com/AgusRdz/bonsai/commit/5ad3837bbc673ccb483ce7e724739a8ef83dba9c))
## [0.20.2] - 2026-05-16

### Bug Fixes
- Restore [l] log shortcut in command bar
([11bbe49](https://github.com/AgusRdz/bonsai/commit/11bbe491b6ed15c906951f9215aea0730d27a3d1))
## [0.20.1] - 2026-05-16

### Bug Fixes
- Use ASCII record separator in git show format instead of null byte
([61d1867](https://github.com/AgusRdz/bonsai/commit/61d186702704434f26b123ce205a0023c8823895))
## [0.20.0] - 2026-05-16

### Features
- Rename and differentiate modes (novice→guided, learning→standard)
([79e7ca1](https://github.com/AgusRdz/bonsai/commit/79e7ca13728ba0349a8420589233bf3d80afbead))
## [0.19.1] - 2026-05-16

### Bug Fixes
- Restore keybinding defaults when config has empty strings
([bd30a02](https://github.com/AgusRdz/bonsai/commit/bd30a029d8fe0a7ad264cdce0f02148958282a81))
## [0.19.0] - 2026-05-16

### Features
- Merge conflict detection and interactive resolution panel
([57febf1](https://github.com/AgusRdz/bonsai/commit/57febf1bccbd604c7fcf2aa03f86833c4b2963dc))
## [0.18.0] - 2026-05-16

### Features
- Log pagination and search filter
([18cc3b4](https://github.com/AgusRdz/bonsai/commit/18cc3b41615c1065f3d8a95be4ae9b99924c1a8f))
## [0.17.0] - 2026-05-16

### Features
- Commit detail panel and log limit increase
([c172207](https://github.com/AgusRdz/bonsai/commit/c172207766014b0427c6f3913a8a44163afe3cb5))
## [0.16.0] - 2026-05-16

### Features
- Interactive setup wizard for global and per-project config
([43c7459](https://github.com/AgusRdz/bonsai/commit/43c745930bec1563c7dad095686adecbbd51ed3d))
## [0.15.1] - 2026-05-16

### Performance
- Reduce git status from 4 subprocess calls to 1
([3502983](https://github.com/AgusRdz/bonsai/commit/350298377c13d56eb95bf0c34040d8b1cbe02785))
## [0.15.0] - 2026-05-16

### Features
- Polish, discoverability, and config commands
([db6b024](https://github.com/AgusRdz/bonsai/commit/db6b024677c7ef00ac2fe5564d3eab7bc367f414))
## [0.14.0] - 2026-05-16

### Features
- Flow-aware branch picker and contextual workflow hints
([5c59a18](https://github.com/AgusRdz/bonsai/commit/5c59a18890e35686354ebe64a375ca2ba374c894))
## [0.13.0] - 2026-05-16

### Features
- Discard working tree changes with confirmation panel
([2a2c2fc](https://github.com/AgusRdz/bonsai/commit/2a2c2fc0f14b5b570cc69b373d6a74335957e3dd))
## [0.12.0] - 2026-05-16

### Features
- Contextual tips for novice and learning modes
([3b77930](https://github.com/AgusRdz/bonsai/commit/3b77930fe12a46916578e18d165e8796fa2ed35e))
## [0.11.0] - 2026-05-16

### Features
- Help panel and condensed command bar
([ad91155](https://github.com/AgusRdz/bonsai/commit/ad91155e24376a691f5aa01ddfd1e8190be650da))
## [0.10.0] - 2026-05-16

### Features
- Stash push and pop with stash list panel
([c6daa13](https://github.com/AgusRdz/bonsai/commit/c6daa13dcee74b0f748fedf30a10299d3ef5e6ca))
## [0.9.0] - 2026-05-16

### Features
- Diff view for staged and unstaged files
([8e38e0e](https://github.com/AgusRdz/bonsai/commit/8e38e0e508e0d5f25d1d4a20f382a6522ef3392f))
## [0.8.0] - 2026-05-16

### Features
- Branch switcher panel
([425787f](https://github.com/AgusRdz/bonsai/commit/425787f1b73674526eec194d090869f0dede6b81))
## [0.7.0] - 2026-05-16

### Features
- Pull command, commit log panel, and ahead/behind indicator
([91afb1e](https://github.com/AgusRdz/bonsai/commit/91afb1e65ae7851fb620a6bb32fefea53828057d))
## [0.6.0] - 2026-05-16

### Features
- Branch convention validation with warn/strict modes
([6cbb360](https://github.com/AgusRdz/bonsai/commit/6cbb360f15aa90c0b5422813bfea31526e9da91b))
## [0.5.0] - 2026-05-16

### Features
- Education panel - plain-language explanations after every action
([ec10161](https://github.com/AgusRdz/bonsai/commit/ec10161c9da83a96e5cc8ed01f66badd1ab3ee4e))
## [0.4.0] - 2026-05-16

### Features
- Git wrapper package and interactive TUI
([c99d2f8](https://github.com/AgusRdz/bonsai/commit/c99d2f8e714e8b46e777b2302b18eefb872db3f0))
## [0.3.1] - 2026-05-16

### Bug Fixes
- Code review - context timeout, config validation, error messages
([5887b1b](https://github.com/AgusRdz/bonsai/commit/5887b1b0f9a498ce73424e24c73123ea9610ef8f))
## [0.3.0] - 2026-05-16

### Features
- Add config system and bubbletea TUI default view
([3e5d1cd](https://github.com/AgusRdz/bonsai/commit/3e5d1cd00ad1def23231c68e3e3f4bf98de74acd))
## [0.2.1] - 2026-05-16

### Bug Fixes
- Always print git version at startup, not only when update is available
([d0f342c](https://github.com/AgusRdz/bonsai/commit/d0f342c7b17b058fa57178f47b06d33ba2500ce7))
## [0.2.0] - 2026-05-16

### Features
- Check git installation and suggest updates at startup
([d47d54a](https://github.com/AgusRdz/bonsai/commit/d47d54a87c2bafc6de37ae412160f402a76caf47))

### Miscellaneous
- Install git-cliff inside dev container
([951b718](https://github.com/AgusRdz/bonsai/commit/951b718266223d055c9394c993802c8d8e17ab14))
## [0.1.0] - 2026-05-16

### Bug Fixes
- Address code review findings in updater, main, and keygen
([8fe48f8](https://github.com/AgusRdz/bonsai/commit/8fe48f8700d0debf7df8c2e136ad10f4621be62f))

### CI/CD
- Add CI and release pipeline with Ed25519 signing and attestations
([7f41476](https://github.com/AgusRdz/bonsai/commit/7f4147677b7389b0cb417f4b8a980b9d18bf9bb4))

### Features
- Add CLI entrypoint with core commands
([8c3e0fe](https://github.com/AgusRdz/bonsai/commit/8c3e0fe4b03e0dd47c9b91df3a09e0f08de41518))
- Add self-update with Ed25519 verification and keygen utility
([14acb44](https://github.com/AgusRdz/bonsai/commit/14acb44ff219c9fae4a588d021189496e8cdec09))
- Add install scripts with automatic PATH registration
([84c0bd1](https://github.com/AgusRdz/bonsai/commit/84c0bd1662e097059218a611452179dd7d54c658))

### Miscellaneous
- Bootstrap project with Go module and build tooling
([ec0075d](https://github.com/AgusRdz/bonsai/commit/ec0075de17227a8aa647a463a1a25d2254d36d9d))
- Add golangci-lint and pre-commit hook
([43644a2](https://github.com/AgusRdz/bonsai/commit/43644a290cd68283ea8066042886821a4ee55530))

### Testing
- Add tests for updater and keygen packages
([43acd6e](https://github.com/AgusRdz/bonsai/commit/43acd6ee5cf5595f9ab854509e8edc61dbc01b96))


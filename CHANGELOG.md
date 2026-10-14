# Changelog

## [2.2.0](https://github.com/bonboncinnabon/sentei/compare/v2.1.0...v2.2.0) (2026-10-14)


### Features

* **remove:** remove one named worktree with --branch ([4bad761](https://github.com/bonboncinnabon/sentei/commit/4bad761b9c1381cd9e3f866c79de7fbbf8055141))


### Bug Fixes

* **creator:** keep install commands from prompting for git credentials ([39b1e3d](https://github.com/bonboncinnabon/sentei/commit/39b1e3df658221f87c5d6bdfec65e6ea5d15e596))
* **git:** run clone's ssh in batch mode past includes that name no ssh command ([26f2104](https://github.com/bonboncinnabon/sentei/commit/26f210426c3f0a78207112cf8b66ca4689adcbb4))
* say how to reach a bare layout git will not enter ([d5ba95a](https://github.com/bonboncinnabon/sentei/commit/d5ba95ac31769dccbeaec70ea73387e6c35f117b))
* **tui:** check a create --branch launch for an existing worktree ([f445ef4](https://github.com/bonboncinnabon/sentei/commit/f445ef4da710bcfa11ea81ecc44c73eb8a9a847a))
* **tui:** fit a long value by what it is, and keep commands runnable ([3b3326b](https://github.com/bonboncinnabon/sentei/commit/3b3326b7a06994f8be8f10215f34b8167a2cd879))
* **tui:** name the gone-upstream branches in the cleanup preview ([6b5dbed](https://github.com/bonboncinnabon/sentei/commit/6b5dbedf80635e64b2e0dbd4387d0c72921c9545))
* **tui:** wrap long values on confirmation and summary screens ([b972e82](https://github.com/bonboncinnabon/sentei/commit/b972e8287e4d8557b422790d4936abaa69631951))

## [2.1.0](https://github.com/bonboncinnabon/sentei/compare/v2.0.0...v2.1.0) (2026-10-13)


### Features

* **cleanup:** name the branches deleted and skipped on the command line ([ed077ee](https://github.com/bonboncinnabon/sentei/commit/ed077ee64cd81e6c8c12c3090c32fed704395c56))
* **config:** let an empty install command clear the inherited one ([427ba93](https://github.com/bonboncinnabon/sentei/commit/427ba93a7166c4d54398111154711dd0d59dc74b))
* **migrate:** convert repositories that have linked worktrees ([ecd4d3e](https://github.com/bonboncinnabon/sentei/commit/ecd4d3eb8b4d5508dc3e1a26f10a75cf63418a0d))
* **migrate:** convert repositories that have submodules ([88ce950](https://github.com/bonboncinnabon/sentei/commit/88ce95031291104dcb6b350b43027438a8caf284))
* **migrate:** convert sparse checkouts and per-worktree config ([8d73475](https://github.com/bonboncinnabon/sentei/commit/8d734757495e0b5a2bd19ff4a858538e4d4509d0))
* **migrate:** make relative remote paths absolute ([c1aab1a](https://github.com/bonboncinnabon/sentei/commit/c1aab1a7a41dd1e549b2c1ce224dcc2fb56141e5))
* plain bare layouts, migrate conversions and the remaining audit items ([52b69dc](https://github.com/bonboncinnabon/sentei/commit/52b69dcb7c44f2937f1154a9c0a8bd25c2b50ecc))
* **tui:** list what else a migration changes before it is confirmed ([cdb2456](https://github.com/bonboncinnabon/sentei/commit/cdb245675fba8963059f6f9e387a6e48862c61c9))


### Bug Fixes

* **cli:** print migrate steps by their labels ([6401ec2](https://github.com/bonboncinnabon/sentei/commit/6401ec2fb76c0442d1c6ffb1345a58c612239575))
* **clone:** print step labels instead of step ids ([9d07794](https://github.com/bonboncinnabon/sentei/commit/9d07794b7ef881b8804036881c26064a8347ee71))
* **git:** drop an inherited GIT_DIR for gh and the relaunched sentei ([840e53b](https://github.com/bonboncinnabon/sentei/commit/840e53b7c3e481360da2446627a628b46a9d10f3))
* **git:** find the default branch when bare repositories must be named ([0f15ee3](https://github.com/bonboncinnabon/sentei/commit/0f15ee3dafcbc2d8d8be88963b7a0132365b61c5))
* **git:** resolve one repository root for config and worktree placement ([ab55bee](https://github.com/bonboncinnabon/sentei/commit/ab55bee5a7d4606826bd1031d72e46f3fb74acf1))
* **git:** return an absolute common dir for a relative path ([e81f44d](https://github.com/bonboncinnabon/sentei/commit/e81f44dc715b898faad58ee77c64b97fdb44f5d9))
* **git:** run ssh in batch mode so it cannot prompt under the TUI ([b8bf8ef](https://github.com/bonboncinnabon/sentei/commit/b8bf8efe9820b458de3ff4114f7e83b5ac2c19a5))
* **git:** take a parent for the root only when its pointer names the common dir ([6122508](https://github.com/bonboncinnabon/sentei/commit/61225089437fd7d4c357775beaeef82c6fc857fc))
* **migrate:** keep a linked worktree under a root spelled in another case ([5c55905](https://github.com/bonboncinnabon/sentei/commit/5c559056d9ffc5c6e1cc5ab12077faff21055623))
* **migrate:** read extensions.worktreeConfig from the repository only ([ad60ada](https://github.com/bonboncinnabon/sentei/commit/ad60adaf97d940f9b7ac8503a2a543478ca8bca8))
* **migrate:** refuse a submodule that has linked worktrees of its own ([de0ee19](https://github.com/bonboncinnabon/sentei/commit/de0ee198a3f48cd19334c8b03f954180fd8a576b))
* **migrate:** rewrite a relative remote that leaves out the .git suffix ([a5a37a9](https://github.com/bonboncinnabon/sentei/commit/a5a37a9d569504297a8cc1b46d56dce4fb46989d))
* **migrate:** undo only the worktree links that were rewritten ([ad0ab8d](https://github.com/bonboncinnabon/sentei/commit/ad0ab8d2edf08f68ab5c6595d791017a22ff06f4))
* **remove:** make --merged right for fast-forwards and unused branches ([016e256](https://github.com/bonboncinnabon/sentei/commit/016e256c4630bde5900eb66d43a4275ebe08dee1))
* **repo:** accept any worktree of a bare repository as an entry point ([52898eb](https://github.com/bonboncinnabon/sentei/commit/52898eb0c571ec6b0e403207fcd50e2e32f6e29a))
* **repo:** name the shared directory to git when asking whether it is bare ([278842a](https://github.com/bonboncinnabon/sentei/commit/278842a46f20c306f827d2c77c4d69e9ebb4d310))
* **tui:** focus the URL field when `sentei clone` opens the form ([42a76d9](https://github.com/bonboncinnabon/sentei/commit/42a76d9d6b7da7895901b81f239d06abe195360c))
* **tui:** indent every row of the removal summary's failure block ([8d2f7b2](https://github.com/bonboncinnabon/sentei/commit/8d2f7b2fe608762e0c66d2076bfb0466db88b291))
* **tui:** keep the command echo on a direct migrate launch ([e98668e](https://github.com/bonboncinnabon/sentei/commit/e98668e57cb27907ca2a64ef9f4ca593cb97f3c0))
* **tui:** keep the open help in step with the view behind it ([c84eec6](https://github.com/bonboncinnabon/sentei/commit/c84eec630e75296a15d2376a9064dcc4c2637d8e))
* **tui:** keep the origin of a long branch on the create screens ([97c9bd3](https://github.com/bonboncinnabon/sentei/commit/97c9bd376a2430a91e9de7d81a39b3c6fae7c7c8))
* **tui:** make the cleanup screens say what happened and what enter does ([8bb1e91](https://github.com/bonboncinnabon/sentei/commit/8bb1e916615c4e74e0471dac6af3be01374a3a3d))
* **tui:** offer no install row for an ecosystem without a command ([e0db9e3](https://github.com/bonboncinnabon/sentei/commit/e0db9e38e46bd98f14ee441289979fcea769ce7b))
* **tui:** show the pre-flight when migrate is launched directly ([1e462ca](https://github.com/bonboncinnabon/sentei/commit/1e462ca874412d8659698df0fa2d7aee456a106f))
* **tui:** stop titling a failed repository run as created or cloned ([366eb56](https://github.com/bonboncinnabon/sentei/commit/366eb5614a8607217790505a0432dd4b2cb07e5c))
* **tui:** tell the truth in the create flow and keep the end of long paths ([0ebaa32](https://github.com/bonboncinnabon/sentei/commit/0ebaa3203e40739d6e6d9b967de2b2f8682d0370))

## [2.0.0](https://github.com/bonboncinnabon/sentei/compare/v1.14.0...v2.0.0) (2026-10-13)


### ⚠ BREAKING CHANGES

* `sentei migrate --delete-backup` is removed. Migrate converts the repository in place and makes no backup.
* `sentei migrate` refuses repositories with submodules, linked worktrees or sparse checkout.
* An unknown config key is an error, and a config file that fails to load stops the run.
* `sentei remove` needs `--force` for a locked worktree.
* Removing worktrees in the TUI no longer runs cleanup afterwards; run `sentei cleanup`.
* `sentei clone` keeps only the default branch as a local branch.
* git never prompts for credentials; use a credential helper or an SSH key.
* the `integrations` command, the integration views, and the integrations_enabled, protected_branches and post_install config keys are gone. Index folders that already exist on disk are not touched.

### Features

* remove the integrations feature ([b9c7bc8](https://github.com/bonboncinnabon/sentei/commit/b9c7bc830c46bb61191c08780fa216bb50561c66))


### Bug Fixes

* cleanup correctness and reporting from the audit backlog ([a2f3db7](https://github.com/bonboncinnabon/sentei/commit/a2f3db7a1b1ecbe9283025cfb3b38f110c7ba112))
* **cleanup:** collapse identical config lines with git itself ([938f7ad](https://github.com/bonboncinnabon/sentei/commit/938f7adaa832004fe2d9072640b8859b2e179184))
* **cleanup:** ignore the detached HEAD entry when listing branches ([16febe9](https://github.com/bonboncinnabon/sentei/commit/16febe9118d0bcb38fd1ed71c5b3e5af246b78da))
* **cleanup:** make the dry run match the run for gone branches ([8d70397](https://github.com/bonboncinnabon/sentei/commit/8d703979c2dcda53eca33281b39a4089537ba336))
* **cleanup:** protect the default branch when given a worktree path ([45983a6](https://github.com/bonboncinnabon/sentei/commit/45983a6c09936a9f02912acde93ab1896a9423f5))
* **cleanup:** run non-interactive cleanup from the repository root ([bd58607](https://github.com/bonboncinnabon/sentei/commit/bd58607493c3ea42ef8c83658ff6543ef5289f3d))
* **cleanup:** show commit details for a branch that shares a tag's name ([721661e](https://github.com/bonboncinnabon/sentei/commit/721661e94749b1ed76a6b112f1e18e9d9ce6f1ad))
* **clone:** check out main from a remote whose HEAD is detached ([6d86ee8](https://github.com/bonboncinnabon/sentei/commit/6d86ee81343ebdfbefdb422337e98cd749a30f82))
* **clone:** keep only the default branch as a local head ([6abfc2b](https://github.com/bonboncinnabon/sentei/commit/6abfc2b69bd04c6d465e2126fcc49cce457f1671))
* close audit findings in removal, cleanup, installs and the CLI ([6cae71f](https://github.com/bonboncinnabon/sentei/commit/6cae71f5b21d2aed571d271b44ae5bdde45cbd43))
* **cmd:** plain output off a terminal, named contexts, ecosystems --help ([f4db193](https://github.com/bonboncinnabon/sentei/commit/f4db19386f5fbf72c8159ac4576c7451669ce73c))
* config errors, default-branch detection and docs after the audit backlog ([9995e4b](https://github.com/bonboncinnabon/sentei/commit/9995e4b816aa61700a923ca136d41ee356129c5f))
* **config:** find the repo config from every path into a repository ([a2ae465](https://github.com/bonboncinnabon/sentei/commit/a2ae465e037b29aa52d1ef0b0ae0fe59115dfa8c))
* **config:** let an explicit empty list clear an inherited value ([015aff8](https://github.com/bonboncinnabon/sentei/commit/015aff8102a74b2dfea33e9c6327dcfb92d11bbb))
* **config:** never resolve the global config against the working directory ([a0a6685](https://github.com/bonboncinnabon/sentei/commit/a0a6685437b0e4baf20afc05e717851c8a2cb840))
* **config:** read a plain bare repository's config from the repository ([aef99c5](https://github.com/bonboncinnabon/sentei/commit/aef99c5571f2e7df2e08a850ba97b1fd92834e9a))
* **config:** reject unknown config keys instead of ignoring them ([0bc8bc4](https://github.com/bonboncinnabon/sentei/commit/0bc8bc402604db55fe2f5a5d36a3f8db95fe7af9))
* create and clone flows from the audit backlog ([c028343](https://github.com/bonboncinnabon/sentei/commit/c0283432670e07f983f606f946245c61ebdda601))
* **create:** check out a branch that only origin has, tracking it ([8f59a39](https://github.com/bonboncinnabon/sentei/commit/8f59a39e698d2bc21c9cf7e2b31c95cd75c6fa46))
* **create:** copy env files without requiring an install selection ([8e5c7ac](https://github.com/bonboncinnabon/sentei/commit/8e5c7aca58ed1a2f03de5e5f0d7b3a40777e9304))
* **create:** install dependencies once at the worktree root ([8dc4811](https://github.com/bonboncinnabon/sentei/commit/8dc4811d803ed10750bedbcc5ac0577f346c0533))
* **create:** never copy env files from the working directory ([7c2bfd9](https://github.com/bonboncinnabon/sentei/commit/7c2bfd9407a5fd56bbb499ecc03ef279e4c021ea))
* **create:** print a header that holds for every branch ([4b58767](https://github.com/bonboncinnabon/sentei/commit/4b587678851fb71d5b4121a8ae4d01f10662db67))
* **create:** report a failed worktree add as failed, keep a hooked one ([201fb82](https://github.com/bonboncinnabon/sentei/commit/201fb829a16723ff26fa3cc565fd2f31a8dd9000))
* **create:** say when an existing branch is reused, block option names ([13685e3](https://github.com/bonboncinnabon/sentei/commit/13685e3126701ea739e88c0c1fbc394e0579d2dd))
* **dryrun:** mark the detected default branch as protected ([bcbcd9f](https://github.com/bonboncinnabon/sentei/commit/bcbcd9fc5935911e16a8fc7d8044ce1d634f62c7))
* **ecosystem:** detect ecosystems in a directory whose path has glob characters ([6845418](https://github.com/bonboncinnabon/sentei/commit/6845418d562e97ba823a25b84b8e2c77782d4d63))
* git runner isolation, timeouts and config loading from the audit backlog ([9838163](https://github.com/bonboncinnabon/sentei/commit/9838163062f6209e025766fcf003d7ea41ace3e7))
* **git:** isolate git child processes and bound them with a timeout ([97d3a88](https://github.com/bonboncinnabon/sentei/commit/97d3a88ba9291ff1cf2671a3a60ae3ddcce9baf4))
* **git:** keep read-only git commands from rewriting the index ([322b6dd](https://github.com/bonboncinnabon/sentei/commit/322b6ddac4d14c186f2b7aa6ac5e1424106e119d))
* **git:** read the default branch from the shared HEAD as a full ref ([1eb6e18](https://github.com/bonboncinnabon/sentei/commit/1eb6e18c4508d58acb33efbb39c8406d1bfa19de))
* **git:** run git with LC_ALL=C so its messages can be matched ([76c89fd](https://github.com/bonboncinnabon/sentei/commit/76c89fd47be8e66cfe76e61ac0a3144743e2bc07))
* **git:** say what to do when git cannot prompt for credentials ([338dbc0](https://github.com/bonboncinnabon/sentei/commit/338dbc088d719d87583e86d949053a5ea9467a80))
* harden removal, cleanup and the non-interactive CLI ([6025a20](https://github.com/bonboncinnabon/sentei/commit/6025a20c223948230895a65b78bb84b9567641d7))
* in-place migrate with rollback, clone and repo creation from the audit backlog ([a10d7b0](https://github.com/bonboncinnabon/sentei/commit/a10d7b09eb09b6a47561e67a7d28fb22f032e885))
* last migrate and clone fixes, and the 2.0 upgrade notes ([055cbcd](https://github.com/bonboncinnabon/sentei/commit/055cbcdb913aea9b1e321ebff8274828df6e5229))
* **main:** remove the playground repo on every exit path ([485e41f](https://github.com/bonboncinnabon/sentei/commit/485e41fd536a67969466a584f921987718d9f59f))
* **migrate:** carry the checkout's own git state into the worktree ([cbb2b7f](https://github.com/bonboncinnabon/sentei/commit/cbb2b7fc778b6f23c2c2115f72987116c45ed7ec))
* **migrate:** convert the repository in place and roll back on failure ([0f17ae9](https://github.com/bonboncinnabon/sentei/commit/0f17ae9e014e541b7e737896529083c552c17bc5))
* **migrate:** do not clean a worktree path git refused to register ([57d1a8c](https://github.com/bonboncinnabon/sentei/commit/57d1a8cf09b66bd5e42e9ffe675e6f07f3a8cf84))
* **migrate:** do not overwrite a path that reappeared during rollback ([4f97c7d](https://github.com/bonboncinnabon/sentei/commit/4f97c7d60f636d8860a21d1ed9ede1cebdc3dc3d))
* **migrate:** find the shared index when the path has brackets ([9deeea0](https://github.com/bonboncinnabon/sentei/commit/9deeea081fc2fec2a129e2750d95ca1764ba9624))
* **migrate:** name the repository root when asked to migrate a subdirectory ([370112c](https://github.com/bonboncinnabon/sentei/commit/370112c2487bc9bb1866e05b58d41965adfbae12))
* **migrate:** refuse repository config the conversion would break ([5bf42f7](https://github.com/bonboncinnabon/sentei/commit/5bf42f737c425eb125df27ea74acc825461883c3))
* **migrate:** restore the config exactly and reach .bare through the pointer ([4a466c9](https://github.com/bonboncinnabon/sentei/commit/4a466c947ba68c881367ab49be9042ca200055e0))
* remove, flags and output from the audit backlog ([6c8768f](https://github.com/bonboncinnabon/sentei/commit/6c8768f24d4875afa263d9f871bab998348f239f))
* **remove:** reject --stale durations that overflow ([90cfdc7](https://github.com/bonboncinnabon/sentei/commit/90cfdc78b8b266733f38efc5b0adf944d3597b3c))
* **remove:** require --force to remove a locked worktree ([09a83ff](https://github.com/bonboncinnabon/sentei/commit/09a83ffcafca3f987d14ecb5dabda03fc60a5866))
* **remove:** say in the dry-run preview what the real run refuses ([c310435](https://github.com/bonboncinnabon/sentei/commit/c3104351624ebca8f3e7fb529d1bd0eff4637cbd))
* **remove:** stop --merged from selecting a brand-new branch ([e72d192](https://github.com/bonboncinnabon/sentei/commit/e72d1923b7cdfdcd7e7068692f905feb0df66c9e))
* **repo:** keep the reason when gh fails without stderr ([e91af65](https://github.com/bonboncinnabon/sentei/commit/e91af6559f8020dbf5402fd1abc67ee860d7e481))
* **repo:** point the bare HEAD at the default branch ([90ae1bb](https://github.com/bonboncinnabon/sentei/commit/90ae1bb5c8e266243536553b7da30c460e49ea3e))
* **repo:** resolve the container when given the .bare directory ([cd3d7a3](https://github.com/bonboncinnabon/sentei/commit/cd3d7a3e5fa74b88b8b6e0985d913a527f012679))
* stop on a config that fails to load instead of running without it ([8910147](https://github.com/bonboncinnabon/sentei/commit/89101477e97976826133b95df66860b09a3af5ca))
* TUI core behaviour and layout from the audit backlog ([e9d9ed6](https://github.com/bonboncinnabon/sentei/commit/e9d9ed637355223b54af8166a51458b10000d03a))
* **tui:** ask before quitting a running operation ([99b917b](https://github.com/bonboncinnabon/sentei/commit/99b917b018b36209ecca9a8817185e89a820101a))
* **tui:** claim a prune on the summary only when one ran ([9991849](https://github.com/bonboncinnabon/sentei/commit/99918495af15c7acb55db261c924457b83c43d42))
* **tui:** clear the shared repo error when a menu entry opens a screen ([f24fe19](https://github.com/bonboncinnabon/sentei/commit/f24fe19cb7ab1c20033a1f0a0606aca5bb01814f))
* **tui:** close the help portal when a progress view ends ([dbe9e40](https://github.com/bonboncinnabon/sentei/commit/dbe9e40bbea4355a34c172f7703504f3c06eea82))
* **tui:** fit the list and confirm views on narrow terminals ([3f6cdb4](https://github.com/bonboncinnabon/sentei/commit/3f6cdb4d41df0c0b2cb9c8bedd6867a14f91f477))
* **tui:** fit the worktree list to the terminal height ([a49b28e](https://github.com/bonboncinnabon/sentei/commit/a49b28e340ff0c2371edd7973422e9b88f736fda))
* **tui:** honor create flags and derive the clone name when interactive ([0c4216c](https://github.com/bonboncinnabon/sentei/commit/0c4216c6aef677e032c66b3e5bba1d059c8c5676))
* **tui:** keep a failed step with a multi-line error on one row ([fa08d75](https://github.com/bonboncinnabon/sentei/commit/fa08d7524d69b27569b7c35530591fe4129fbd5e))
* **tui:** keep ecosystem choices when the branch form detects again ([7557d6f](https://github.com/bonboncinnabon/sentei/commit/7557d6fe22f92cbf6ae644c7bb8f64bc9b78c6e7))
* **tui:** keep form validation errors until the text changes ([6d2d428](https://github.com/bonboncinnabon/sentei/commit/6d2d428c56a1658703799e6abaeb746b2f950883))
* **tui:** keep migrate screens aligned when text wraps ([45176d9](https://github.com/bonboncinnabon/sentei/commit/45176d98294daece73cbb31c36232454f60e450b))
* **tui:** keep the tail of a long destination path on screen ([0459ea4](https://github.com/bonboncinnabon/sentei/commit/0459ea440077d0b1f8fbda41664e6a3ba25f6acd))
* **tui:** leave the merge off for a branch that already exists ([cf892e0](https://github.com/bonboncinnabon/sentei/commit/cf892e0192ec8c0ab1b68e57cf53c68a9bb35c45))
* **tui:** let git re-check rows that looked clean before removing them ([59f5104](https://github.com/bonboncinnabon/sentei/commit/59f5104227c9b3ecbebcbe3209ed447b5f774cb4))
* **tui:** list every failed branch deletion on the cleanup screens ([fcad104](https://github.com/bonboncinnabon/sentei/commit/fcad10431eaf9d51cb11c6823a5f1ce3429fc40e))
* **tui:** load worktrees when remove opens the list directly ([4c19630](https://github.com/bonboncinnabon/sentei/commit/4c19630f25fbd0b1953787bdb83703ef1b084b4d))
* **tui:** make the cleanup screens report what happened ([a978257](https://github.com/bonboncinnabon/sentei/commit/a9782571623ec1e46d3e6603c0f48632d0f2bc30))
* **tui:** make the create repository form control what runs ([9a73da7](https://github.com/bonboncinnabon/sentei/commit/9a73da70902c213a3f00abb3084d0a4c39d029c3))
* **tui:** offer only detected ecosystems on the create options screen ([6d3b345](https://github.com/bonboncinnabon/sentei/commit/6d3b3451cc3c1d87fc1ea9e8c7c1c3e9873174d0))
* **tui:** prune locally after a removal instead of running cleanup ([ad981f7](https://github.com/bonboncinnabon/sentei/commit/ad981f75a65c764623c8886d522e826f17c8f88c))
* **tui:** quit on ctrl+c from every view ([7776fc1](https://github.com/bonboncinnabon/sentei/commit/7776fc1da7c00b62765cc56834c98a337fd6dc71))
* **tui:** re-read status before removing rows that looked clean ([bf842a5](https://github.com/bonboncinnabon/sentei/commit/bf842a5c8877728e1e74a76308a4d8e7e0540a1b))
* **tui:** remove locked worktrees without unlocking them first ([c9789cf](https://github.com/bonboncinnabon/sentei/commit/c9789cf9011cf02f6ce75844f9e863e3fb4c655b))
* **tui:** report an interrupted flow only while it is running ([6fa596b](https://github.com/bonboncinnabon/sentei/commit/6fa596bea6307902dc2129e503938250c6bb49f0))
* **tui:** report failed worktree loads and drop rows that are gone ([663fcb3](https://github.com/bonboncinnabon/sentei/commit/663fcb3868950a9e3156c7d858dd48e60b3eecd4))
* **tui:** say "1 branch remains" on the cleanup result ([ec40928](https://github.com/bonboncinnabon/sentei/commit/ec40928e60dadc9dc184ed10460100b48750604d))
* **tui:** show the whole reason when the worktree list fails to load ([bef99dc](https://github.com/bonboncinnabon/sentei/commit/bef99dccd85ac523a5404051aac31f72aa94f3e3))
* **tui:** stop a stale motion tick from doubling the clock ([a7e1e72](https://github.com/bonboncinnabon/sentei/commit/a7e1e72a4f5a782489be8fb01c00c75f62287b54))
* **tui:** tick the elapsed stopwatch once a second ([066e607](https://github.com/bonboncinnabon/sentei/commit/066e607b1d07c95fe35c05163e595c315f1767ec))
* **tui:** truncate by display width, not rune count ([8ae4225](https://github.com/bonboncinnabon/sentei/commit/8ae4225bbe7ab0205713fcd852a6ddeded1c3bc6))
* **worktree:** date worktrees by committer date, not author date ([9a8c872](https://github.com/bonboncinnabon/sentei/commit/9a8c872d54d8e9381b040afbf5b9ea5f7607bd6f))
* **worktree:** see untracked files whatever status.showUntrackedFiles says ([ce49f27](https://github.com/bonboncinnabon/sentei/commit/ce49f27a10fc659ee1ee95bf252d3887f9ea84e1))


### Documentation

* **readme:** add upgrade notes for 2.0 ([a98e478](https://github.com/bonboncinnabon/sentei/commit/a98e478bd785b686500d74d9898cc8a96215d607))

## [1.14.0](https://github.com/abiswas97/sentei/compare/v1.13.0...v1.14.0) (2026-07-29)


### Features

* copy repository files into new worktrees ([6740340](https://github.com/abiswas97/sentei/commit/6740340cfe5718910c8ca32cece71e3afa01c9d2))
* copy repository files into new worktrees ([e23afae](https://github.com/abiswas97/sentei/commit/e23afae2bb004d86d288cd506aa38e9aa6b14a21))

## [1.13.0](https://github.com/abiswas97/sentei/compare/v1.12.0...v1.13.0) (2026-07-16)


### Features

* **tui:** support native paste in text inputs ([0179157](https://github.com/abiswas97/sentei/commit/01791571e6770bafef4b4a101bc83fcb29dc2f3d))
* **tui:** support paste in form inputs ([25d55f7](https://github.com/abiswas97/sentei/commit/25d55f70ddcdaae8d4cf71ccf3c278d96b01778f))


### Bug Fixes

* **tui:** isolate paste to visible text inputs ([043e591](https://github.com/abiswas97/sentei/commit/043e5910201008a81a95f65319a5ad4893aafde4))

## [1.12.0](https://github.com/abiswas97/sentei/compare/v1.11.1...v1.12.0) (2026-07-15)


### Features

* **creator,repo:** declare known steps and close phases in pipeline flows ([2fa0778](https://github.com/abiswas97/sentei/commit/2fa07789d3ee032dd1581c0d866c9c33f5f02caa))
* **integration:** declare apply plans upfront, worktree-outer execution ([b2dfdb2](https://github.com/abiswas97/sentei/commit/b2dfdb2c2a387119a438d38d76d94c7948f4fb43))
* progress consolidation arc ([2db9c12](https://github.com/abiswas97/sentei/commit/2db9c12192e4310bf7bb9abb26de38eb77265555))
* **progress:** add consolidated progress package vocabulary, runner, and fold ([e7bf135](https://github.com/abiswas97/sentei/commit/e7bf13558b8c2a6effd3df6f72773aa6a2e07b70))
* **progress:** finalize execution contracts ([abfe5f1](https://github.com/abiswas97/sentei/commit/abfe5f18614054cc894ffcb0b681c6f32ee56890))
* **progress:** make plans own terminal state ([b5b5665](https://github.com/abiswas97/sentei/commit/b5b566520cd24b46102b98104eebf80cc558882e))
* **progress:** plan declarations, checkpoints, close markers, settled predicate ([f7bcfde](https://github.com/abiswas97/sentei/commit/f7bcfdeac82d9ecaec7dc26ddc63940933ba4166))
* **tui:** checkpoint-derived bar and settled-gated phase collapse ([453788d](https://github.com/abiswas97/sentei/commit/453788d0bf24070ed12e0b1129b3f6bb8053fda3))
* **tui:** pin the progress live region ([d8c080c](https://github.com/abiswas97/sentei/commit/d8c080ca91a196ff0358af5ce2d1a460cd12868f))
* **tui:** removal flow declares teardown plans and removal checkpoints ([6e2cf42](https://github.com/abiswas97/sentei/commit/6e2cf42be37edf9db125c76bc3ecd3e0d9c37079))
* **tui:** truth polish for errors title, failed counts, skip traces, elapsed gate ([120c314](https://github.com/abiswas97/sentei/commit/120c31449818b0a6e90305b48883dfb80d4bcda7))
* **tui:** unconditional state-relative completion settle ([548e976](https://github.com/abiswas97/sentei/commit/548e9760da96a322fa66786ca5fb4cb6ad8ce015))


### Bug Fixes

* **creator:** preserve independent setup fallbacks ([9e988a7](https://github.com/abiswas97/sentei/commit/9e988a73976fb23bb6977d8a6703b8cfcfe2e0b9))
* **creator:** settle the prepared creation plan ([7b79972](https://github.com/abiswas97/sentei/commit/7b799725376a49b9525a9069ce285e9ac54b2b8a))
* **demos:** fail closed outside fixture ([552f70f](https://github.com/abiswas97/sentei/commit/552f70f95651330757cb91314d4cefcf00dcd904))
* **hooks:** install hooks in shared git dir ([37a7e55](https://github.com/abiswas97/sentei/commit/37a7e555b019ae028afc85ddfa85213443fe987d))
* **integration:** complete frozen operation engine ([a2f18e4](https://github.com/abiswas97/sentei/commit/a2f18e43e64fae77b1b8d22041b4292919e8a1a4))
* **integration:** contain destructive operations ([ebff4c1](https://github.com/abiswas97/sentei/commit/ebff4c11d85a329006cd2530b5e1a700c8f74554))
* **integration:** freeze exact apply plans ([2f0b476](https://github.com/abiswas97/sentei/commit/2f0b47637839cf26889fedccff25ad7c2feb7df0))
* **integration:** preserve detection fallback ([f38c862](https://github.com/abiswas97/sentei/commit/f38c86251965b44ab945fb64ab3bc56bcc35a8db))
* **integration:** reject incomplete apply results ([d692f8c](https://github.com/abiswas97/sentei/commit/d692f8c197f2d08b4959be2b84c5a99e4b349da3))
* **integration:** surface prepared apply failures ([2dea12c](https://github.com/abiswas97/sentei/commit/2dea12cd19576c9521a40d1adca9e771827fd30f))
* **progress:** flush terminal events before finish ([c23ae8f](https://github.com/abiswas97/sentei/commit/c23ae8f3aa5fb68f9bee2f7a7bbef7848b852c9c))
* **progress:** harden execution concurrency contract ([b7adb31](https://github.com/abiswas97/sentei/commit/b7adb31eb35cf8bcd91281e8aeceb9f3bba646ab))
* **progress:** require stable stream labels ([15a909a](https://github.com/abiswas97/sentei/commit/15a909a20c47aecf424a4170349fb421ed0e546c))
* **removal:** report all frozen execution failures ([c851159](https://github.com/abiswas97/sentei/commit/c85115906b5f587a6f4afd0740787370b3354501))
* **repo:** execute frozen repository plans ([011b7ce](https://github.com/abiswas97/sentei/commit/011b7ce87c59efbc5110f165851691f81f8174b4))
* **repo:** preserve backup on partial restore ([4bc307c](https://github.com/abiswas97/sentei/commit/4bc307c81e5f28c2e088f524f6b332310aef69a9))
* **repo:** preserve migration recovery metadata ([54cff72](https://github.com/abiswas97/sentei/commit/54cff72429b002045b514dd6211bcd82d24b426e))
* **repo:** reserve migration backups safely ([a0bff9d](https://github.com/abiswas97/sentei/commit/a0bff9d86b68e3f45384e20beea4f920c51d1307))
* **tui:** bound progress views to the terminal ([07cac82](https://github.com/abiswas97/sentei/commit/07cac8271c377eb5c8c801e3f56b39f0612d82d4))
* **tui:** bound settled summaries ([17c83ae](https://github.com/abiswas97/sentei/commit/17c83aec1875c4e89cdf4ec3ae79ac7add022e02))
* **tui:** keep removal summaries truthful ([04d5a5f](https://github.com/abiswas97/sentei/commit/04d5a5f9fbe4334aac67e3fedc17e58bebf81399))
* **tui:** preserve narrow progress details ([06c9737](https://github.com/abiswas97/sentei/commit/06c9737a2d1ab8b49611f0de60fe9d03b8f92522))
* **tui:** preserve omitted progress details ([c50160b](https://github.com/abiswas97/sentei/commit/c50160b79436b120f692cbe076ac1b691b5ccc31))
* **tui:** prevent stale progress motion ([75cc8d9](https://github.com/abiswas97/sentei/commit/75cc8d91236afb9a29dae9401a7d7e0533c4949c))
* **tui:** scope progress motion ([e236c0b](https://github.com/abiswas97/sentei/commit/e236c0bfc8faeeea73509b8ab9bdf86a6d7b964d))
* **tui:** settle progress summaries cleanly ([0cd8462](https://github.com/abiswas97/sentei/commit/0cd84621c97ffb2c37c3f2d177cc5fc43c025b18))

## [1.11.1](https://github.com/abiswas97/sentei/compare/v1.11.0...v1.11.1) (2026-06-12)


### Bug Fixes

* **integration:** align ccc install with upstream docs ([3904038](https://github.com/abiswas97/sentei/commit/390403882e42f88e28ca10792ed52a55aa0b9021))
* **integration:** align ccc install with upstream docs ([7183448](https://github.com/abiswas97/sentei/commit/7183448aa0dc1ab52afcc5466475ada03fcf66dc))

## [1.11.0](https://github.com/abiswas97/sentei/compare/v1.10.0...v1.11.0) (2026-06-12)


### Features

* **tui:** completion settles the bar green ([b896614](https://github.com/abiswas97/sentei/commit/b896614e9c5b01fc28d15b68372edc5c725fdc77))
* **tui:** completion settles the bar green ([5defccb](https://github.com/abiswas97/sentei/commit/5defccbfc228393b91965ef0e0d3a96b2c9cda22))
* **tui:** light palette contrast ([deac935](https://github.com/abiswas97/sentei/commit/deac9352d6f7e656994ebb9b75e3ace707271f93))
* **tui:** light palette contrast ([1c87d1c](https://github.com/abiswas97/sentei/commit/1c87d1c9ce44747ef9d3f0962b72018c2b607e40))
* **tui:** milestone whisper ([13ba7a4](https://github.com/abiswas97/sentei/commit/13ba7a4657018c89ab048301ea80844654938164))
* **tui:** milestone whisper ([86abf02](https://github.com/abiswas97/sentei/commit/86abf029613573a4e57a53f49c7e697cb3efff07))
* **tui:** P3 polish batch ([199621a](https://github.com/abiswas97/sentei/commit/199621ad2a329b52b98af7e7b6e70ca11b7c970d))
* **tui:** persistent input fields ([b667f45](https://github.com/abiswas97/sentei/commit/b667f4529d0c45bce5eed72836f233a457c2f542))
* **tui:** persistent input fields ([f71ab5b](https://github.com/abiswas97/sentei/commit/f71ab5b899be92f1108ec3aa0ac54829b47a35f4))
* **tui:** polish batch for the audit p3 list ([e24578c](https://github.com/abiswas97/sentei/commit/e24578cedbc6e9b49151e6c0695f6c157034ab83))
* **tui:** voice registry ([dc86c19](https://github.com/abiswas97/sentei/commit/dc86c19c5d594025ee6d94e874957f01613c668f))
* **tui:** voice registry ([20bb72d](https://github.com/abiswas97/sentei/commit/20bb72dcbe852a4c37c8616bdd87e119c00400fb))


### Bug Fixes

* **tui:** sanitize child-process output in error peeks ([d7b7dc7](https://github.com/abiswas97/sentei/commit/d7b7dc7dd972cb3388b297d153a5024888bda23b))
* **tui:** sanitize child-process output in error peeks ([8d1b0a6](https://github.com/abiswas97/sentei/commit/8d1b0a609e082a53b889f37d506883b82bde30ff))

## [1.10.0](https://github.com/abiswas97/sentei/compare/v1.9.0...v1.10.0) (2026-06-11)


### Features

* **tui:** animate progress with a breathing dot and gradient bar ([371bc9f](https://github.com/abiswas97/sentei/commit/371bc9f7e2c4e860696b621234ee593e2172d14f))
* **tui:** animate progress with a breathing dot and gradient bar ([21a4272](https://github.com/abiswas97/sentei/commit/21a42723883ce567522ddc37abaad244b2f34e01))
* **tui:** columnar confirm rows ([796ff90](https://github.com/abiswas97/sentei/commit/796ff90efc9d06f7e3b20c9500096890936d43fa))
* **tui:** columnar confirm rows ([804e9a6](https://github.com/abiswas97/sentei/commit/804e9a67755656ce6e2ca07c3cb308445c03114a))
* **tui:** name the terminal tab and mirror native progress ([ee2440f](https://github.com/abiswas97/sentei/commit/ee2440f6aea113b218045248cf4f62972a394b49))
* **tui:** name the terminal tab and mirror native progress ([4f62833](https://github.com/abiswas97/sentei/commit/4f62833c01f3998d43faf0a68d78fc298120fc0d))
* **tui:** star-shimmer vocabulary ([28fca1f](https://github.com/abiswas97/sentei/commit/28fca1f15be7dc27b18bceda06d94f61e4c2758f))
* **tui:** star-shimmer vocabulary ([d8fd73b](https://github.com/abiswas97/sentei/commit/d8fd73b7366e79c8c0737cb3df02eef2ac99854d))
* **tui:** weight selection, danger, and input focus ([e54873c](https://github.com/abiswas97/sentei/commit/e54873c619cc62e32ed04bca74f48e11ea0781d4))
* **tui:** weight selection, danger, and input focus ([750bb93](https://github.com/abiswas97/sentei/commit/750bb93b0b7ca16ec63e751d83677427d61ef1a2))


### Bug Fixes

* **integration:** install ccc with the embeddings-local extra ([f372376](https://github.com/abiswas97/sentei/commit/f372376664ef687c06db25293c4ce45ed0168c97))
* **integration:** install ccc with the embeddings-local extra ([4281d80](https://github.com/abiswas97/sentei/commit/4281d80ecf9bcc7d2680e8918c70e5dd2effa972))
* **integration:** pin ccc install resolution to python 3.11 ([c51bbe0](https://github.com/abiswas97/sentei/commit/c51bbe02612526443a5f89633fdaab207cf4a149))
* **integration:** pin ccc install resolution to python 3.11 ([d3f4058](https://github.com/abiswas97/sentei/commit/d3f405829a3b04d6064000b0d86a90c3d66ab385))
* **integration:** presence-based detection and bounded failure output ([fa83ce7](https://github.com/abiswas97/sentei/commit/fa83ce7e15a9c8e2bd23a5d89ff9369cace5d2d2))
* **integration:** presence-based detection and bounded failure output ([076476b](https://github.com/abiswas97/sentei/commit/076476b1da2f038944d5fcf41d721755d4f5c556))
* **tui:** center-aligned breathing dot, verdict checkmarks, settled bar ([527aeb2](https://github.com/abiswas97/sentei/commit/527aeb2a6ee22d4e95fded014deb36910cd9682b))
* **tui:** center-aligned breathing dot, verdict checkmarks, settled bar ([76ad186](https://github.com/abiswas97/sentei/commit/76ad186ef746d878f1fd4032827f996a8811c559))
* **tui:** cleanup preview, prompt, and result state effective counts ([58149bc](https://github.com/abiswas97/sentei/commit/58149bc070d3dc9feba7094a300f7aa9ed80b8d6))
* **tui:** cleanup preview, prompt, and result state effective counts ([35f90d6](https://github.com/abiswas97/sentei/commit/35f90d6c4fde99f47dc2b6e442a8e423b9e52775))
* **tui:** keep list rows on one line at any terminal width ([3eb18ac](https://github.com/abiswas97/sentei/commit/3eb18acd6fba6fc92ef679eddda3234eda3eca96))
* **tui:** keep list rows on one line at any terminal width ([1d743b0](https://github.com/abiswas97/sentei/commit/1d743b06abd7d7c0370c4184150e1cbda83f6a05))
* **tui:** keep the progress bar and its label coherent ([4d89a99](https://github.com/abiswas97/sentei/commit/4d89a9923971b7088c3f6bf0c30aef4b951ab1fd))
* **tui:** keep the progress bar and its label coherent ([fbc6079](https://github.com/abiswas97/sentei/commit/fbc6079210b1293cf0c2f12f9e90810a64eb22fb))
* **tui:** one working spinner and a silky bar ([7f069e2](https://github.com/abiswas97/sentei/commit/7f069e2bc6e09e3c0c248096fad4e26288c6b93b))
* **tui:** one working spinner and a silky bar ([bb8b1d7](https://github.com/abiswas97/sentei/commit/bb8b1d7a79864880a9afcae60b98b0c3f5e77638))
* **tui:** progress flows end truthfully ([bcaa3ad](https://github.com/abiswas97/sentei/commit/bcaa3ad59c50b25758de714936de8f2311ea8d5a))
* **tui:** progress flows end truthfully ([471a9c2](https://github.com/abiswas97/sentei/commit/471a9c25ee8d41341b17dcd50544260e9190ddea))

## [1.9.0](https://github.com/abiswas97/sentei/compare/v1.8.0...v1.9.0) (2026-06-11)


### Features

* **cli:** adopt charm log for CLI errors and warnings ([220a536](https://github.com/abiswas97/sentei/commit/220a536ecf04cd9416537caa04924b9c7c07a65e))
* **cli:** adopt charm log for CLI errors and warnings ([e765fb4](https://github.com/abiswas97/sentei/commit/e765fb4e12f523231939057da643e6e5c2d41428))
* **tui:** animate indeterminate waits with bubbles/spinner ([d71c1a8](https://github.com/abiswas97/sentei/commit/d71c1a848ac53c028914aa0de2da3e026d385cc1))
* **tui:** animate indeterminate waits with bubbles/spinner ([f26e105](https://github.com/abiswas97/sentei/commit/f26e105912a9d6b440e36eb88bb9010dc49cdb81))
* **tui:** animate the overall progress bar and show elapsed time ([55f1938](https://github.com/abiswas97/sentei/commit/55f19388988f25bc023baa69cfe32fdb3045088a))
* **tui:** animate the overall progress bar and show elapsed time ([286421a](https://github.com/abiswas97/sentei/commit/286421aaf20bc801a08e220ddc6854f1aae83f4c))
* **tui:** derive all key hints and help from keys.go via bubbles/help ([d9bcc62](https://github.com/abiswas97/sentei/commit/d9bcc622fc0ce11f8fc0de95ac5a7f4435f05f52))
* **tui:** derive all key hints and help from keys.go via bubbles/help ([6a190d7](https://github.com/abiswas97/sentei/commit/6a190d70baad553384247fb20cbf51521436a7da))

## [1.8.0](https://github.com/abiswas97/sentei/compare/v1.7.0...v1.8.0) (2026-06-11)


### Features

* **tui:** adapt the palette to light and dark terminal backgrounds ([c71b123](https://github.com/abiswas97/sentei/commit/c71b123cc77972f81eb0a3a77e45d5554a5ecfda))
* **tui:** adapt the palette to light and dark terminal backgrounds ([1e7abfc](https://github.com/abiswas97/sentei/commit/1e7abfcacb61d0fbb5319cad436be9c44664ffd6))

## [1.7.0](https://github.com/abiswas97/sentei/compare/v1.6.0...v1.7.0) (2026-06-11)


### Features

* **tui:** scroll integration apply summary via detail portal ([2f34759](https://github.com/abiswas97/sentei/commit/2f34759973181d8a43affa8e8876392ceb39af8f))
* **tui:** scroll integration apply summary via detail portal ([7d66626](https://github.com/abiswas97/sentei/commit/7d6662632b48dbed8d7c2430ef468bddeb03ae56))


### Bug Fixes

* **cleanup:** predict `git branch -d` in the dry-run merged check ([ebac38b](https://github.com/abiswas97/sentei/commit/ebac38b909bcbdc490eadf928e501c5aaa9def13))
* **cleanup:** predict git branch -d in the dry-run merged check ([86ac737](https://github.com/abiswas97/sentei/commit/86ac7373c28372e3af352d51a39cf40c707ab2de))
* **state:** resolve git common dir instead of assuming .bare ([65f36c3](https://github.com/abiswas97/sentei/commit/65f36c33a7f9f82c183d8b9fb6ef7d0a732a7105))
* **state:** resolve git common dir instead of assuming .bare ([10f3afb](https://github.com/abiswas97/sentei/commit/10f3afb55916a300147b5f5821882064478ee75c))
* **tui:** pluralize confirm/summary copy and unbullet skipped branches ([d71c65f](https://github.com/abiswas97/sentei/commit/d71c65f270a26d2fdbbcf3d62c1864ffc2c0beb1))
* **tui:** pluralize confirm/summary copy and unbullet skipped branches ([d1764e2](https://github.com/abiswas97/sentei/commit/d1764e20ab83db5bcc799d32bf6802f4071b91ce))
* **worktree:** detect unpushed commits against all remotes ([c5fa3f5](https://github.com/abiswas97/sentei/commit/c5fa3f55d7290406bd1a39cd688e0a627a0e22db))
* **worktree:** detect unpushed commits against all remotes ([4070f20](https://github.com/abiswas97/sentei/commit/4070f209706dc9ac4252b0008864fccdfd1d39e4))

## [1.6.0](https://github.com/abiswas97/sentei/compare/v1.5.0...v1.6.0) (2026-06-10)


### Features

* **cleanup:** preview-first cleanup flow, removal safety gate, and --yes flag ([6172e3a](https://github.com/abiswas97/sentei/commit/6172e3a03a7a1de0251d1abd2463000323c7e29b))
* **cleanup:** preview-first cleanup flow, removal safety gate, and --yes flag ([a2ef467](https://github.com/abiswas97/sentei/commit/a2ef46767b18016e87c5ffb3b62477152fb2501f))

## [1.5.0](https://github.com/abiswas97/sentei/compare/v1.4.0...v1.5.0) (2026-06-10)


### Features

* **tui:** flow-state correctness + unified view chrome ([a28aadb](https://github.com/abiswas97/sentei/commit/a28aadb5d232462f739f36fa38d403e89b9e83d8))

## [1.4.0](https://github.com/abiswas97/sentei/compare/v1.3.0...v1.4.0) (2026-06-10)


### Features

* activate playground min-progress-duration (1.5s hold) ([e3ee831](https://github.com/abiswas97/sentei/commit/e3ee83104499d798affa4ba8a6901a376ad077ca))
* add --dry-run support to remove command ([0a140c8](https://github.com/abiswas97/sentei/commit/0a140c860fa8bebd20153a04bdffd8aa3c660aaa))
* add cleanup confirmation view and TUI entry for decision commands ([5a52383](https://github.com/abiswas97/sentei/commit/5a5238334d91be413c485f421305982be8671986))
* add clone command with --non-interactive support ([7c54ab0](https://github.com/abiswas97/sentei/commit/7c54ab05849be3cbfc67e2a0eab5d700437967fa))
* add command registry for CLI/TUI handoff ([638e6d2](https://github.com/abiswas97/sentei/commit/638e6d2854e04641438acb3481c6a5787f528db5))
* add create worktree command with --non-interactive support ([1fb2556](https://github.com/abiswas97/sentei/commit/1fb25563ca6ed457b4005ffec79b80295fbd7eb0))
* add holdOrAdvance helper and progressHoldExpiredMsg for min-duration hold ([a578e83](https://github.com/abiswas97/sentei/commit/a578e83083fd323391bddff0b6bb3a5d04da1a87))
* add migrate command with --non-interactive support ([968134c](https://github.com/abiswas97/sentei/commit/968134c1bb922e8aed22e9d6789359525bc78476))
* add ModelOption + WithMinProgressDuration to tui.Model ([8fa5bf2](https://github.com/abiswas97/sentei/commit/8fa5bf292ea9bdc86e2cf82f921756351a0b6b61))
* add progress tracker package, fix integration progress to use upfront total ([bfd8130](https://github.com/abiswas97/sentei/commit/bfd8130ee75b4373597edce30e6bd04c7e19f50e))
* add remove command with filter flags and --non-interactive ([efa4108](https://github.com/abiswas97/sentei/commit/efa41087a697d34024ca4d06cdbbe453d1356af5))
* add remove filter pre-selection and TUI tests ([4e4e2d0](https://github.com/abiswas97/sentei/commit/4e4e2d002e23a4fa5a045ac85908882766b1e9fb))
* add reusable confirmation view component ([3008561](https://github.com/abiswas97/sentei/commit/30085616fa10889d65b976a790373217c39942ef))
* add teatest infrastructure for TUI E2E testing ([ad31b0e](https://github.com/abiswas97/sentei/commit/ad31b0e8d13346b034b426ca6c1af61369ccf252))
* add TUI confirmation views for create, clone, and migrate ([a37091f](https://github.com/abiswas97/sentei/commit/a37091ff8cb542c8f7c9139576bd1250982a7965))
* add UnlockWorktree for safe locked worktree deletion ([4c1b048](https://github.com/abiswas97/sentei/commit/4c1b048fa56eb08e0b88912f6c596a51bcb3c577))
* CLI/TUI handoff with command registry and --non-interactive support ([06be0b8](https://github.com/abiswas97/sentei/commit/06be0b8c61fa132fa1cc8ea0f498305cbde45801))
* **creator:** reuse an existing branch when creating a worktree ([ef1ec24](https://github.com/abiswas97/sentei/commit/ef1ec24d1d5cc60a79bd3164d7ece6712dafd7ca))
* record progress view start time at all entry points ([665449f](https://github.com/abiswas97/sentei/commit/665449fe982d417b17f39b7e34883827f6aba7bd))
* show percentage instead of step counts in all progress views ([91445ee](https://github.com/abiswas97/sentei/commit/91445ee9e17b9489a606955623671c6b21e024e9))
* UI alignment - locked deletion, progress hold, eager reload ([779aa0d](https://github.com/abiswas97/sentei/commit/779aa0d045f0e0aa90745489a7e653b3e8873eca))
* use holdOrAdvance in all progress completion handlers ([686d99f](https://github.com/abiswas97/sentei/commit/686d99ff58f5f560e4b165b7236a10011d8ce8a1))
* wire cleanup command into registry with --non-interactive support ([d178e8d](https://github.com/abiswas97/sentei/commit/d178e8dc357426f559d01998b5237a43c8855a68))


### Bug Fixes

* add Ctrl+C/q handler to all progress views, refresh worktree list ([72f36ce](https://github.com/abiswas97/sentei/commit/72f36cefa574f7a17c1fc70e383e74f506550119))
* address all remaining review suggestions ([3012140](https://github.com/abiswas97/sentei/commit/30121407259355241559bb7844fe0e2c26d98d99))
* address all review findings from final review round ([a9048c7](https://github.com/abiswas97/sentei/commit/a9048c78d52acfe5e802a88e09949a782fccb330))
* address code review findings from codex/gemini ([e4e40ec](https://github.com/abiswas97/sentei/commit/e4e40ec79a4d3a49dfb924901d8aea032136c780))
* address codex final review — remove architectural inversion and dead code ([4ee7922](https://github.com/abiswas97/sentei/commit/4ee79228c38394e25f218d9de1bb73f4473d8b30))
* address verification findings — routing, dead code, determinism ([f84c7a3](https://github.com/abiswas97/sentei/commit/f84c7a3f41833c38cc112fa52f4e975e73cda870))
* cleanup now prunes stale worktrees, TUI filters prunable entries ([d578757](https://github.com/abiswas97/sentei/commit/d578757158c7f16535f40cde045672a85c19fe74))
* **cleanup:** force-delete unmerged gone branches only in aggressive mode ([911cba9](https://github.com/abiswas97/sentei/commit/911cba9ad3604b018c6a64699a023a844dabe3aa))
* **cleanup:** make --force reach cleanup; warn on dirty worktrees in remove dry-run ([eae2d84](https://github.com/abiswas97/sentei/commit/eae2d848ffd6b05ed8b6a3f5b169e9cb9774d649))
* **cleanup:** prepend re-injected --force so it survives a positional repo path ([8bf4b7c](https://github.com/abiswas97/sentei/commit/8bf4b7c375fd33891264331f8e85691814218ac6))
* **cli:** preserve git error cause; --help exits 0 ([9881b9d](https://github.com/abiswas97/sentei/commit/9881b9d38df9b0adfa83a8a970de67ccc700ae62))
* **clone:** create the worktree before tracking, make fetch best-effort ([bceb356](https://github.com/abiswas97/sentei/commit/bceb3563f69af7bcdfebeda440d94a4c62b2f760))
* **clone:** surface StepSkipped so best-effort tracking-skip isn't a silent gap ([a970eb8](https://github.com/abiswas97/sentei/commit/a970eb8e7d8a658ec83e6215d049fd8783512c42))
* **clone:** validate target + roll back partial clones; fix flaky e2e ([e39d4c4](https://github.com/abiswas97/sentei/commit/e39d4c4c42fa7c8a9ef36ee65f20d8ee347c83b9))
* **create:** publish with gh's configured protocol; flag orphaned remote on push fail ([41f995f](https://github.com/abiswas97/sentei/commit/41f995f9a9e7a4383fb5dadeb16a629350825aa4))
* **creator:** quote interpolated worktree path; record gitignore failures ([4df3e1d](https://github.com/abiswas97/sentei/commit/4df3e1d339a95c8413f64ef6bc0038bccd7fec51))
* detect non-standard default branch on bare clone ([a46729d](https://github.com/abiswas97/sentei/commit/a46729d4d36f462a360614166217b44a9a534c55))
* eliminate all time.Sleep from tests, enforce condition-based waiting ([717efe0](https://github.com/abiswas97/sentei/commit/717efe04b78c7e29e5bfe38e95a8cb8ffda9fb5d))
* emit StatusSkipped for steps not run, fixing progress bar total ([ee8295d](https://github.com/abiswas97/sentei/commit/ee8295d815dc603cdd7e5c89cc82b95c68e246f4))
* fetch tracking refs after bare clone so upstream can be set ([c8e55d8](https://github.com/abiswas97/sentei/commit/c8e55d859dd95822539a93276e1c8ff213670525))
* harden all flows — surface failures, protect default branch, prevent data loss (33-finding audit) ([d3225dc](https://github.com/abiswas97/sentei/commit/d3225dc13ad6a60b12c12efa6d08ec142ef8973b))
* include locked worktrees in CLI deletion with unlock-first strategy ([175e4a5](https://github.com/abiswas97/sentei/commit/175e4a5993d29f214c69de2da0796978c44c264d))
* integration progress bar counts unique steps, not raw events ([0f37618](https://github.com/abiswas97/sentei/commit/0f37618ceed7b87c805e361d1a5061aa050921a7))
* isolate playground tests with per-test temp dirs ([b301fe5](https://github.com/abiswas97/sentei/commit/b301fe5b21b772004630dc66a15f92ac8fdc7d31))
* **migrate:** backup-failure no longer yields a repo-deleting restore command ([6e024ce](https://github.com/abiswas97/sentei/commit/6e024ce2acbca81cc7260ad018db53ebd0ad8273))
* **migrate:** prevent data loss from detached HEAD, slash branches, severed origin ([f9aeb7e](https://github.com/abiswas97/sentei/commit/f9aeb7e8c82151ad38e9f5ca1ab9a186285f741e))
* **playground:** retry seed cleanup to kill the macOS RemoveAll flake ([ad5043a](https://github.com/abiswas97/sentei/commit/ad5043ad1fe27f232bcee66f0358a4bfa8877cae))
* **protection:** protect the actual default branch, not a hardcoded set ([1038a29](https://github.com/abiswas97/sentei/commit/1038a29c0a72f5c7841dc3a2c49103fb263e88fa))
* refresh worktree list when returning to menu from deletion summary ([f883965](https://github.com/abiswas97/sentei/commit/f883965cb45a5baef73705dfd9b4d15f07b32261))
* **remove:** normalize to bare root so default branch is protected from a worktree ([1d890c2](https://github.com/abiswas97/sentei/commit/1d890c24eef9da2a6d797c8be7902855c97cd1d3))
* replace lazy stateStale reload with eager worktree context refresh ([dd8293c](https://github.com/abiswas97/sentei/commit/dd8293ca307fc96a6e9d2086a2c65834159a2efe))
* **repo:** retry RemoveAll on freshly-cloned git dirs (clone rollback, migrate) ([1f8c6e6](https://github.com/abiswas97/sentei/commit/1f8c6e6eff20e2238a066b3fedc573ad1ad727c4))
* resolveBareRoot returns correct path for plain bare repos ([34a21c0](https://github.com/abiswas97/sentei/commit/34a21c0e6865c30d6e33f41401db64776848dbb0))
* restore --force flag and repo path for cleanup, fix magic constant ([5c55522](https://github.com/abiswas97/sentei/commit/5c5552221690197a53553ff9f586ac6845c23502))
* **security:** quote the missed {path} injection site; hoist shellQuote to one home ([0710353](https://github.com/abiswas97/sentei/commit/0710353b757cd4b29898d00371dea48fbef64a66))
* set git user identity in confirm test for CI ([5abf438](https://github.com/abiswas97/sentei/commit/5abf438bdab08c59ae33c92bafe338ed1ddfa8c2))
* set git user identity in unlock worktree tests for CI ([6e187ca](https://github.com/abiswas97/sentei/commit/6e187ca089d4848df326ec84d541e6a3e85155ca))
* skip remote ref pruning when origin remote doesn't exist ([d6cc1be](https://github.com/abiswas97/sentei/commit/d6cc1be3daac7d1956ba1ff6dac9788273df6453))
* **state:** no crash on corrupt config, no false-success on failed save, durable writes ([d2a82f4](https://github.com/abiswas97/sentei/commit/d2a82f4582e0d0f58b6c64ba361e43585f50e123))
* **tui:** surface clone/create/migrate failures instead of false success ([c4a3295](https://github.com/abiswas97/sentei/commit/c4a329555e7b697f110dfab05e75165337478863))
* unlock locked worktrees before deletion in TUI ([6b489f1](https://github.com/abiswas97/sentei/commit/6b489f11a90c0883531b9fe8d453b1e40fd43847))
* use --initial-branch=main in all test bare repo setups ([9e0e2e0](https://github.com/abiswas97/sentei/commit/9e0e2e0a28e164d9094b1dd54281b035cc9ec074))

## [1.3.0](https://github.com/abiswas97/sentei/compare/v1.2.0...v1.3.0) (2026-03-30)


### Features

* add ccc copy optimization for cross-worktree index reuse ([6777dbc](https://github.com/abiswas97/sentei/commit/6777dbcc1f2432a9599366ecabda96f047b7cbae))
* add cleanup results screen with detailed metrics ([9525ee3](https://github.com/abiswas97/sentei/commit/9525ee3cc735a20fa89ff5145f5c9c164caf8dc5))
* add IndexCopyDir field for config-driven index copy optimization ([7f00df7](https://github.com/abiswas97/sentei/commit/7f00df7043278d6ee7c6a99bfd1ee47a3300889b))
* add integration manager for enable/disable across worktrees ([a6a27af](https://github.com/abiswas97/sentei/commit/a6a27af856318524d2b3e4f655fa8490e6594bc8))
* add integration selection during migration onboarding ([df0c4cc](https://github.com/abiswas97/sentei/commit/df0c4cce2b9fbfa5a4995b69412417786a53ac23))
* add Manage integrations menu item with state loading ([1929389](https://github.com/abiswas97/sentei/commit/192938925bb3f470076a701f6aebaff2c8388ade))
* add navigation keys and staged styles for integration views ([ba9ef16](https://github.com/abiswas97/sentei/commit/ba9ef1666d53054936f5079fdcd1f6a61b92de53))
* add on-disk integration artifact detection ([80acf6b](https://github.com/abiswas97/sentei/commit/80acf6bf69bc0089eaf30a289e3206241f8241fc))
* add state persistence for bare repo integration config ([605391c](https://github.com/abiswas97/sentei/commit/605391c250e5638b8de2c9cf77d8a9ed868ffa24))
* **cli:** add 'sentei ecosystems' and 'sentei integrations' subcommands ([a34fc28](https://github.com/abiswas97/sentei/commit/a34fc285f36cb306a4c7afeecc083b571e6bfd9d))
* **config:** add Config types with YAML unmarshaling and tests ([0396b91](https://github.com/abiswas97/sentei/commit/0396b91d39d95b1da18cfd5c0c7c3a4a96d9e211))
* **config:** add embedded default ecosystem definitions (16 ecosystems) ([7261471](https://github.com/abiswas97/sentei/commit/7261471ed5bd8ec2ba91935406c715066e2e484d))
* **config:** implement three-layer config loading with merge and validation ([39ccb72](https://github.com/abiswas97/sentei/commit/39ccb72540a2fc45c6d0942e886273732567bea1))
* **creator:** add ecosystem dependency installation with parallel workspace support ([43d572b](https://github.com/abiswas97/sentei/commit/43d572b8c111d87628ee5c71495f171c1e6da85f))
* **creator:** add integration dependency resolution and setup ([a8f4c08](https://github.com/abiswas97/sentei/commit/a8f4c08b42ced9f24a9cb297ea40c8aacec4dbee))
* **creator:** add integration teardown (scan artifacts, cleanup, fallback deletion) ([df26e13](https://github.com/abiswas97/sentei/commit/df26e139a955b09aeb7842eeaf5ea231d5fd7fd9))
* **creator:** add pipeline types and setup phase (create worktree, merge, copy env) ([9eec78b](https://github.com/abiswas97/sentei/commit/9eec78b1f3379e4166cb7d2b2797b7120fffba48))
* **creator:** wire Run() orchestrator with setup, deps, and integrations phases ([b4b8f88](https://github.com/abiswas97/sentei/commit/b4b8f887ff5fd2116d57525babb40578b13dd0aa))
* **ecosystem:** add registry with priority-ordered detection and glob support ([f853c86](https://github.com/abiswas97/sentei/commit/f853c86f754c34257a50df6556867a86fdb52dd2))
* **ecosystem:** add workspace detection for pnpm, npm, go, and cargo ([6105365](https://github.com/abiswas97/sentei/commit/61053657d0b4a16e5ae225e890cbf786230f7134))
* expand sentei to full worktree lifecycle manager ([e5c0882](https://github.com/abiswas97/sentei/commit/e5c0882d5a770f52704bd057e1beab04ff459a69))
* implement integration apply progress view ([f1fd1b7](https://github.com/abiswas97/sentei/commit/f1fd1b7dd97fbbb4f616e1fe4a443e30a09a401d))
* implement integration list view with toggle and info carousel ([324f484](https://github.com/abiswas97/sentei/commit/324f484931f1487dfc9ccc3bcfaac382994a7bc4))
* **integration:** add registry with code-review-graph and cocoindex-code definitions ([23a4fe0](https://github.com/abiswas97/sentei/commit/23a4fe07284018ac4f820d4f0a5a622b6ca81302))
* redesign integration info dialog with visual hierarchy ([9b616e7](https://github.com/abiswas97/sentei/commit/9b616e72b2d6d2c319c35f7661d2e5b6ccb5b2b9))
* replace integration toggles with repo-level state in create flow ([5efac03](https://github.com/abiswas97/sentei/commit/5efac031daf81bcd0efbbbae58528d3608ce5689))
* repo-level integration management with migration onboarding ([fdb0977](https://github.com/abiswas97/sentei/commit/fdb0977845bdffe99de721acf3643c02bd0af1d2))
* **repo:** add clone repo pipeline with URL-to-name derivation and default branch detection ([94a882b](https://github.com/abiswas97/sentei/commit/94a882bdad0f3cdd35977ef96dcd4f8ead3cf94e))
* **repo:** add create repo pipeline with setup and GitHub phases ([291a911](https://github.com/abiswas97/sentei/commit/291a9113afbfb083c97ba1a1eecd25609de86b78))
* **repo:** add migrate repo pipeline with backup, bare conversion, and file copy ([f254563](https://github.com/abiswas97/sentei/commit/f254563e2b0f9e4d01be7d75b9eb688ba5e48a94))
* **repo:** add shared types and context detection (bare/non-bare/no-repo) ([7af785f](https://github.com/abiswas97/sentei/commit/7af785fc72d78f26e050386dd20982f5d91a690b))
* scaffold integration view states and model ([f078af8](https://github.com/abiswas97/sentei/commit/f078af802d733e378dc1b3e05373c81790d765e7))
* **tui:** adapt menu based on repo context (bare/non-bare/no-repo) ([35bb61e](https://github.com/abiswas97/sentei/commit/35bb61ebf7bd37946cb17535a5873c66bd693aad))
* **tui:** add clone URL input view with real-time name derivation ([08b35d6](https://github.com/abiswas97/sentei/commit/08b35d643b7a5bb45600052c95261c164e6924a2))
* **tui:** add create repo name input and options views with GitHub disclosure ([4182e87](https://github.com/abiswas97/sentei/commit/4182e871960c0c0beeef0997ffe214ef27272e21))
* **tui:** add migrate confirmation and summary views with backup cleanup ([3cd12d7](https://github.com/abiswas97/sentei/commit/3cd12d7412471accb3caeb3c25b6c2b2a6b8626a))
* **tui:** add phase indicator and separator styles for new visual language ([90bb534](https://github.com/abiswas97/sentei/commit/90bb534dac305d4c779fd90ee90ae7928d0a5e88))
* **tui:** add shared progress and summary views with sentei re-launch ([247d6e1](https://github.com/abiswas97/sentei/commit/247d6e13575e207920d647ddc3e377d3719e2b47))
* **tui:** enhance confirm view with integration teardown info and execution ([a44faf8](https://github.com/abiswas97/sentei/commit/a44faf8bc19da547228a0ae2aaa6f034e31d2a37))
* **tui:** enhance removal progress with phased teardown/remove/prune reporting ([a4ab530](https://github.com/abiswas97/sentei/commit/a4ab530ff7f0b47f6bd2f9361646c1993bd172be))
* **tui:** implement create branch view with input validation ([5c3e569](https://github.com/abiswas97/sentei/commit/5c3e569e645162f7edcd67df1557c0ef12ad77e9))
* **tui:** implement create options view with ecosystem and integration toggles ([d196056](https://github.com/abiswas97/sentei/commit/d196056c934656660d6eb0fe6b170ea42a9b6ca7))
* **tui:** implement create progress view with phased parallel reporting ([a7c804b](https://github.com/abiswas97/sentei/commit/a7c804b24c205f37952d4455056d52a8d3bfb761))
* **tui:** implement create summary view with success/failure display ([d20647d](https://github.com/abiswas97/sentei/commit/d20647daec77eaed67191d08021570b0d8d0dc06))
* **tui:** implement menu view as new entry point with lazy worktree loading ([8676518](https://github.com/abiswas97/sentei/commit/867651875983b2877d6dcc9ec24f3b3fa088e59c))
* **tui:** split migrate summary into backup decision + what-next screens ([cb5e807](https://github.com/abiswas97/sentei/commit/cb5e80765046c4893b2d84e23cc0c57c862175bc))
* update main.go to start at menu with lazy worktree loading and config ([174bbca](https://github.com/abiswas97/sentei/commit/174bbca4a41ae2b4043a60350eb1f301f801943a))
* wire context detection into main.go and pass to TUI ([6beed65](https://github.com/abiswas97/sentei/commit/6beed656f4ae7a8dc176a940ae9d5f291d3ac986))


### Bug Fixes

* address code review findings (validation, imports, detect output) ([b0c74d9](https://github.com/abiswas97/sentei/commit/b0c74d9e192695ece03482415d6e091d2b7a7754))
* address critical review findings (shell runner, binary detection, validation) ([52fdfb6](https://github.com/abiswas97/sentei/commit/52fdfb6bc20b19d701f556e025c8d0543f9dc41c))
* address final review findings (shell injection, dedup, shared utilities) ([2a75385](https://github.com/abiswas97/sentei/commit/2a75385413b4559d424cbd258f165cf7af80de73))
* address remaining review findings (result propagation, async menu, parallel teardown, registry pattern) ([81611f8](https://github.com/abiswas97/sentei/commit/81611f81bc205d5c010c99e53f9f474bcae0b691))
* address review findings in implementation plan ([45602e5](https://github.com/abiswas97/sentei/commit/45602e5111540c1ea5dbeb99f22d366f7034e7c7))
* allow esc to exit from main menu ([582c0e9](https://github.com/abiswas97/sentei/commit/582c0e90344167c1465bb2dbf31fe8e883bfedd5))
* auto-update location as repo name is typed, use repoPath not CWD ([0261fe8](https://github.com/abiswas97/sentei/commit/0261fe8ee83b15e58936f941e7f5f2d9b315656d))
* clean root directory after migration, add E2E assertion for clean root ([7049f73](https://github.com/abiswas97/sentei/commit/7049f73e10555625028dadd34afd3047641d8912))
* enable worktree toggle, focus description input on cursor navigation ([f31cbaf](https://github.com/abiswas97/sentei/commit/f31cbaf96a5782290c216f76c2384c027bdc095d))
* make info dialog responsive to terminal width with text wrapping ([821354c](https://github.com/abiswas97/sentei/commit/821354c191a161766b1a4e234815a77171cd04db))
* make integration descriptions more descriptive for info dialog ([aaf6051](https://github.com/abiswas97/sentei/commit/aaf6051c398f0fb52e5847bd292abb6abd7d92f4))
* options toggle, description input, gh create args, error wrapping ([f1e14d5](https://github.com/abiswas97/sentei/commit/f1e14d5f0004cc9761776ba44dbfbeff0855c607))
* resolve absolute paths and pre-fill CWD in create repo location ([ccb8426](https://github.com/abiswas97/sentei/commit/ccb8426f138ef34ffc58e43fde68113610614155))
* resolve bare repo root from inside worktrees using --git-common-dir ([769842b](https://github.com/abiswas97/sentei/commit/769842bbc1bd5bfd46c0e9db65386f6637980dc7))
* resolve CI failures — lint QF1012 and race in parallel deps ([5967973](https://github.com/abiswas97/sentei/commit/59679734d8d443ab54dbcfd43b5b41db7f0bd5f6))
* standardize key bindings to hjkl throughout, fix title consistency ([0901eab](https://github.com/abiswas97/sentei/commit/0901eab81fe32e53850bafd24aef73381880a033))
* standardize remove worktrees flow UI to match app-wide style ([242dfe5](https://github.com/abiswas97/sentei/commit/242dfe5151b35e14b06a49f44c04d0f5b2efda81))
* update post-migration text to remove specific integration names ([4e5460a](https://github.com/abiswas97/sentei/commit/4e5460af3806d390f2bd7c8fde310699fe38193b))

## [1.2.0](https://github.com/abiswas97/sentei/compare/v1.1.0...v1.2.0) (2026-03-21)


### Features

* add esc to quit keys ([af6c5f6](https://github.com/abiswas97/sentei/commit/af6c5f6276502514728d9b54d5d06e023fb3b9a7))
* add version and readme ([51ea174](https://github.com/abiswas97/sentei/commit/51ea1740d984c99de78f9a5ba701d614fba228ee))
* auto prune orphans ([9350f13](https://github.com/abiswas97/sentei/commit/9350f134a2e146ba8a3d31494bac8b2a6875b00a))
* branch protection ([00df628](https://github.com/abiswas97/sentei/commit/00df628d8568d4860e74495815b64a7d2e7e6e44))
* cicd pipeline and releases ([e4a7a64](https://github.com/abiswas97/sentei/commit/e4a7a646457617ad9dcd41c013734a407c88255c))
* cicd pipeline and releases ([8d2d8b8](https://github.com/abiswas97/sentei/commit/8d2d8b8b3bef2ad716cd696d992ed87daadc436f))
* clean up ui to be more tabular ([9a3c4a7](https://github.com/abiswas97/sentei/commit/9a3c4a7e2d7b3498de55fe90f70fcf223ecc3aea))
* **cleanup:** add CLI subcommand and TUI integration ([4c1fca0](https://github.com/abiswas97/sentei/commit/4c1fca04b48b889196b9773bdaaf18d579fa767d))
* **cleanup:** add types, fixtures, resolveConfigPath test, and stub orchestrator ([a9debb1](https://github.com/abiswas97/sentei/commit/a9debb161b7c4fce845092477481c60851f4c016))
* **cleanup:** implement branch cleanup (gone-upstream and non-worktree) ([4fc4272](https://github.com/abiswas97/sentei/commit/4fc427209132d2efa4c028a7352d7d5a66b98bc6))
* **cleanup:** implement config dedup with atomic writes ([8db2f74](https://github.com/abiswas97/sentei/commit/8db2f7408a818dee01e8401d3f5672147507655e))
* **cleanup:** implement orphaned config section purge ([a1f8964](https://github.com/abiswas97/sentei/commit/a1f896495485465b5c7304e400217f5ff19febd1))
* **cleanup:** implement remote ref pruning ([26ba91b](https://github.com/abiswas97/sentei/commit/26ba91bcb28f390016d522ead47a0f06162e40ce))
* **cleanup:** wire orchestrator with all 5 pipeline steps ([3e027e9](https://github.com/abiswas97/sentei/commit/3e027e9dc230ce372a4d346ec3ed88db41fbf15d))
* dry run mode ([881bc99](https://github.com/abiswas97/sentei/commit/881bc999ba22f84203f00843284a5be2099b43b3))
* init ([408af98](https://github.com/abiswas97/sentei/commit/408af987b682ae1cd407a1e931d3ad42579bd388))
* interactive test playground ([0362dc7](https://github.com/abiswas97/sentei/commit/0362dc74431155213ae7c377c444bb8b59b4964e))
* PRD ([c4779b1](https://github.com/abiswas97/sentei/commit/c4779b162c8101c5e8c8c022a21f198ce44a635e))
* progress indicator ([a1b78fe](https://github.com/abiswas97/sentei/commit/a1b78fec8df784da824ab63658b7e79a9627dfd7))
* sort and filter ([4008dfa](https://github.com/abiswas97/sentei/commit/4008dfa02f3e2fcb0d4f36b4d783f1327ef71dea))
* status legend ([fe36b75](https://github.com/abiswas97/sentei/commit/fe36b7535b11925bf68595fcb0cee66d90237b08))
* ui setup ([ac02c5c](https://github.com/abiswas97/sentei/commit/ac02c5c55886700a0521e22d862112d21ced94a9))
* worktree enricher ([ba61c50](https://github.com/abiswas97/sentei/commit/ba61c500d1ba52deebf71186b07c6474191612dc))
* worktree parser ([dcf278b](https://github.com/abiswas97/sentei/commit/dcf278b63fe682fa21f24cf1dd38382a2496f740))


### Bug Fixes

* **cleanup:** address code review findings ([3e3c7bd](https://github.com/abiswas97/sentei/commit/3e3c7bd283e34467124610461fd1d7aa09dd104a))
* code review ([2833e16](https://github.com/abiswas97/sentei/commit/2833e168690d23ef854100e823c4e3aca481dcfe))
* merge release workflows so goreleaser triggers on release ([15834a0](https://github.com/abiswas97/sentei/commit/15834a07f22a56bd2451c7f8d1c3f83595db30de))
* merge release workflows so goreleaser triggers on release ([ef9c46f](https://github.com/abiswas97/sentei/commit/ef9c46f4fe9e9797e5e72bcd439fd518212957f7))
* potential guard ([5bfc617](https://github.com/abiswas97/sentei/commit/5bfc6179e3529d872bb9ec49d127f61a92e67a5e))
* remaining staticcheck QF1012 lint errors in summary.go ([640d4b1](https://github.com/abiswas97/sentei/commit/640d4b1d74c20b169a22e33f542409e58c3a3ff6))
* remove component prefix from release-please tags ([22257a7](https://github.com/abiswas97/sentei/commit/22257a74ea9f09ae95add086205e953afa9baba4))
* remove component prefix from release-please tags ([9e1e67a](https://github.com/abiswas97/sentei/commit/9e1e67a8e54559cfb5f75082f752317e7fa977a9))
* replace git worktree remove with os.RemoveAll for reliable cleanup ([be03198](https://github.com/abiswas97/sentei/commit/be031988be2bccd79ca6ee30b0787b6c76c5d612))
* resolve lint errors (errcheck, staticcheck QF1012) ([ae27fd7](https://github.com/abiswas97/sentei/commit/ae27fd7ae1470b8f46063bd0d8e4cf23c0a2ee60))
* ubuntu test failure ([3a0ef94](https://github.com/abiswas97/sentei/commit/3a0ef94e3fda359777454a184e72bf2fe753b3ac))
* use fmt.Fprintf instead of WriteString(fmt.Sprintf) to satisfy staticcheck QF1012 ([daa1e2d](https://github.com/abiswas97/sentei/commit/daa1e2dade5bb3a6d32e9779ba5bd3202f53f9ee))
* use homebrew formula instead of cask and reset version ([9f8fcae](https://github.com/abiswas97/sentei/commit/9f8fcaeb6f45239bb313f01943539d3fd27b4ad8))
* use homebrew formula instead of cask and reset version ([023a717](https://github.com/abiswas97/sentei/commit/023a717de479a879256728b553af29800d132e09))
* use PAT for release-please to trigger CI on PR branches ([50225fd](https://github.com/abiswas97/sentei/commit/50225fdc845e1a7d130edfde48b3102d5d73d72f))

## [1.1.0](https://github.com/abiswas97/sentei/compare/v1.0.1...v1.1.0) (2026-03-21)


### Features

* **cleanup:** add CLI subcommand and TUI integration ([4c1fca0](https://github.com/abiswas97/sentei/commit/4c1fca04b48b889196b9773bdaaf18d579fa767d))
* **cleanup:** add types, fixtures, resolveConfigPath test, and stub orchestrator ([a9debb1](https://github.com/abiswas97/sentei/commit/a9debb161b7c4fce845092477481c60851f4c016))
* **cleanup:** implement branch cleanup (gone-upstream and non-worktree) ([4fc4272](https://github.com/abiswas97/sentei/commit/4fc427209132d2efa4c028a7352d7d5a66b98bc6))
* **cleanup:** implement config dedup with atomic writes ([8db2f74](https://github.com/abiswas97/sentei/commit/8db2f7408a818dee01e8401d3f5672147507655e))
* **cleanup:** implement orphaned config section purge ([a1f8964](https://github.com/abiswas97/sentei/commit/a1f896495485465b5c7304e400217f5ff19febd1))
* **cleanup:** implement remote ref pruning ([26ba91b](https://github.com/abiswas97/sentei/commit/26ba91bcb28f390016d522ead47a0f06162e40ce))
* **cleanup:** wire orchestrator with all 5 pipeline steps ([3e027e9](https://github.com/abiswas97/sentei/commit/3e027e9dc230ce372a4d346ec3ed88db41fbf15d))


### Bug Fixes

* **cleanup:** address code review findings ([3e3c7bd](https://github.com/abiswas97/sentei/commit/3e3c7bd283e34467124610461fd1d7aa09dd104a))
* resolve lint errors (errcheck, staticcheck QF1012) ([ae27fd7](https://github.com/abiswas97/sentei/commit/ae27fd7ae1470b8f46063bd0d8e4cf23c0a2ee60))

## [1.0.1](https://github.com/abiswas97/sentei/compare/v1.0.0...v1.0.1) (2026-03-05)


### Bug Fixes

* replace git worktree remove with os.RemoveAll for reliable cleanup ([be03198](https://github.com/abiswas97/sentei/commit/be031988be2bccd79ca6ee30b0787b6c76c5d612))
* use fmt.Fprintf instead of WriteString(fmt.Sprintf) to satisfy staticcheck QF1012 ([daa1e2d](https://github.com/abiswas97/sentei/commit/daa1e2dade5bb3a6d32e9779ba5bd3202f53f9ee))

## 1.0.0 (2026-02-07)


### Features

* add esc to quit keys ([af6c5f6](https://github.com/abiswas97/sentei/commit/af6c5f6276502514728d9b54d5d06e023fb3b9a7))
* add version and readme ([51ea174](https://github.com/abiswas97/sentei/commit/51ea1740d984c99de78f9a5ba701d614fba228ee))
* auto prune orphans ([9350f13](https://github.com/abiswas97/sentei/commit/9350f134a2e146ba8a3d31494bac8b2a6875b00a))
* branch protection ([00df628](https://github.com/abiswas97/sentei/commit/00df628d8568d4860e74495815b64a7d2e7e6e44))
* cicd pipeline and releases ([e4a7a64](https://github.com/abiswas97/sentei/commit/e4a7a646457617ad9dcd41c013734a407c88255c))
* cicd pipeline and releases ([8d2d8b8](https://github.com/abiswas97/sentei/commit/8d2d8b8b3bef2ad716cd696d992ed87daadc436f))
* clean up ui to be more tabular ([9a3c4a7](https://github.com/abiswas97/sentei/commit/9a3c4a7e2d7b3498de55fe90f70fcf223ecc3aea))
* dry run mode ([881bc99](https://github.com/abiswas97/sentei/commit/881bc999ba22f84203f00843284a5be2099b43b3))
* init ([408af98](https://github.com/abiswas97/sentei/commit/408af987b682ae1cd407a1e931d3ad42579bd388))
* interactive test playground ([0362dc7](https://github.com/abiswas97/sentei/commit/0362dc74431155213ae7c377c444bb8b59b4964e))
* PRD ([c4779b1](https://github.com/abiswas97/sentei/commit/c4779b162c8101c5e8c8c022a21f198ce44a635e))
* progress indicator ([a1b78fe](https://github.com/abiswas97/sentei/commit/a1b78fec8df784da824ab63658b7e79a9627dfd7))
* sort and filter ([4008dfa](https://github.com/abiswas97/sentei/commit/4008dfa02f3e2fcb0d4f36b4d783f1327ef71dea))
* status legend ([fe36b75](https://github.com/abiswas97/sentei/commit/fe36b7535b11925bf68595fcb0cee66d90237b08))
* ui setup ([ac02c5c](https://github.com/abiswas97/sentei/commit/ac02c5c55886700a0521e22d862112d21ced94a9))
* worktree enricher ([ba61c50](https://github.com/abiswas97/sentei/commit/ba61c500d1ba52deebf71186b07c6474191612dc))
* worktree parser ([dcf278b](https://github.com/abiswas97/sentei/commit/dcf278b63fe682fa21f24cf1dd38382a2496f740))


### Bug Fixes

* code review ([2833e16](https://github.com/abiswas97/sentei/commit/2833e168690d23ef854100e823c4e3aca481dcfe))
* merge release workflows so goreleaser triggers on release ([15834a0](https://github.com/abiswas97/sentei/commit/15834a07f22a56bd2451c7f8d1c3f83595db30de))
* merge release workflows so goreleaser triggers on release ([ef9c46f](https://github.com/abiswas97/sentei/commit/ef9c46f4fe9e9797e5e72bcd439fd518212957f7))
* potential guard ([5bfc617](https://github.com/abiswas97/sentei/commit/5bfc6179e3529d872bb9ec49d127f61a92e67a5e))
* remove component prefix from release-please tags ([22257a7](https://github.com/abiswas97/sentei/commit/22257a74ea9f09ae95add086205e953afa9baba4))
* remove component prefix from release-please tags ([9e1e67a](https://github.com/abiswas97/sentei/commit/9e1e67a8e54559cfb5f75082f752317e7fa977a9))
* ubuntu test failure ([3a0ef94](https://github.com/abiswas97/sentei/commit/3a0ef94e3fda359777454a184e72bf2fe753b3ac))
* use homebrew formula instead of cask and reset version ([9f8fcae](https://github.com/abiswas97/sentei/commit/9f8fcaeb6f45239bb313f01943539d3fd27b4ad8))
* use homebrew formula instead of cask and reset version ([023a717](https://github.com/abiswas97/sentei/commit/023a717de479a879256728b553af29800d132e09))

## Changelog

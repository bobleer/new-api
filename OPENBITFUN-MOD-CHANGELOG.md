# OpenBitFun Mod — Changelog & 升级交接文档

> 本文件记录 OpenBitFun 在上游 new-api（QuantumNous）之上叠加的定制改动，供**下一次版本升级适配时**的 AI Agent / 开发者阅读。请配合根目录 `AGENTS.md` / `CLAUDE.md` 一起使用。

## 当前状态

| 项 | 值 |
|---|---|
| 当前 mod 分支 | `v1.0.0-rc.15-openbitfun-mod` |
| 远端（fork） | `origin` = `https://github.com/bobleer/new-api`（已推送并跟踪） |
| 上游 | `upstream` = `https://github.com/QuantumNous/new-api.git` |
| 基线 tag | `v1.0.0-rc.15`（commit `69b0f0b56f528efa292a2893feb0c55c37399f4b`） |
| 上一版 mod 分支 | `v1.0.0-rc.14-openbitfun-mod`（基线 rc.14，commit `5e866446497652bf8581bb7833fae565beb39fd6`，仍保留在本地与 origin） |

## Mod 概述

本 mod 只围绕**一个核心功能** + 两处小改动：

1. **令牌粒度 `X-Verification-Code` 请求头校验**（核心功能，全栈）
2. `.gitignore` 加入 `.bitfun/`
3. `fix(token)`：模糊化 403 错误信息，避免泄露自定义头名

### 核心功能：令牌粒度 X-Verification-Code 请求头校验

每个 API Key（token）可开启一个开关；开启后，凡使用该令牌的请求必须携带 HTTP 头 `X-Verification-Code: from_bitfun`，否则中间件返回 `403`。本质是“请求是否来自 BitFun 客户端”的软关卡（非真密钥，见“已知问题”）。

**设计要点（务必在下次升级时复核仍成立）：**

- `VerificationCodeEnabled` 字段标注 `gorm:"-"`，**不写入数据库、不改表结构**，规避跨库（SQLite/MySQL/PG）迁移。改为把“哪些 token 开启了校验”作为一个 JSON map（`token_id -> true`）存到现有 `options` 表，key = `TokenVerificationCodeMap`。
- 启动时 `main.go` 调 `model.LoadTokenVerificationSettings()` 把 options 表的 map 载入内存（`map[int]bool` + `sync.RWMutex`）。
- 中间件 `TokenAuth()` 在 IP 限制检查之后、用户状态检查之前校验该头。

## 提交清单

rebase 到 rc.15 后的 3 个提交（新→旧）：

| rebased hash（rc.15 分支） | 原 hash（rc.14 分支） | 说明 |
|---|---|---|
| `657a15c19fb7ae38e10df4a80558f327043423f8` | `639726c61ce770e54c44d9caa1ac6574165fc12e` | fix(token): obscure verification error message to avoid leaking header name |
| `9b67dca16cbd2ea01654395514e399205a47e3d7` | `e92e1562cffe3e54571aa775788ea993559fc2c9` | chore: add .bitfun/ to gitignore |
| `a60e78e7bf9c9f3d1adffd5b383b99de61934f05` | `05f2b5eccd01d3982c254ee0e777c808c585663c` | feat(token):（标题为空，见“已知问题”） |

> 下次升级识别“自定义提交”时，基准是**上一个基线 tag**：`git rev-list --count "<prevTag>^{commit}..<modBranch>"` 应为 3。

## 涉及文件

后端：
- `model/token_verification.go`（新增，持久化层）
- `model/token.go`（加字段 + 各读取/删除路径回填与清理）
- `controller/token.go`（`AddToken`/`UpdateToken` 读写开关）
- `middleware/auth.go`（`TokenAuth()` 校验头 + 错误信息脱敏）
- `main.go`（启动加载设置）

前端（两套主题都覆盖）：
- `web/default/src/features/keys/components/api-keys-mutate-drawer.tsx`
- `web/default/src/features/keys/lib/api-key-form.ts`
- `web/default/src/features/keys/types.ts`
- `web/classic/src/components/table/tokens/modals/EditTokenModal.jsx`
- i18n：`web/default/src/i18n/locales/{en,zh}.json`、`web/classic/src/i18n/locales/{en,zh-CN}.json`

其它：`.gitignore`

## 升级流程（rc.15 → 下一版，已验证可用）

1. `git fetch upstream --tags --prune`
2. 确认新旧 tag 的 commit：`git rev-parse "<newTag>^{commit}" "<oldTag>^{commit}"`
3. 识别自定义提交：
   ```
   git rev-list --count "<oldTag>^{commit}..v1.0.0-rc.15-openbitfun-mod"   # 期望 3
   git log --oneline "<oldTag>^{commit}..v1.0.0-rc.15-openbitfun-mod"
   ```
4. 升级 fork 的 main 到新 tag：若旧 main 是新 tag 的祖先，`git checkout main && git merge --ff-only "<newTag>^{commit}" && git push origin main`（无需 force）。
5. 从新 tag 建分支：`git checkout -b <新mod分支名> "<newTag>^{commit}"`
6. cherry-pick 自定义提交（**最旧优先**）：`git cherry-pick <hash1> <hash2> <hash3>`
7. 按需解决冲突，校验（见下）。

## 坑点 / 经验（务必阅读）

- **带注释 tag（annotated tag）**：`git rev-parse <tag>` 返回的是 tag **对象**哈希，不是 commit。解析 commit 必须用 `"<tag>^{commit}"`；`merge --ff-only` 也建议传 `"<tag>^{commit}"`。
- **Git 工具的 rev-range 不可靠**：在本仓库用 `git log A..B` 时，该工具曾忽略范围直接回放 HEAD 历史，造成误导。**一律改用 ExecCommand** 跑 `git rev-list --count`、`git log --oneline "<A>^{commit}..<B>"`，并用 count 交叉验证。
- **i18n 文件是嵌套结构** `{"translation": { ... }}`，并非 AGENTS.md 所说的 flat。自定义 key 必须落在 `translation` 内部，否则顶层 key 数会异常增多。校验时用 dup-key 感知的 parser（`object_pairs_hook`）。
- **冲突预判**：对 `git diff --name-only "<oldTag>^{commit}..<modBranch>"` 与 `git diff --name-only "<oldTag>^{commit}..<newTag>^{commit}"` 取交集，即可预知 cherry-pick 可能冲突的文件。
- **校验 rebase 保真**：`git range-diff "<oldBase>..<oldTip>" "<newBase>..<newTip>"`；`=` 表示提交一致，`!` 表示有变化（双方都改过的文件预期会 `!`）。
- **非交叠文件字节比对**：对“仅 mod 改动、上游未动”的文件，新旧 mod 分支间对应文件内容应完全一致，可用 `git show A:path` vs `git show B:path` 比对。

## rc.14 → rc.15 适配结论（本次）

- rc.15 **未触碰** `model/token.go`、`model/token_cache.go`、`middleware/auth.go`、`controller/token.go`（各 0 提交）。故 mod 与 rc.15 天然自洽，cherry-pick 零人工冲突，仅 `main.go` 与 4 个 i18n JSON 自动合并。
- rc.15 新增 system-tasks / system-info / email-NTLM / token-limit-section 等，与本 mod 的“令牌粒度请求头校验”**无概念重叠**。
- 后端编译通过（`go build ./model/ ./middleware/ ./controller/ ./common/`）；`go build ./...` 唯一报错是 `//go:embed web/classic/dist` 找不到前端产物，属环境问题，与 mod 无关。

## 校验清单（本次可用，下次复用）

- `go build ./model/ ./middleware/ ./controller/ ./common/`（绕开 embed dist）
- i18n：dup-key 感知 JSON 解析 + 嵌套 key 存在性检查
- `git range-diff` 提交级比对
- 非交叠文件 `git show` 字节比对

## 已知问题 / 下次升级需复核

1. **异步缓存竞态（非 TTL 级，但需关注上游是否改动缓存机制）**：`Token.Update()` 的 defer 通过 `gopool.Go(cacheSetToken)` 异步刷新 Redis 缓存，会写入新的 `VerificationCodeEnabled`。`Update()` 返回到 goroutine 完成之间存在极小竞态窗口，请求可能读到旧值。这与代码库其它 token 字段的异步缓存模式一致，非 mod 特有。**下次升级若上游改了 `token_cache.go` / `GetTokenByKey` / `Token.Update()` 的缓存失效逻辑，需重新评估 `SetTokenVerificationEnabled` 是否要主动失效缓存。**
2. **校验值 `from_bitfun` 硬编码且前端文案明示**：仅作软关卡，不具保密性。若要当真密钥用，需改为可配置且不暴露。
3. **`feat(token):` 提交标题为空**：下次在新分支上可 amend 补全为 `feat(token): add per-token X-Verification-Code header verification`。
4. **若上游新增 token 字段或重构 Token 结构/API**：需确认 `verification_code_enabled` 不与之冲突，且 `gorm:"-"` + options 表的持久化方式仍成立。

## 受保护信息

本 mod 叠加于上游 new-api（QuantumNous）之上。依项目治理规则，上游品牌、署名、元数据等受保护信息不得修改、删除或替换。

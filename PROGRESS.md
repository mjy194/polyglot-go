# Polyglot Migration Progress

## 当前阶段
端到端联调中。两个 Go 仓库已初始提交：
- `new/polyglot` 9e39f44 (119 files)
- `new/uipath_adapter` 7ff8def (24 files)

## 已完成
- ✅ 两仓库初始提交，data/ 目录化，.gitignore 完善
- ✅ dev.sh 集成 adapter 启动（加载 .env 注入 UIPATH_* 凭据）
- ✅ frontend stdin EIO 修复（`< /dev/null`）
- ✅ 端到端链路验证：srv ↔ adapter gRPC 反向注册成功

## 当前阻塞
- ❌ UiPath 账号 abc@uruz.top 被反爬封禁 (429 suspicious login)
  - 根因：srv 5s 一次低水位 → adapter 反复完整 4 阶段登录
  - 单账号反复登录踩到反爬

## 待执行（用户拍板：两个一起做）

### Task 1: StorageService → KV 模式（主框架去 UiPath 化）
**问题**：`SaveAuthStateRequest` 硬编码 email/access_token/refresh_token/upstream_url，违反"框架不管 adapter 私有数据"原则。

**改动**：
1. `proto/adapter.proto`（srv + adapter 双方）：
   - 删 `SaveAuthState` / `LoadAuthState`
   - 加 `Put(source_id, key, value bytes, expires_at)` / `Get(source_id, key)` / `Delete` / `List(source_id, prefix)`
2. srv `internal/storage/uipath_storage.go`：实现通用 KV（SQLite 一张 `kv_store(source_id, key, value, expires_at)` 表）。改名 `kv_storage.go`。
3. adapter `internal/server/account_source.go`：`saveToStorage`/`LoadAuthState` 调用换成 `Put`/`Get`，value 是 JSON。
4. 重生 .pb.go 双方，build + test。

### Task 2: OAuth 方案 1（一次登录拿 N orgs）
**改动**：
1. adapter `internal/auth/oauth.go`：
   - 拆 `Authenticate()` 为：
     - `AuthenticateWeb()` 跑 Phase 1+2，返回 `(WebSession{cookies, idToken}, []OrgInfo{name, globalID})`
     - `BootstrapOrgToken(webSess, orgID, orgName)` 跑 Phase 3+4，返回 portal token + upstream URL
   - `fetchUserOrgs` 改成返回全部 orgs（现已经打了 log，逻辑改成返回切片）
2. adapter `cmd/adapter/main.go`：启动时 1 次 `AuthenticateWeb` → 循环每个 org 调 `BootstrapOrgToken` → 全部 `AddBaseAccount`。也可以 lazy（保留 WebSession，SupplyAccounts 来时才 BootstrapOrgToken 下一个 org）。
3. adapter `internal/server/account_source.go`：`SupplyAccounts` 命中 KV 缓存优先，未命中走 BootstrapOrgToken（不再 4 阶段），过期走 RefreshToken。

### Task 3: 端到端验证（账号冷却后）
- 重启 dev.sh，看 adapter.log 拿到 N orgs
- 跑 test_anthropic.sh / test_openai.sh

## 关键文件位置
- 主仓 cwd: `/home/javion/work/tools/polyglot/new/polyglot`
- adapter: `/home/javion/work/tools/polyglot/new/uipath_adapter`
- proto 双份：`new/polyglot/src/srv/proto/adapter/adapter.proto` + `new/uipath_adapter/proto/adapter.proto`
- 凭据 .env: `new/polyglot/.env`（gitignored）
- 启动：`./dev.sh start` / stop / restart

## 重要环境约定
- protoc-gen-go 在 ~/go/bin（不在默认 PATH）
- 用 `PATH=$PATH:$HOME/go/bin ./generate.sh`
- 双仓的 proto 必须同步——package 名都是 `adapter`，go_package 各自不同

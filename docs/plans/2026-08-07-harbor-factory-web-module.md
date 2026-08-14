# Harbor Factory Web Module

将 harbor-factory 的任务板 TUI 移植为本框架的一个 Web 模块，使创题、题目管理与返修可通过 Web 页面完成。

## 命名

历史上曾用 `pptflow` 指代该系统，这是错误叫法。正确名称是 harbor factory。各层写法：

| 场景 | 写法 |
| --- | --- |
| Go 包名 | `harborfactory`（Go 包名不允许下划线；harbor 仓库已有 `internal/harborfactory` 先例） |
| SQL 表与列 | `harbor_factory_*` |
| HTTP 路径 | `/api/v1/harbor-factory/*` |
| 前端目录与路由 | `harbor-factory` |
| 权限字符串 | `harbor-factory:*` |

## 已定决策

1. **集成形态**：守护进程 + 客户端模块。harbor-factory 新增 `serve` 子命令暴露 HTTP；本框架内一个模块作为其客户端。
2. **首期范围**：读（任务板 + 详情 + 日志）、创题、审核。恢复 / 重试 / 取消留二期。
3. **部署形态**：web 与 daemon 同机。
4. **审计主体**：harbor 侧不区分创题人与审核人，保持单一 OS actor。
5. **归属语义**：用户只能看和操作自己的题，审核同样受归属过滤约束——自己的题自己审。
6. **无归属任务**：CLI/TUI 直接创建、框架无归属记录的任务，仅对持 `harbor-factory:task:read-all` 的用户可见。
7. **实时性**：SSE。
8. **共享 Git 凭据风险**：本服务仅在 localhost 使用，接受该风险，留档见"已接受风险"。

## 为何是守护进程而非内嵌

harbor-factory 的运行时模型无法安全内嵌进多副本 web 进程：

- SQLite 全局单连接：`MaxOpenConns(1)` 出现在 4 处（`store/db.go:369` 只读、`db.go:420` 可写、`db.go:653` 契约派生、`backup.go:545` 备份校验），无读写分离余地。
- 目录状态：`--root .harbor-factory` 持有控制面数据库与工作区。
- worker 是本机 `setsid` 子进程，Docker 打本机 daemon。
- `deployments/` 必须与真实可执行文件同级，显式拒符号链接。

守护进程把单写者约束关在一个进程内，web 侧保持可多副本。附带收益：`FlushQueuedRuns` 当前依赖"有人开着 TUI"来推进 queued Run，常驻 daemon 天然修掉这一点。

## 双向 `internal/` 屏障

两侧都需要新的导出面，这是先决条件：

- 改造前本框架全部代码在 `internal/` 下，且旧 module path 为非 URL 的本地路径（无法 `go get`）。外部模块无法 import。
- harbor-factory 的应用边界 `app.TaskBoardGateway` 在其 `internal/app` 下，本框架无法 import。其唯一对外可用包是 `pkg/workflowkit`，为纯决策内核，不含任务板编排。

因此 harbor-factory 需新增 `pkg/taskboardapi` 存放纯 DTO 与错误码常量（不依赖 internal），`internal/` 内写 DTO 与内部类型的 mapper。内部类型保持权威，不改动。

## 分层落点

本框架的架构测试（`tests/architecture/boundaries_test.go`）禁止 `internal/modules` 下的代码 import `net/http`、chi、pgx、go-redis、asynq 及 `internal/platform`。据此拆三层：

```
internal/modules/harborfactory/        纯域：DTO 镜像、归属与过滤逻辑、Port 接口。无 net/http
internal/modules/harborfactory/store/  Postgres 归属表访问（模块内 store 子包）
internal/platform/harborfactoryclient/ 实现 Port 的 HTTP/SSE 客户端
internal/app/harborfactory.go          路由挂载 + 认证 + RBAC + 审计 + 装配
```

框架没有后端模块注册表，接入需手工动四处：模块、store、路由注册、`internal/app` 装配。首期接受手工装配；是否抽注册表另议。

## harbor-factory 侧改动

### `cmd/serve.go`

精确复用 `cmd/tui_v2.go` 的装配链：`preflightLifecycleServices()` → `store.Open(root)` → `openLifecycleServices()` → 取 `services.TaskBoard`。差异仅在于把 bubbletea 换成 HTTP server。

必须以 root 目录锁强制单实例，否则两个 daemon 会同时打开同一 SQLite。

绑定 loopback 或 unix socket + bearer token。daemon 自身不做授权与多租户——其防护边界是"只有本框架 API 能到达它"。

### `pkg/taskboardapi`

`TaskBoardGateway` 的 12 个方法几乎可原样映射为 REST。首期只暴露 5 个方法所需的 DTO：`List`、`StartAuthoring`、`DecideReview`、`ReadRunLog`，加 SSE 流。

读模型 `TaskBoardSnapshot` 已是展示形态（待处理 / 运行中 / 已完成三列），直接镜像。

### actor

`os/user.Current()` 在 3 处被用作审计主体。因已决定不区分创题人与审核人，harbor 侧 actor **不改**，保持单一 OS actor。归属与身份完全由框架侧承担。

### SSE 事件源

**不使用 outbox。** `OutboxDispatcher` 是租约独占消费（claim / heartbeat / ack / nack，返回 nil 即 ACK），`Topics` 仅限制某 worker 处理哪些 topic。挂第二个 dispatcher 到已被真实副作用消费的 topic 会与之抢事件——每条事件只会进其中一个，破坏现有派发。

改用 daemon 内**单个 poller** 读 store 做快照 diff，向所有已连接浏览器 fan-out。这把 N 个客户端轮询压成 1 个内部 poller，直接缓解 `MaxOpenConns(1)` 瓶颈。

事件设计为**瘦失效通知**而非全量推送：事件只带 `task_id` / `run_id` 与变更类型，浏览器据此 refetch 对应资源。`List()` 无分页，全量推送不可持续。

后续优化方向：in-process commit observer 取代 poller（daemon 单进程单写者，提交后回调是安全的），需 harbor 侧改动，非首期。

## 本框架侧改动

### 归属模型

harbor 是单租户执行引擎，多租户完全由本框架承担。新增归属表：

```
harbor_factory_task_owners(
  task_id, owner_user_id, idempotency_key, created_at
)
```

daemon 的 snapshot 返回全量，框架按 `owner_user_id` 过滤后返给前端。审核、日志读取、详情均先校验归属再代理。

### 原子性：先写意图

daemon 建 task 成功、框架记归属前崩溃，task 会成为无主孤儿。因此顺序必须是：

1. 框架生成 UUIDv7 幂等键，落 `(idempotency_key, owner_user_id)`，task_id 暂空。
2. 携该键调 daemon `StartAuthoring`。
3. 拿到 task_id 回填。

任何时刻崩溃都能靠幂等键恢复出"谁发起的"，复用 harbor 已有幂等语义，无需另造对账机制。

幂等键由框架侧生成（两侧均有 `google/uuid` v1.6.0，`NewV7()` 可用），不走 daemon 的 `NewIdempotencyKey`。这样键归属浏览器会话，表单重试可复用同一键，对上 TUI 既有语义："保留原表单重试会复用同一键和原始 durable 回执"。

### 身份与鉴权

均为现成能力，无需新建：

- `iamhttp.Authenticate(jwt)` 将 subject 注入 context（`iam.WithSubject`）。
- handler 内 `iam.Subject(ctx)` 取用。
- `iamhttp.RequirePermission(app, "...")` 做 RBAC，权限为字符串，授予到 role。

权限位：

| 权限 | 含义 |
| --- | --- |
| `harbor-factory:task:read` | 读自己的题 |
| `harbor-factory:task:read-all` | 跨用户读，含无归属的历史任务 |
| `harbor-factory:authoring:create` | 创题 |
| `harbor-factory:review:decide` | 审核（仍受归属过滤约束） |

### API 契约

`api/openapi_test.go` 强制约束，新端点必须遵守：

- 非 public 端点 `security` 必须为 `bearerAuth` 与 `accessCookie` 并列（OR 语义）。
- 响应标准搭配 `422` / `401` / `403`；写操作按需加 `404`、`409`。
- 现有 29 条端点中 `public: true` 仅 6 条，新端点均为 `false`。
- 最完整范本为 `POST /api/v1/task-executions`（五个响应全带），直接抄其形状。

首期端点：

```
GET  /api/v1/harbor-factory/tasks                 任务板快照（已按归属过滤）
GET  /api/v1/harbor-factory/tasks/{id}            详情
GET  /api/v1/harbor-factory/runs/{id}/log         日志尾部
POST /api/v1/harbor-factory/authoring             创题
POST /api/v1/harbor-factory/reviews/decision      审核裁决
GET  /api/v1/harbor-factory/events                SSE 失效通知流
```

### 错误映射

harbor store 有 18 个哨兵错误，需完整映射。已确认的关键项：

| 哨兵错误 | HTTP | 备注 |
| --- | --- | --- |
| `ErrOptimisticLock` | 409 | 可重试 |
| `ErrIdempotencyConflict` | 409 | **不可**重试 |
| `ErrLeaseHeld` / `ErrFencingToken` | 423 / 409 | |
| `ErrTaskPurged` | 410 | |
| `ErrDispatchFenceLost` | 409 | 租约失效 |

其余 13 个在实现时补全。租约常量：`DefaultLeaseTTL = 90s`、`DefaultLeaseHeartbeatInterval = 20s`（`v2_jobs.go:12-13`）。

### SSE 的两个前提

1. **`WriteTimeout` 会掐断长连接**。`internal/app/api.go:161` 设 `WriteTimeout: 30s`，且 `api_test.go:74` 断言其 `<= 30s`。解法：SSE handler 内用 `http.ResponseController.SetWriteDeadline(time.Time{})` 单独解除。不动全局值，不破坏既有测试（Go 1.26 支持）。
2. **middleware 链可穿透**。`middleware_test.go:136` 已断言下游能看到 `http.Flusher`，无缓冲阻碍。

其他注意：`EventSource` 无法设自定义头，SSE 端点认证必须走 `accessCookie` 分支（框架已支持）；归属校验在流打开时做一次；反向代理需关闭缓冲（nginx 置 `X-Accel-Buffering: no`）。

### 前端

框架已支持"一行挂载"：在 `app/(app)/harbor-factory/page.tsx` 内一行 import 该 feature 即可。无需新建 feature registry——此前认为需要自建是错误判断。

feature 落在 `web/src/features/harbor-factory/`。技术栈沿用现有：Next.js 16 + React 19 + Radix + TanStack Query + Tailwind。SSE 事件到达后调用 TanStack Query 的 invalidate，而非手动改 store。

## 必须复刻的三个机制

丢掉任一都会丢掉对应的安全属性：

1. **epoch 丢弃陈旧响应**。TUI 用 epoch 使过期响应失效，Web 侧需等价机制（TanStack Query 的请求版本 / abort）。
2. **幂等键跨重试保留**。见上文"先写意图"。
3. **preview → confirm 的 checkpoint + fingerprint 陈旧校验**。天然对应 ETag / If-Match。丢掉它就丢掉"计划已变则拒绝而非静默重定向"。首期不含恢复流程，但二期必须实现，接口设计需预留。

## 已接受风险

**共享 Git 凭据无法按用户隔离**。私有仓库 clone 依赖宿主机 ssh-agent socket（`HARBOR_FACTORY_STANDARD_AUTHORING_SSH_AUTH_SOCK`）。多租户后，任何持 `harbor-factory:authoring:create` 的用户都能让 daemon clone 该凭据可读的任意私有仓库。

因本服务仅在 localhost 使用，接受此风险。缓解措施：谨慎授予 `harbor-factory:authoring:create`。若将来对外暴露，此项必须先解决。

## 顺带修复

`displayStageName` 与生产 v3 catalog 已漂移：16 个 stage key 仅 6 个有中文名，映射表中约 14 条指向已退役流程。应从 `deployments/.../operation-catalog.v1.json` 生成而非手写。

## 实施顺序

遵循框架 `AGENTS.md` 的 TDD 要求：每步先加失败测试再写实现。

1. harbor-factory：`pkg/taskboardapi` DTO + mapper（含 DTO 与内部类型的往返测试）。
2. harbor-factory：`cmd/serve.go` + root 单实例锁 + 5 个只读/写端点。
3. harbor-factory：SSE poller + fan-out。
4. 框架：`internal/modules/harborfactory` 域逻辑 + 归属过滤（纯单元测试，无 I/O）。
5. 框架：归属表 migration + store。
6. 框架：`internal/platform/harborfactoryclient` HTTP/SSE 客户端。
7. 框架：OpenAPI 补 6 条端点 + `internal/app/harborfactory.go` 路由与装配。
8. 框架：前端 feature + 一行挂载。
9. 端到端：创题 → 看板 → 详情 → 日志 → 审核。

每步收尾运行 `go test ./...`、`go vet ./...`、`go build ./...`。

## 二期

- 恢复 / 重试 / 取消（含 preview → confirm 与 fingerprint 校验）。
- 迁 Postgres。代价明确：75 张表、大量 CHECK 约束与触发器、乐观锁、79 处内联 `BeginTx`。优先级高于最初估计，因单连接是最终吞吐瓶颈。
- 实时逐行输出。底层已有 `StreamingConversation.TurnStream`、`TurnUpdate` 增量与 `RunStreamingWithOutput`，但 TUI 完全未用（仅读日志文件尾部 64 KiB + 手动刷新）。接上后可替代日志轮询。
- 后端模块注册表，把四处手工装配收敛为一行。
- in-process commit observer 取代 SSE poller。

## 待核实项

以下为调研中未能确认、实施前需先验证的点：

1. **旧 quota 表族是否为死表**：`quota_accounts`、`quota_leases`、`quota_ledger_entries`、`quota_policies`、`budget_grants`、`budget_settlements` 无 CHECK 约束，是否仍被 Go 层写入未确认。迁 Postgres 前必须先定死活。
2. harbor 侧 app 层完整 API 面、执行模型、`deployments/` 加载三块结论来自直读代码（附文件与行号可复核），其对应的并行深挖未返回。
3. 框架侧平台能力清单同上。
4. 首期端点的 `409` 与 `422` 边界需在实现时结合 18 个哨兵错误逐一敲定。

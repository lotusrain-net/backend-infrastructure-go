# Backend Infrastructure Go Code Review

## 审查结论

- 审查日期：2026-07-22
- 审查范围：整个仓库，包括 118 个 Go 文件，以及 SQL、OpenAPI、Docker 和 Compose 配置
- 技术栈：Go 1.26、Chi、pgx/sqlc、PostgreSQL、Redis、Asynq、Prometheus
- 审查建议：**REQUEST CHANGES**
- 总体评分：**7.2 / 10**

项目的基础工程、测试结构、鉴权细节和部署加固高于一般脚手架，但任务可靠投递和声明的架构边界存在阻断级问题，当前不建议直接作为高可靠通用基底发布。

## 1. 架构与分层

### HIGH：领域层未真正独立于基础设施适配器

项目约定 `internal/modules` 只拥有领域和应用行为，基础设施适配器应由 `internal/platform` 持有，但实际代码存在以下越界：

- [`internal/modules/iam/http.go`](../../internal/modules/iam/http.go#L3) 直接依赖 `net/http` 和 Chi，同时包含路由、认证中间件和 HTTP handler。
- [`internal/modules/audit/requestmeta/requestmeta.go`](../../internal/modules/audit/requestmeta/requestmeta.go#L16) 在审计模块目录中实现 HTTP 请求元数据中间件。
- [`internal/modules/audit/postgres/repository.go`](../../internal/modules/audit/postgres/repository.go#L10) 直接依赖 pgx 和 `internal/platform/database/dbgen`。

这使模块目录无法作为可靠的领域复用边界。建议将 IAM HTTP、审计请求元数据和审计 PostgreSQL repository 分别迁移到 `internal/platform/http`、`internal/platform/http/requestmeta` 和 `internal/platform/database/auditstore`。

### MEDIUM：架构测试未覆盖声明的真实边界

[`tests/architecture/scanner_test.go`](../../tests/architecture/scanner_test.go#L61) 将 `/postgres/`、`/adapter/` 等目录排除在基础设施依赖检查之外，且没有禁止模块导入 `net/http` 或 Chi。这种检查更接近目录命名约束，而不是依赖方向约束。

建议直接禁止非适配器模块包导入 HTTP、pgx、Redis、Asynq 和 `internal/platform`，不要通过目录名称提供例外。

### MEDIUM：组合根职责重复

[`internal/app/run.go`](../../internal/app/run.go#L15) 与 [`internal/bootstrap/command.go`](../../internal/bootstrap/command.go#L12) 都负责配置加载、日志创建和组件生命周期。实际 API、worker、scheduler 装配又位于 `internal/app`。

建议收敛为一个 composition root，由 `cmd` 或 `internal/bootstrap` 统一拥有装配行为。

## 2. 可扩展性与设计模式

### 优点

任务模块通过 `ExecutionStore`、`Publisher`、`Handler`、`Registry` 和 observer 接口进行依赖反转，核心逻辑易于替换和测试。

### MEDIUM：任务扩展点被跨层硬编码抵消

- [`internal/app/platform_http.go`](../../internal/app/platform_http.go#L94) 仅允许提交 `system.test`。
- [`internal/app/tasks.go`](../../internal/app/tasks.go#L75) 仅向 worker 注册 `system.test` 和调度分发任务。
- [`internal/app/api.go`](../../internal/app/api.go#L121) 和 worker 指标只声明 `system.test`。
- OpenAPI 测试同样固定该任务类型。

增加新任务需要同时修改 API、worker、指标和契约测试。建议引入统一 `TaskCatalog`，由组合根注入任务定义，并驱动输入验证、worker 注册、指标标签及 OpenAPI 契约。

### LOW：存在未兑现的基础设施抽象

`CircuitBreaker`、`Retry` 和资源 `Stack` 都有实现及测试，但生产装配没有使用；`BREAKER_FAILURES` 和 `BREAKER_TIMEOUT` 因此属于无效运维配置。

应将其接入明确的依赖调用，或者删除尚未使用的抽象与配置，避免产生错误的容错预期。

## 3. 代码质量与可读性

### 优点

- 命名总体明确，领域错误、repository 接口和 adapter 类型容易理解。
- SQL 通过 sqlc 生成，业务代码中没有发现手写 SQL 拼接。
- 大多数核心函数职责集中，公共 API 的测试覆盖较充分。

### MEDIUM：运行时装配函数承担过多职责

[`internal/app/api.go`](../../internal/app/api.go#L91) 和 [`internal/app/tasks.go`](../../internal/app/tasks.go#L68) 同时处理连接创建、业务服务装配、指标、路由、队列和资源清理。新增依赖时容易漏掉错误路径的关闭逻辑。

建议按 API、worker、scheduler 拆分资源 builder，并复用统一资源栈；不建议引入通用 controller/service 基类。

### LOW：HTTP 解码和错误映射存在局部重复

IAM HTTP 和平台 HTTP 分别实现 JSON 限制、未知字段检查和验证错误输出。可以抽取小型、明确的 HTTP JSON helper，但应避免形成过度通用的反射式框架。

## 4. 错误处理与健壮性

### HIGH：任务落库和 Redis 入队不是原子操作

[`internal/modules/task/service.go`](../../internal/modules/task/service.go#L68) 先写入 PostgreSQL execution，再于第 86 行发布到 Asynq。若进程在两步之间崩溃，将永久留下状态为 `queued`、实际却从未入队的任务。

发布失败后的补偿更新无法覆盖进程崩溃、网络结果不确定等场景。建议使用 transactional outbox，或者引入 `pending_publish` 状态、可靠 dispatcher 和周期性 reconciliation。

### HIGH：任务状态缺少原子认领

[`db/queries/tasks.sql`](../../db/queries/tasks.sql#L15) 的状态更新没有旧状态条件，processor 使用“读取状态后再写入”的方式。Asynq 至少一次投递或多个 worker 并发处理时，可能重复执行同一业务任务。

建议使用条件更新实现 compare-and-set，例如仅允许 `queued -> running`，检查 affected rows，并为长任务增加 lease/heartbeat 或明确的幂等约束。

### MEDIUM：依赖故障被转换为认证失败

[`internal/modules/iam/service.go`](../../internal/modules/iam/service.go#L23) 将用户 repository 的任意错误转换成 `ErrInvalidCredentials`；刷新流程也将 Redis 或数据库故障转换成无效 token。

这会把 PostgreSQL/Redis 故障错误地返回为 401，并削弱告警和故障定位。只有 `ErrNotFound`、密码不匹配、已撤销 token 等真实认证失败应被隐藏，其余错误应映射为 5xx。

### MEDIUM：最终任务失败状态写入错误被吞掉

[`internal/platform/queue/asynq.go`](../../internal/platform/queue/asynq.go#L108) 调用 `HandleExhausted` 后直接丢弃错误，可能让已经耗尽重试的任务永久停留在 `running`。

至少应记录结构化错误日志和指标，并提供状态修复或死信 reconciliation。

## 5. 安全与数据校验

### 优点

- SQL 使用参数化查询，未发现 SQL 注入入口。
- 刷新令牌以哈希形式存储，并通过 Lua 脚本原子轮换。
- Cookie 使用 `HttpOnly` 和 `SameSite=Strict`，生产环境强制 `Secure`。
- 生产配置会拒绝公开占位 JWT、管理员和数据库密码。

### MEDIUM：生产配置允许明文依赖连接

[`internal/config/config.go`](../../internal/config/config.go#L195) 的生产校验没有禁止 PostgreSQL `sslmode=disable`。Redis 和 Asynq 配置也没有 TLS 配置入口。

内部 Compose 网络可显式作为例外，但通用生产部署应支持 CA、server name 和最低 TLS 版本，并默认要求加密连接。

### MEDIUM：Redis 限流故障时高成本接口 fail-open

[`internal/platform/httpserver/middleware.go`](../../internal/platform/httpserver/middleware.go#L253) 在 limiter 出错时放行非 critical 请求，而 [`internal/app/api.go`](../../internal/app/api.go#L133) 仅将认证路径标记为 critical。

Redis 故障期间，任务提交和审计查询将失去限流保护。建议将高成本写接口加入 critical 集合，或者提供本地后备限流器。

### MEDIUM：任务数值输入只有下限，没有上限

[`internal/app/platform_http.go`](../../internal/app/platform_http.go#L108) 未限制最大重试次数、最大超时和最大延迟。超大值可能造成 `time.Duration` 溢出或长期资源占用。

应根据平台能力设置显式上限，并在领域服务层再次验证，不能只依赖 HTTP handler。

## 6. 性能与资源管理

### 优点

- HTTP server 配置了 header/read/write/idle timeout。
- pgx 使用连接池，Redis 和数据库连接在主要关闭路径中得到释放。
- [`internal/bootstrap/lifecycle.go`](../../internal/bootstrap/lifecycle.go#L16) 支持取消、反向顺序 shutdown 和有界关闭时间。

### MEDIUM：审计查询不适合大数据量

[`internal/modules/audit/postgres/repository.go`](../../internal/modules/audit/postgres/repository.go#L66) 每次分页都执行列表查询和完整 `COUNT(*)`。[`db/queries/audit.sql`](../../db/queries/audit.sql#L8) 使用多组可选 OR 过滤及 OFFSET 分页，无过滤查询也缺少与 `created_at DESC, id DESC` 完全匹配的索引。

建议使用 keyset pagination、按实际过滤组合增加复合索引，并将精确 total 计数改为可选能力。

### HIGH：重复执行会放大资源消耗

任务缺少原子 claim，在多 worker 或 Redis 重投递场景下可能重复消耗数据库连接、CPU 和外部 API 配额。该问题应与任务投递一致性一起处理。

## 7. 可测试性

### 优点

- 118 个 Go 文件中有 60 个测试文件。
- 核心包覆盖率多数在 75% 到 95%：`bootstrap` 95.1%、IAM 78.4%、task 84.3%、HTTP server 82.1%。
- 存在架构、OpenAPI、HTTP contract 和 Compose E2E 测试入口。

### MEDIUM：默认测试未覆盖真实依赖链

常规 `go test ./...` 会在未配置环境变量时跳过 PostgreSQL、Redis、迁移和 Compose E2E。因此跨存储一致性、重启恢复和多实例行为没有被默认 CI 证明。

### 缺失的关键回归测试

- execution 已落库但进程在入队前崩溃后的恢复测试。
- 同一 Asynq 消息被重复或并发投递时仅执行一次的测试。
- 两个 API 实例并发初始化管理员的测试。
- PostgreSQL/Redis 故障应返回 5xx 而不是 401 的认证测试。
- 超过 1 MiB IAM 请求体的契约测试。

## 8. 配置与部署

### 优点

- Docker 使用 scratch runtime 和非 root 用户。
- Compose 使用只读文件系统、tmpfs、`no-new-privileges` 和内部后端网络。
- API、worker、scheduler 启动依赖健康检查和成功迁移。

### MEDIUM：API 稳态运行依赖一次性管理员秘密

[`internal/app/api.go`](../../internal/app/api.go#L106) 在每次 API 启动时执行管理员引导；[`internal/config/config.go`](../../internal/config/config.go#L152) 因此要求 API 容器长期持有管理员初始密码。

管理员初始化属于控制面操作，建议迁移为单独的 seed/init 命令，并在成功后从 API 运行环境移除初始密码。

### MEDIUM：管理员引导不支持并发启动

[`internal/app/admin.go`](../../internal/app/admin.go#L22) 使用“查询后插入”。两个 API 副本首次并发启动时都可能判断管理员不存在，其中一个将因唯一约束失败而终止启动。

建议使用 UPSERT，或在唯一冲突后重新查询，并增加并发集成测试。

### LOW：运维配置与实际能力不一致

`BREAKER_FAILURES` 和 `BREAKER_TIMEOUT` 会被加载和校验，但没有生产调用方。应删除无效配置或完成熔断器接入。

## 总体评价

项目具备以下明显优势：

- 领域服务普遍采用接口注入，单元测试友好。
- IAM 的密码、JWT、刷新令牌和 Cookie 处理较为谨慎。
- HTTP 生命周期、错误响应、指标和部署基线比较完整。
- 数据库迁移、约束和 sqlc 查询契约清晰。

但作为其他业务模块依赖的基础架构，以下问题必须优先解决：

1. PostgreSQL 与 Redis 之间缺少可靠任务投递机制。
2. 领域目录混入 HTTP 和数据库适配器，声明的依赖边界不可信。
3. 管理员初始化、任务类型注册和 TLS 配置仍偏向单实例演示环境。

## 最优先行动项

1. **可靠任务执行**：落地 transactional outbox、原子任务 claim、重启 reconciliation 和重复投递测试。
2. **修复架构边界**：将 HTTP/PostgreSQL 适配器移出 `internal/modules`，收敛 composition root，并强化架构测试。
3. **生产化运行配置**：将管理员初始化迁移为一次性流程，补齐 PostgreSQL/Redis TLS、依赖故障错误映射和真实依赖集成测试。

## 验证记录

以下命令执行成功：

```text
go test -count=1 ./...
go vet ./...
go build ./...
git diff --check
```

验证限制：

- `govulncheck` 和 `staticcheck` 未安装。
- 当前环境 `CGO_ENABLED=0` 且没有 GCC，未运行 race detector。
- PostgreSQL、Redis、迁移集成测试和 Compose E2E 因相关环境变量未配置而跳过。

本次审查未修改生产代码。

# 认证基础设施

正式控制台位于 `web/`。认证设置以父 Issue #1 的默认值和流程为准；`tmp/` 不参与实现。

## 初始化与升级

执行迁移后运行 `seed-admin`。它锁定单例 `authentication_settings`，创建或复用 `ADMIN_EMAIL` 对应的账号、分配超级管理员角色，并绑定 `bootstrap_admin_user_id`。重复 seed 不修改已有密码，也不允许改绑另一个账号。升级已有安装时继续使用原来的 `ADMIN_EMAIL`；迁移不从多个管理员中猜测 bootstrap 身份。

`initialized_at` 为空时，只有该 bootstrap 管理员能通过密码登录。成功签发会话后原子记录初始化时间。其他登录统一 `401`，公开注册及注册验证码统一 `403 operation not permitted`。历史 access/refresh 会话也不能绕过初始化门槛。初始化字段不在可编辑 DTO 中，服务器拒绝未知 JSON 字段。

## 设置与登录

`GET/PUT /api/v1/system-settings/basic-auth` 分别要求 `system-settings:read` / `system-settings:write`，超级管理员的 `*` 权限同样有效。

| 字段 | 默认值 | 行为 |
| --- | --- | --- |
| `password_login_enabled` | `true` | 初始化后是否允许新的密码登录 |
| `registration_enabled` | `false` | 是否允许公开注册 |
| `registration_email_verification_required` | `true` | 与注册开关独立保存；开启时注册必须提供邮箱码 |
| `allowed_email_domains` | `[]` | 空数组不限制，否则只接受指定域名，不支持通配符 |

注册成功返回 `201` 用户资料，不签发 token 或 cookie。关闭注册邮箱验证时，新用户保持 `email_verified_at=null`，可以密码登录。

个人安全策略 `mode` 为 `default`、`email`、`totp`。`default` 不要求邮箱码；`email` 仅适用于已验证邮箱，与注册邮箱验证开关无关。启用 `email` 时，修改邮箱返回 `403`，原邮箱与验证时间保持不变；同邮箱的资料修改不受影响。用户需先将安全策略改为 `default` 才能修改邮箱。其他模式下修改邮箱会清空验证时间；当前尚无既有账号重新验证邮箱入口，因此修改后不能再次启用 `email`。邮箱修改与邮箱策略启用在数据库中串行校验，避免并发操作使账号无法登录。

登录页仅填写邮箱和密码，不再提供“我的账号启用了邮箱二次验证”选择框；注册页也不预先显示验证码输入。服务端要求邮箱验证且请求未提供 `email_code` 时，返回 `422`，`data` 为 `{"email_code":"required"}`，前端据此弹出验证框，获取验证码后附带原始表单信息重新提交。登录仅在账号密码校验成功后返回该信号；注册先校验开放策略、邮箱域名和基本信息。不填写验证码不会消耗验证码尝试次数；填写错误或过期验证码仍返回 `401`。

邮箱验证码、TOTP 一次性代码和恢复码共用身份验证弹窗，统一提交状态、错误重试及返回操作。六位代码只接受数字，恢复码支持原有字符；邮箱验证码保留手动发送和重发倒计时。取消验证会丢弃已输入的验证码，验证期间不允许重复提交或关闭弹窗。

TOTP 密码登录返回 `202 {status:"totp_required",challenge_id,expires_in:300}`，不设置认证 cookie。`POST /api/v1/auth/login/totp/verify` 接受 challenge 和 `code` 或 `recovery_code` 二选一。验证成功后才返回会话。正式前端在 challenge 阶段不查询当前用户、不导航、不写入当前用户缓存。

## 验证码与事务

验证码由 `crypto/rand` 生成六位数字，有效期十分钟、冷却六十秒、最多五次错误。Redis key 按环境、用途和规范化邮箱的 HMAC 摘要隔离；值只保存验证码 HMAC、随机消费 ID 和错误次数。Lua 原子执行冷却、每邮箱每小时十次、每 IP 每小时五十次限流及验证码验证/消费。`429` 携带 `Retry-After`；前端据此倒计时。请求失败、未知邮箱的发送响应不披露账户存在状态，发送也不查找用户以区别 SMTP 路径。

PostgreSQL 与 Redis 不共享事务。采用已确认的方案：注册事务锁定当前认证策略，并在 `authentication_consumptions` 唯一插入验证码消费 ID、创建用户、标记邮箱、分配 `user` 角色。任一步失败全部回滚，原验证码可重试；提交后再清理 Redis。即使清理失败，唯一消费记录也阻止重放。过期消费记录在后续消费事务中清理。

## SMTP 与密钥

API 需要以下独立、稳定的密钥，各由 `openssl rand -hex 32` 生成：

- `AUTHENTICATION_KEY`：32 字节 AES-256-GCM 加密密钥，用用户 ID 作为关联数据。
- `AUTHENTICATION_PEPPER`：32 字节 HMAC pepper，用于验证码、缓存 key 和恢复码摘要。

保持这些值跨重启不变。修改加密密钥而未重新加密已有密文，会导致现有 TOTP 密钥无法解密；本次不实现密钥轮换。

生产必须 `SMTP_ENABLED=true`，配置 `SMTP_HOST`、`SMTP_PORT`、`SMTP_USERNAME`、`SMTP_PASSWORD`、`SMTP_FROM`。`SMTP_TLS_MODE=starttls`（默认，通常端口 587）或 `tls`（通常端口 465），均验证服务端证书，禁止明文降级；`SMTP_TIMEOUT` 默认十秒。证书、认证失败与超时映射为脱敏错误。开发/测试只有显式 `SMTP_ENABLED=false` 才跳过 SMTP；仍保存 Redis 凭据并记录用途及收件人摘要，不记录原始邮箱或验证码。需要手工完成邮箱验证时请配置可收信的开发 SMTP。

`./scripts/compose-up.sh` 创建新的 `.env` 时自动生成独立密钥。升级已有 `.env` 时手动补齐这两个值及 SMTP 配置；脚本不会覆盖现有秘密。

## 两步验证生命周期

`GET/PUT /api/v1/users/me/security` 读取/修改个人策略。`POST /api/v1/users/me/security/totp/enroll` 验证当前密码后返回十分钟内有效的手动密钥和 `otpauth_uri`，二维码由浏览器本地生成。`confirm` 接受六位一次性代码，成功后只展示一次十个随机恢复码。数据库只保留加密 TOTP 密钥和恢复码 HMAC。恢复码默认有效期为 30 天（`RECOVERY_CODE_TTL=720h`，必须为正时长），资料页显示到期时间；过期后即使摘要匹配也不可消费，使用一次或停用后立即失效。

TOTP 使用 RFC 6238、SHA-1、六位数字、三十秒周期，接受前后一个时间窗口，并通过数据库行锁和最后使用的时间步阻止重放。challenge 最多五次尝试；数据库保存其摘要消费记录。恢复码消费会原子更新安全版本，使旧 challenge 和旧设置写入失效，防止恢复已消费凭据。

密码重置在同一 PostgreSQL 事务中更新密码、递增安全版本并清除待确认的 TOTP 绑定。重置前获取的 challenge 无法再使用 TOTP 或恢复码登录；已启用的 TOTP 及未使用的恢复码保留，必须用新密码获取新 challenge。

停用要求当前密码和 TOTP/恢复码；停用后再次提交有效密码具备幂等行为，每次重试都会完成 refresh 会话撤销，包括数据库停用已提交但 Redis 撤销失败的情况。安全设置不允许通过普通 PUT 静默关闭已启用的 TOTP。短时凭据仅在请求体或内存中传递；响应包含 `Cache-Control: no-store`，前端不持久化这些值。

`enroll`、`confirm`、`disable` 均要求登录。会话缺失或 access token 过期返回 `401`，前端可刷新会话并重试一次；已登录请求的密码、TOTP 或恢复码错误返回 `422`，`data.proof` 为 `invalid or expired credential`，不会刷新、自动重放验证码或清除登录状态。公开登录接口的无效凭据仍返回 `401`。

## 验证

常规检查：`go test ./...`、`go test -race ./...`、`go vet ./...`、`go build ./...`，及 `web/` 下 `npm run lint`、`npm run typecheck`、`npm test`、`npm run build`。

设置 `AUTH_TEST_DATABASE_URL` 后，数据库事务测试和认证 HTTP 集成测试使用随机 schema，结束时只清理自己的 schema：

```sh
AUTH_TEST_DATABASE_URL='postgres://.../test?sslmode=disable' go test ./tests/integration ./tests/e2e
```

`MIGRATION_TEST_DATABASE_URL` 用于既有迁移 up/down/up 测试，必须指向可丢弃的独立测试库。Redis Lua 测试使用 miniredis；SMTP 测试使用本地 TLS/STARTTLS 服务和证书。

CI 的 PostgreSQL 任务同时设置上述两个数据库变量，先执行迁移测试，再以 `-race -count=1 -v` 执行 `iamstore` 与 `tests/e2e`，覆盖认证事务、并发消费、故障重试与 HTTP 全流程。

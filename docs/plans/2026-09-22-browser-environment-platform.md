# 浏览器环境平台（Undetectable 方案）— 总体设计

- 状态：设计草案（仅总体设计，不含实现）
- 日期：2026-09-22
- 关联仓库：本仓库承载 Go 控制面与管理控制台；用户机**节点代理（Node Agent）为独立 Go 仓库**，厂商浏览器为**商业闭源软件**
- 指纹引擎：**Undetectable Fingerprint Browser**（Chromium 内核，商业授权，本地 REST API + CDP）
- 取代：`2026-09-22-camoufox-browser-environment-platform.md`（Camoufox 方案，已废弃，见 §19 变更历史）

---

## 0. 结论摘要

平台分三层，职责严格分离：

1. **控制面（本仓库，Go）**：多租户与 IAM、环境库、凭据保险库、代理池、客户端记录与调度、会话生命周期、审计与可观测。
2. **管理控制台（本仓库 `web/`，Next.js）**：管理员管理账号、环境、代理、席位与设备；用户凭邀请注册、登录、发起会话。
3. **节点代理（独立 Go 仓库）**：运行在用户本机 Windows / macOS arm64，是**唯一能访问厂商本地 API 的进程**，把平台指令翻译为 Undetectable 的 profile 操作与浏览器动作；驱动 Chromium 内核执行指纹与页面加载。

核心边界：**本仓库不引入任何浏览器自动化运行时**；厂商浏览器与节点代理均为外部交付物。服务端通过**客户端主动建立的 WebSocket 长连**下发环境、账号与会话指令，数据面（页面流量）经代理直连出站。

**与 Camoufox 方案的关键差异**：

- 厂商为 **Chromium 内核** → 原生支持 Client Hints，消除上一版最大的兼容缺口。
- 厂商直接提供 **Windows / macOS arm64** 安装包 → **不再需要 Linux 构建机与交叉编译**。
- 暴露**真实 CDP** → 节点代理可用 **Go（chromedp）**直连，**无需 Python**，`AGENTS.md` 禁令不被触碰。
- 指纹不再是自研生成，改为**厂商「配置(configid)」+ 有限覆盖** → 指纹可控度下降，需正面处理。
- 引入**商业授权与席位成本**、**本地 API 无鉴权**、**内核/配置漂移**等新风险。

---

## 1. 术语与角色

| 术语 | 含义 |
|------|------|
| 租户 Tenant | 隔离单元。用户、环境、账号、代理、会话均归属某租户 |
| 平台管理员 | 跨租户运维，管理租户、配额、全局代理与席位 |
| 租户管理员 | 管理本租户的环境、账号库、代理、邀请与成员 |
| 用户 User | 凭邀请注册的终端用户，是**唯一身份主体**（控制台与客户端同一账号） |
| 节点代理 Node Agent | 独立 Go 程序，装在用户机，驱动厂商浏览器，是平台的执行适配层 |
| 环境 Environment | `厂商配置(configid) + 覆盖(overrides) + 代理策略 + 账号绑定` 的强绑定单元 |
| 厂商配置 Vendor Config | Undetectable 提供的指纹配置，含 UA/屏幕/WebGL/字体/navigator 等成套数据 |
| Profile | 厂商浏览器的用户配置实例（指纹+代理+cookie 的载体），**每会话临时新建** |
| 凭据 Credential | 目标站点账号密码（加密存于保险库），**运行时注入，不落盘** |
| 代理 Proxy | 出口 IP 资源，含协议、认证、地理与风控标签 |
| 会话 Session | 一次「用户机 ↔ 环境 ↔ 代理 ↔ 账号」的租用，有生命周期与独占性 |
| 席位 Seat | 该用户机上 Undetectable 的付费授权；一人一席，公司统一采购 |

---

## 2. 需求（已确认）

1. **多租户**：管理员管理环境与账号并下发给客户端；租户间强隔离。
2. **用户自助注册（邀请制）**：管理员生成邀请码/邀请链接，用户凭邀请注册并加入指定租户；无邀请不可注册。
3. **单一身份**：用户账号同时用于控制台与客户端；客户端只是"这台机器"的记录，非独立账号。
4. **客户端下拉选具体配置**：管理员发布具体环境清单，用户选中后服务端自动配齐该环境的**独占账号 + 指纹配置 + 代理**。
5. **指纹粒度**：环境 = 厂商配置 + 覆盖 + 代理强绑定，做逻辑自洽校验。
6. **代理池与质量管控**：存活、属地真实性、风控标签检测与会话绑定；逐 profile 注入。
7. **加密凭据库**：平台内加密存储，按会话短期取件，**运行时注入、不落盘**。
8. **运行位置**：厂商浏览器运行在用户本机 macOS / Windows；Go 服务端仅下发控制。
9. **目标平台**：Windows + macOS arm64。
10. **Profile 生命周期**：**每次会话临时新建**，用完清理。
11. **账号注入**：**运行时注入，不落盘**（密钥仅驻留节点代理内存）。
12. **自动化通道**：**优先用厂商内置 `/browser/*` 动作，能力不足再回退到 CDP**。
13. **版本策略**：**锁定内核/配置版本，手动灰度升级**。
14. **席位**：**公司统一采购，一人一席**。
15. **会话触发**：用户手动发起（非定时批量）。
16. **账号占用**：**账号独占**——同一站点账号同一时刻仅归一个会话。
17. **不可用环境处理**：不做置灰、不做排队；服务端**过滤掉被占用环境**，空列表显示"无可用环境"。
18. **前端**：复用现有 Next.js 做管理控制台。
19. **开放 API**：本期不做，仅控制台 + 节点面。

---

## 3. 架构总览

```text
                        ┌──────────────────────────────────────────────┐
                        │      控制面 Control Plane (本仓库, Go)        │
                        │  IAM/多租户 · 环境库 · 凭据保险库 · 代理池      │
                        │  客户端记录 · 会话调度 · 席位/授权 · 审计/指标  │
                        │  Asynq: 代理体检/凭据轮换/会话回收/配置同步     │
                        └───────▲───────────────────────┬──────────────┘
              HTTPS/WSS(出站长连)│                       │  REST /api/v1
                                │                       │
        ┌───────────────────────┴───────┐      ┌────────┴─────────────────┐
        │  节点代理 (独立 Go 仓库)       │      │  管理控制台 (web/, Next)  │
        │  本地 API 客户端 + chromedp    │      │  环境/账号/代理/席位/会话  │
        │  ↓ 127.0.0.1:25325            │      │  复用 IAM/任务/审计/双主题 │
        │  Undetectable (Chromium 内核)  │      └──────────────────────────┘
        │  指纹+代理+cookie 于 profile   │
        └───────────────┬───────────────┘
                        │ 指纹在浏览器内核层生效；出站流量经指定代理
                        ▼
                 目标站点 / 代理出口网络
```

数据面流量由节点经代理直连出站，**不经过控制面**；控制面只承载控制信令与元数据。

---

## 4. 组件职责

### 4.1 控制面（本仓库）

复用现有能力并新增领域模块：

- **复用**：`internal/modules/iam`、`audit`、`task` + Asynq（`internal/platform/queue`、`scheduler`）、`database`、`httpserver`、`observability`。
- **新增领域模块**（领域包保持无适配器依赖）：
  - `tenant`：租户、配额、成员与角色、邀请。
  - `environment`：环境定义（configid + overrides）、版本、发布自洽校验、厂商配置目录。
  - `credential`：凭据保险库（加密、轮换、授权、短期取件）。
  - `proxy`：代理资源、健康与风控标签、绑定与归还。
  - `device`：客户端记录、在线状态、能力上报、命令队列、席位/授权信息。
  - `session`：会话编排（原子分配 环境×代理×账号×客户端）、生命周期、回收。
- **新增平台适配器**：`internal/platform/kms`（信封加密主密钥抽象）、`internal/platform/controlchannel`（WebSocket 控制通道网关）。

### 4.2 节点代理（独立 Go 仓库，不在本仓库）

- **技术**：Go；通过**厂商本地 REST API**（默认 `http://127.0.0.1:25325`）管理 profile，通过**厂商 `/browser/*` 动作**或 **CDP（chromedp）** 驱动页面。
- **职责**：登录平台、建立 WebSocket 长连、上报设备信息与厂商版本、接收命令、创建/启动/停止/删除 profile、注入代理、运行时注入账号、回收清理。
- **安全**：仅与 `127.0.0.1` 上的厂商 API 通信；账号密码**仅驻留内存**，注入后即弃。
- **交付**：交叉编译为 `windows/amd64` 与 `darwin/arm64`；签名自动更新。
- 本仓库仅定义**接口契约**（OpenAPI + 命令信封），不承载节点代理代码。

### 4.3 厂商浏览器（Undetectable）

- Chromium 内核；安装包由厂商提供（Windows / macOS arm64）。
- 指纹在浏览器内核层生效；配置与覆盖经本地 API 注入。
- **本地 API 无鉴权**：必须配置为仅监听 `127.0.0.1`，并加主机防火墙，严禁暴露到 LAN。
- 商业授权，需在用户机上安装并登录有效席位。

### 4.4 管理控制台（本仓库 `web/`）

- 新增 pages：环境管理、厂商配置目录、账号库、代理池、客户端/席位、会话监控、租户与成员/邀请；复用任务、审计、IAM 页面与双主题。

---

## 5. 领域模型与数据表（PostgreSQL）

> 现有表需增加 `tenant_id` 维度；迁移策略见 §11。

| 表 | 关键字段 | 说明 |
|----|----------|------|
| `tenants` | id, name, status, quota | 租户 |
| `users`（改造） | + tenant_id, principal_type | 统一主体，`principal_type ∈ {user, admin}` |
| `tenant_invitations` | id, tenant_id, code, role, expires_at, used_by, used_at | 邀请码/邀请链接 |
| `devices` | id, tenant_id, user_id, name, os, arch, agent_version, machine_id, vendor_version, core_version, seat_status, status, last_seen_at | 客户端记录 |
| `client_sessions` | id, user_id, device_id, access_token_hash, issued_at, expires_at, revoked_at | 客户端登录会话与令牌 |
| `vendor_configs` | id, tenant_id, config_id, browser, os, screen, useragent, webgl, synced_at | 厂商配置目录（从 `GET /configslist` 同步） |
| `environments` | id, tenant_id, name, target_os, config_id, overrides_json, proxy_policy, status, latest_revision_id | 环境定义 |
| `environment_revisions` | id, environment_id, revision_no, config_id, overrides_json, checksum, validated_at | 不可变快照 |
| `credentials` | id, tenant_id, site, username, secret_ciphertext, key_version, status, rotated_at | 凭据保险库 |
| `credential_bindings` | credential_id, environment_id | 账号↔环境绑定（独占） |
| `proxies` | id, tenant_id, protocol, host, port, auth_ciphertext, geo, provider, status, cooldown_until | 代理资源 |
| `proxy_health_checks` | proxy_id, checked_at, alive, latency_ms, ip, geo_actual, risk_tags | 体检记录 |
| `sessions` | id, tenant_id, device_id, user_id, environment_revision_id, proxy_id, credential_id, vendor_profile_id, state, issued_at, expires_at, released_at | 会话 |
| `device_commands` | id, device_id, user_id, session_id, type, payload_json, state, expires_at, issued_at, acked_at | 下发给节点的命令 |

`state` 枚举：session ∈ {requested, provisioning, active, releasing, released, expired, failed}；command ∈ {pending, delivered, acked, failed, expired}。

**关于 `overrides_json`**：可覆盖维度为 UA、CPU 核数、内存、分辨率（**仅 Windows 配置**）、语言、时区、地理、WebRTC。Canvas/WebGL/字体等由 `config_id` 决定，**不可单独覆盖**。

---

## 6. 控制通道与下发协议

### 6.1 连接方向（关键约束）

用户机在 NAT 之后，**连接必须由客户端出站发起**。已确认采用：

- **WebSocket 长连**：节点代理连接 `wss://.../api/v1/nodes/connect`，携带**用户访问令牌**；服务端在其上推送命令帧。心跳用于在线判定；断线后指数退避重连并补齐未确认命令。

不实现长轮询降级。命令帧保持 HTTP/JSON 语义。

### 6.2 命令信封

```json
{
  "id": "cmd_...",
  "type": "provision_session",
  "session_id": "ses_...",
  "issued_at": "2026-09-22T10:00:00Z",
  "expires_at": "2026-09-22T10:02:00Z",
  "nonce": "base64",
  "payload": { },
  "signature": "ed25519:..."
}
```

命令类型（首期）：

- `provision_session`：下发环境（configid + overrides + 代理）+ 短期凭据取件令牌；节点创建并启动 profile。
- `stop_session`：停止并删除临时 profile。
- `rotate_proxy`：会话内换出口 IP。
- `update_agent`：触发签名更新。
- `collect_logs` / `probe`：诊断。

### 6.3 安全属性

- 身份：节点代理以**用户账号**登录取得访问令牌，长连走 WSS。
- 指令完整性：命令签名 + `nonce` + 短 TTL + 服务端单调计数。
- 传输：全程 HTTPS/WSS；生产强制 `COOKIE_SECURE=true` 与受信代理 CIDR。

### 6.4 配置与凭据交付

- **配置下载**：`GET /api/v1/nodes/environments/{revisionID}/config`，返回 `config_id + overrides + proxy`；节点代理据此经厂商 API 创建 profile。
- **凭据交付（已确认）**：**短期令牌取件 + 运行时注入**。控制面签发一次性、短 TTL 令牌；节点凭令牌取回凭据，**仅驻留内存**，通过厂商 `/browser/fill`（或 CDP）实时填入登录表单，随后丢弃。**不写入厂商 profile 的 accounts 字段，不落盘。**
- 明文凭据**禁止**进入日志、遥测、审计 payload。

### 6.5 本地 API 安全（强制要求）

- 厂商 API 必须配置为**仅监听 `127.0.0.1`**，并加主机防火墙规则；严禁绑定 LAN IP。
- 节点代理是唯一调用方；控制面**从不**直接访问厂商 API（也访问不到）。
- 设备丢失或重装时，吊销用户访问令牌并清理残留 profile。

---

## 7. 用户账号与客户端认证

- **用户主体**：凭邀请注册的用户是控制台与客户端的统一身份（`principal_type=user`）。
- **邀请制注册**：管理员生成邀请码/邀请链接 → 用户注册时提交 → 校验有效后创建 `users` 行并加入指定租户；无邀请不可注册。
- **客户端认证**：节点代理用**同一用户账号**登录取得访问令牌，凭该令牌建立长连、下载配置、发起会话。
- **客户端记录**：登录时上报设备信息（OS、arch、agent 版本、厂商版本、内核版本、机器标识），控制面登记 `devices` 行并归属该用户；用于在线状态、席位可见性与遥测，非独立认证主体。
- **吊销**：禁用用户即吊销其客户端访问；用户可在控制台查看并注销自己的在线客户端。
- **两层账号概念**：用户对**平台**用自助注册账号；对**目标站点**由平台下发的站点账号自动登录，用户无感。二者无关。

---

## 8. 环境与指纹自洽

### 8.1 环境定义

一个环境 = `target_os + config_id（厂商配置）+ overrides_json + proxy_policy + 绑定账号`。

> **重要改写**：不再由控制面"生成指纹"。指纹数据由**厂商配置**提供，控制面只决定**用哪个配置、覆盖哪些维度**。

### 8.2 覆盖与自洽校验（发布前 checklist）

环境版本发布必须通过校验器，拒绝矛盾组合：

- `config_id` 必须存在于 `vendor_configs` 目录；
- `target_os` 与所选配置的 `os` 一致；
- **分辨率覆盖仅对 Windows 配置有效**，macOS 配置不得设置分辨率覆盖；
- **时区 / 语言 / 地理位置与代理出口地理一致**（由 `proxy_policy` 推导，写入 overrides）；
- UA 覆盖（若使用）不得与配置的 OS/平台冲突；
- 结果写入 `validated_at`，未通过不得发布。

### 8.3 版本与漂移控制（已确认）

- **锁定内核/配置版本，手动灰度升级**：在小范围机器先验证新内核/新配置对指纹表现的影响，通过后再铺开。
- 厂商自动更新可能改变指纹表现，因此需在节点代理屏蔽/延后厂商自动升级，并由平台记录每台机器的 `vendor_version`、`core_version`。
- 环境快照记录 `config_id` 与 overrides 的 `checksum`，保证可复现。

### 8.4 兼容性说明

- **Client Hints**：Chromium 内核原生支持，上一版 Firefox 分叉的 UA-CH 缺口**已消除**。
- **TLS/JA3**：由 Chromium 内核网络栈决定，仍无法任意定制；作为已知风险登记。
- **指纹自定义边界**：Canvas/WebGL 等无法单独设定，随厂商配置捆绑；如需特殊组合，只能挑选不同 `config_id`。

---

## 9. 会话生命周期

```text
requested → provisioning → active → releasing → released
                     └──────── failed / expired ────────┘
```

1. **用户发起**：用户在客户端下拉选择具体配置 → 经 WebSocket 长连到达控制面。**选择与确认的完整交互流程见 §9.1（部分待定）。**
2. **自动分配**：控制面**自动挑一个可用环境实例**（环境版本 + 其独占账号 + 可用出口 IP），在单事务内一并锁定；账号独占，被占用时该环境**不出现在可用列表**（见 §9.1）；分配时二次校验，被抢占则返回错误要求重选。
3. **下发**：创建 `session` 与 `provision_session` 命令 → 经 WebSocket 到达节点 → 节点经厂商 API **新建临时 profile**（configid + overrides + 代理）并启动。
4. **激活**：节点回执 + 心跳，会话转 `active`，代理与账号进入独占状态；节点凭短期令牌取回凭据并在内存中注入登录。
5. **释放**：用户关闭或 TTL 到期 → `stop_session` → 节点停止并**删除临时 profile**，代理归还并冷却，账号授权撤销。
6. **回收兜底**：Asynq 定时任务 `session.reap` 处理超时/失联会话；节点侧兜底清理残留 profile。

**独占性**：同一 `proxy` 与同一 `credential` 在同一时刻至多被一个活动会话占用。

### 9.1 环境选择与确认流程（部分定稿，其余待定）

> 状态：**第 3 项已定稿**，其余待定。任何"点下拉即自动完成"的说法都不成立。

1. **选择粒度与呈现（待定）**：下拉展示哪些字段、是否分组/搜索。
2. **是否二次确认（待定）**：选中即分配，还是先确认面板。取向 A 一键即用 / B 选择→确认→开始 / C 选择→预分配预览→确认。
   - 该取向决定 API 是否需要"预分配/确认"两接口，以及 `sessions` 是否需 `pending_confirmation` 态。
3. **不可用环境处理（已定稿）**：**不做置灰、不做排队**。服务端在可用环境列表中**直接过滤被占用项**，响应中不出现；前端只渲染返回值，空列表显示**"无可用环境"**（正常状态，非错误）。因过滤与分配存在时点差，**分配时仍须二次校验**。
4. **分配与就绪的反馈（待定）**：是否需要进度态与取消。
5. **失败与重试（待定）**：分配/下发/启动失败的呈现与重试。
6. **会话信息可见性（待定）**：环境名、代理地区、剩余时长、脱敏账号标识、结束入口。
7. **多会话并行（待定）**：是否允许同时多会话及配额约束。
8. **幂等与重连恢复（待定）**。

---

## 10. 代理池与质量管控

- **来源**：HTTP / SOCKS5（含认证）；住宅/移动优先，数据中心 IP 标注高风险。
- **体检 Worker（Asynq 周期任务）**：`proxy.healthcheck` 检测存活、延迟、真实出口 IP、属地真实性（与登记 geo 比对）、基础风控标签。
- **状态机**：`available → leased → cooldown → available`；连续失败进入 `quarantine`。
- **绑定**：出口 IP 与指纹地理强绑定；会话内 IP 不得变更（除非显式 `rotate_proxy`）。
- **注入**：由节点代理经厂商 API 的 profile `proxy` 字段或 `/proxies/add` 注入，格式如 `socks5://host:port:login:pass`。
- **泄露防护**：依赖厂商内核的 DNS/WebRTC 处理能力；环境校验中确认 WebRTC 覆盖与代理地理一致。

---

## 11. 多租户与现有 IAM 的衔接

- 现有 `users/roles/permissions` 增加 `tenant_id`；角色：平台管理员、租户管理员、用户、只读。
- 所有仓储查询强制带 `tenant_id` 过滤（领域服务层做租户边界，避免越权）。
- **邀请制注册**：落库时确定 `tenant_id`；邀请可限定角色。
- 迁移：新增列允许为空 → 回填默认租户 → 加非空约束与索引（分步 migration）。
- 前端路由与 API 携带租户上下文；RBAC 判定加入租户维度。

---

## 12. API 契约（`/api/v1` 扩展，仅设计）

**控制台/管理面**（JWT + RBAC）：

- `GET/POST /api/v1/environments`、`GET/PATCH /api/v1/environments/{id}`
- `POST /api/v1/environments/{id}/revisions`（发布，返回自洽校验结果）
- `GET /api/v1/vendor-configs`（厂商配置目录，管理员同步/查看）
- `GET/POST /api/v1/credentials`、`POST /api/v1/credentials/{id}/rotate`
- `GET/POST /api/v1/proxies`、`POST /api/v1/proxies/{id}/check`
- `GET /api/v1/devices`、`DELETE /api/v1/devices/{id}`
- `GET/POST /api/v1/sessions`、`POST /api/v1/sessions/{id}/release`
- `GET /api/v1/tenants`、`GET/POST /api/v1/tenants/{id}/members`、`POST /api/v1/tenants/{id}/invitations`
- `POST /api/v1/auth/register`（凭邀请码注册并加入指定租户）

**节点面**（用户账号令牌认证）：

- `WS /api/v1/nodes/connect`（命令通道，携带用户访问令牌）
- `GET /api/v1/nodes/environments/available`（该用户可见且**当前可分配**的具体配置；被占用项由服务端过滤；空 = 无可用环境）
- `GET /api/v1/nodes/environments/{revisionID}/config`
- `POST /api/v1/nodes/sessions`（用户发起会话，服务端自动分配）
- `POST /api/v1/nodes/credentials/{sessionID}/claim`（短期令牌取件）
- `POST /api/v1/nodes/heartbeat`、`POST /api/v1/nodes/commands/{id}/ack`

所有新增端点需同步进 `api/openapi.yaml` 并补齐契约测试。

---

## 13. 控制台信息架构

| 页面 | 关键交互 |
|------|----------|
| 注册与个人中心 | **凭邀请码注册**、登录、下拉选具体配置发起会话（**只列可用环境，无可用即空状态**）、查看/注销自己的在线客户端 |
| 环境管理 | 环境列表、选 `config_id` + 覆盖、版本历史、**发布前自洽校验 checklist** |
| 厂商配置目录 | 从 `/configslist` 同步的配置清单（OS/屏幕/UA/WebGL 概览），供环境引用 |
| 账号库 | 凭据列表（默认遮罩）、按站点/标签筛选、轮换、绑定环境 |
| 代理池 | 资源列表、体检结果与风险标签、状态/冷却、批量导入 |
| 客户端/席位 | 在线状态、OS/agent/厂商/内核版本、所属用户、席位授权状态、命令历史 |
| 会话监控 | 活动/历史会话、环境×代理×账号×客户端的绑定关系、一键释放 |
| 租户与成员 | 租户、配额、角色分配、邀请码管理 |
| 任务 / 审计 | 复用现有页面；新增任务类型（体检/轮换/回收/配置同步） |

复用现有双主题与组件状态契约，满足 WCAG 2.1 AA。

---

## 14. 安全模型

- **身份**：控制台用户 JWT；节点代理以同一用户账号登录取得短期访问令牌，长连走 WSS。
- **指令完整性**：命令签名 + 短 TTL + nonce。
- **凭据保护**：信封加密（AES-256-GCM，AAD 绑定 tenant/environment/session），主密钥抽象为 `kms` 适配器；**短期取件 + 运行时注入 + 内存驻留 + 用后即焚**；不写入厂商 profile。
- **本地 API 加固**：厂商 API 仅绑 `127.0.0.1` + 主机防火墙；节点代理为唯一调用方。
- **审计**：复用 `audit` 模块，覆盖环境发布、凭据取用、代理分配、会话下发/释放、客户端登录/注销、席位变更；**审计不含凭据明文**。
- **供应链与合规**：厂商为客户闭源商业软件，席位授权合规由公司统一采购保证；节点代理产物签名校验。

---

## 15. 交付与部署

- **节点代理**：Go 交叉编译 `windows/amd64`、`darwin/arm64`；打包安装器；签名自动更新。
- **厂商浏览器**：由厂商提供 Windows / macOS arm64 安装包；公司统一采购席位并分配到机器；安装与激活纳入客户端引导流程。
- **控制面**：沿用现有 Dockerfile / Compose 与迁移流程；新增表走标准 migration。
- **不再需要**：Linux 构建机、`multibuild.py` 交叉编译、Python 运行时打包。
- **联调**：需至少一台真实 Windows 与一台 macOS arm64 机器（WSL 无法运行厂商浏览器；控制面仍可在 WSL 开发）。

---

## 16. 与现有框架的映射

| 能力 | 复用 | 新增 | 改造 |
|------|------|------|------|
| 认证/RBAC | iam | 租户维度、客户端登录会话、邀请注册 | users/roles 加 tenant_id |
| 审计 | audit | 新事件类型 | — |
| 任务/调度 | task + Asynq | 代理体检、凭据轮换、会话回收、配置同步 handler | — |
| 可观测 | observability + Prometheus | 会话/代理/客户端/席位指标 | — |
| HTTP | httpserver + Chi | 环境/配置/凭据/代理/客户端/会话路由 | 租户中间件 |
| 数据 | pgx/sqlc | §5 各表 + 迁移 | 既有表加 tenant_id |
| 前端 | web/ 壳、双主题、IAM/任务/审计页 | 环境/配置目录/账号/代理/客户端席位/会话页 | 租户上下文 |

---

## 17. 分阶段路线图（设计视角）

- **P0 链路 PoC**：在单台 Windows/Mac 上验证「控制面下发 configid+overrides+proxy → 节点经本地 API 建临时 profile → 启动 → 注入账号 → 经代理打开站点」，确认指纹与地理自洽。
- **P1 多租户与用户准入**：租户模型、邀请制注册、客户端登录与记录、环境库与厂商配置同步、发布校验、单客户端下发。
- **P2 资源与生命周期**：代理池 + 体检、凭据保险库、会话原子分配与回收，**并定稿 §9.1 交互流程**（API、状态机、客户端交互）。
- **P3 控制台与治理**：完整管理台、席位列管、审计/指标、配额与权限细化。
- **P4 交付硬化**：节点代理双端签名更新、厂商版本锁定与灰度机制、TLS/JA3 与指纹漂移专项验证。

---

## 18. 风险与开放问题

| 风险 | 影响 | 应对 |
|------|------|------|
| 厂商锁定与闭源 | 受制于单一供应商与产品路线 | 环境模型与节点代理做隔离层，便于未来替换引擎 |
| 按席位持续付费 | 成本随用户数增长 | 公司统一采购、席位可见与回收 |
| **本地 API 无鉴权** | 同机/LAN 可起停 profile、读 cookie | 强制仅绑 127.0.0.1 + 主机防火墙；节点代理唯一调用方 |
| 内核/配置漂移 | 指纹表现随升级变化 | 锁定版本 + 手动灰度 + 记录 core_versions |
| 指纹自定义受限 | Canvas/WebGL 不可单独设 | 以选配 `config_id` 替代；环境校验保证自洽 |
| 临时 profile 每次重登 | 登录耗时、可能触发风控 | 评估引入 cookie 预热（仅 cookie，不含密码）；或按环境放宽为持久 profile |
| Chromium 内核 TLS/JA3 | 网络层指纹仍不可控 | 登记为已知风险，评估代理侧 TLS 方案 |
| 多租户改造 IAM 表 | 迁移与越权风险 | 分步迁移 + 领域层强制租户过滤 + 越权测试 |
| 凭据泄露面 | 高敏感 | 信封加密、短期取件、运行时注入内存驻留、审计脱敏 |

**待定事项（TBD）**：

1. **§9.1 环境选择与确认流程**：第 3 项已定稿；仍待定——是否二次确认、分配反馈、失败/取消呈现，以及 `sessions` 是否需要 `pending_confirmation`。
2. **多会话并行**的配额与独占约束（§9.1 第 7 项）。
3. 是否需要**cookie 预热**以缓解临时 profile 的重复登录（仅 cookie，不含密码）。

**已确认决策（2026-09-22）**：

1. 指纹引擎：**Undetectable**（Chromium 内核），**全量替换 Camoufox**。
2. 控制通道：**WebSocket 长连**（客户端出站），不做长轮询。
3. 节点代理：**独立 Go 仓库**，厂商本地 API + 优先内置动作、必要时 CDP（chromedp）；不用 Python。
4. 厂商边界：**仅用本地执行器**，不用其云端 profile / 团队 / 用户功能；平台为唯一真值源。
5. 席位：**公司统一采购，一人一席**。
6. 环境/指纹：**厂商配置 configid + 有限覆盖**；控制面不再自研生成指纹。
7. 代理：**平台代理池，逐 profile 注入**（HTTP/SOCKS5 + 认证）。
8. Profile 生命周期：**每会话临时新建**，用完删除。
9. 账号注入：**运行时注入、不落盘**，密钥仅驻留节点内存。
10. 自动化通道：**优先厂商 `/browser/*` 动作，能力不足再回退 CDP**。
11. 版本策略：**锁定内核/配置版本，手动灰度升级**。
12. 目标平台：**Windows + macOS arm64**。
13. 用户准入：**邀请制注册**，客户端以同一用户账号认证。
14. 身份模型：**用户账号是唯一身份**；客户端只是机器记录。
15. 环境分配：**服务端自动分配可用环境**；用户下拉选**具体配置**。
16. 不可用环境：**服务端过滤，不做置灰/排队**，空列表显示"无可用环境"。
17. 账号占用：**独占**。
18. 会话触发：**用户手动发起**。
19. 开放 API：**本期不做**。
20. 本地 API 安全：**仅绑 127.0.0.1 + 防火墙**。

---

## 19. 变更历史

- **2026-09-22**：初版基于 **Camoufox**（Firefox 分叉，Python/Playwright，需 Linux 交叉编译）。
- **2026-09-22**：因期望"不改浏览器源码、用扩展/脚本/API 配置"并规避 Python 与交叉编译，**改用 Undetectable**（Chromium 内核、本地 REST API + CDP、厂商直供双端安装包）。Camoufox 方案作废，保留于 `2026-09-22-camoufox-browser-environment-platform.md`。

---

## 20. 明确不做

- 不在本仓库引入浏览器自动化运行时或节点代理代码（独立 Go 仓库）。
- 不使用厂商云端 profile、团队与用户功能。
- 不在本期实现浏览器实时投屏/人工接管（前端仅管理控制台）。
- 不做开放租户 API。
- 不做数据面流量经控制面中转。

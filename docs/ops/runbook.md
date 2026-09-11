# Runbook

## 1. Start Services

先确认 MySQL 开发库已创建、Redis 可用。GameServer 保留 AutoMigrate，只创建或调整表，不创建数据库。两个进程从仓库根目录启动：

终端 1：启动 GameServer，注册 node-a。

```bash
redis-cli -h 127.0.0.1 -p 6379 ping
export GAME_DB_DSN='game:password@tcp(127.0.0.1:3306)/game_demo?charset=utf8mb4&parseTime=True&loc=Local'
export GAME_TICKET_SECRET='local-dev-ticket-secret'
GAME_CONFIG=configs/gameserver.local.yaml go run ./cmd/gameserver
```

终端 2：用同一密钥启动 LoginServer，无需 GAME_DB_DSN。

```bash
export GAME_TICKET_SECRET='local-dev-ticket-secret'
GAME_CONFIG=configs/loginserver.local.yaml go run ./cmd/loginserver
```

| 进程 | 端口与路由 |
|---|---|
| LoginServer | 8080：POST /api/login、GET /healthz |
| GameServer A | 8081：/ws；8082：/healthz、/metricsz、/admin/* |
| GameServer B（扩容/验收） | 8091：/ws；8092：管理 HTTP |

- 六份正式配置为 gameserver.local/staging/prod.yaml 和 loginserver.local/staging/prod.yaml；省略 GAME_CONFIG 时读取对应 local 文件，无旧文件名 fallback。
- 两个进程必须共享 ticket issuer、算法、密钥，以及同一 Redis 实例、DB、节点和玩家归属 key 前缀。
- staging/prod 必须设置 GAME_ADMIN_TOKEN，管理路由和指标需要 Bearer Token；LoginServer 不读取管理 Token。
- 本地 ws.allowed_origins: ["*"] 只用于开发；staging/prod Web 客户端应配置准确 Origin。
- 正式环境须覆盖 Redis 地址和 advertised_ws_addr。新增 GameServer 使用独立配置、唯一 node_id、WS/Admin 端口和 advertised_ws_addr，共享业务数据库与 Redis。LoginServer 自动感知节点，无需重启。
- Ctrl-C 或 SIGTERM 分别停止对应进程。LoginServer 先停 HTTP，再关闭 Redis；已进入 GameServer 的玩家仍可请求。GameServer 停止时注销节点并关闭 WS，维护时先执行第 6 节 drain。
- LoginServer /healthz 只表示 HTTP 进程存活；无可用节点或 Redis 故障时登录返回 HTTP 500、code=1，不签发票据。

## 2. Baseline Smoke

- `curl http://127.0.0.1:8080/healthz`
- `curl http://127.0.0.1:8082/healthz`
- `curl http://127.0.0.1:8082/metricsz -H "Authorization: Bearer ${GAME_ADMIN_TOKEN}"`
- `curl -X POST http://127.0.0.1:8080/api/login -H 'Content-Type: application/json' -d '{"account":"u1001","password":"x","client_ip":"127.0.0.1","client_ver":"1.0.0"}'`
- `go run ./scripts/loadtest/ws_auth_smoke.go`
- `go run ./scripts/loadtest/ws_biz_smoke`
- `go run ./scripts/loadtest/ws_reconnect_smoke`
- `go run ./scripts/loadtest/ws_prototype_smoke`
- `curl -X POST http://127.0.0.1:8082/api/login` 应返回 404。

### 2.1 Split-process Acceptance

创建独立测试数据库，安装 redis-server，确保 8080/8081/8082/8091/8092 空闲：

```bash
LOGIN_SPLIT_TEST_DB_DSN='game_test:password@tcp(127.0.0.1:3306)/game_test?charset=utf8mb4&parseTime=True&loc=Local' \
  go test ./scripts/loadtest/loginserver_split -v -count=1
```

单节点使用 `-run '^TestSingleNode$'`，双节点使用 `-run '^TestMultiNode$'`。未设置 LOGIN_SPLIT_TEST_DB_DSN 时默认跳过。测试会写所选数据库，自动创建和清理临时 Redis、服务进程及配置，不停止已有开发 Redis 或服务；测试库中的玩家数据保留。

覆盖登录到 auth_ack、主链路 smoke、管理路由、默认监控、错误密钥、五个 claims 字段篡改、重放、错服票据、双节点分配和原服优先、drain/满载避让、正常注销、异常退出 TTL、Redis 故障以及停 LoginServer 后在线 WS 可用。双节点测试用 1 秒心跳、3 秒 TTL 加速验收，本地正式示例仍为 5/15 秒；drain 和连接数断言等待 Redis 心跳更新。

容量验收分别占用每节点 2000 个真实 WS 连接槽，确认第 2001 个连接返回 SERVER_FULL。这是短时准入测试，不替代下文正式时长的 2000 在线性能压测。

## 3. P5 Load Test (k6)

### 3.1 Prerequisites
- k6 installed and available in PATH
- Local API endpoint reachable: `http://127.0.0.1:8080`
- WS endpoint reachable: `ws://127.0.0.1:8081/ws`

默认正式阶段以本表和 `scripts/loadtest/k6_2k_online.js` 为唯一口径：

| Scenario | Target | Ramp Up | Hold | Ramp Down | Total |
|---|---:|---:|---:|---:|---:|
| S1 | 200 | 2m | 8m | 1m | 11m |
| S2 | 2000 | 5m | 30m | 2m | 37m |
| S3 | 2200 | 10m | 1m | 1m | 12m |

任务文档中提到的“正式时长”均指以上完整 stages，不再单独维护一组简写时间。

### 3.2 Prepare Output Directory
- `mkdir -p reports`

### 3.3 Run S1 (200 online smoke)
- `k6 run --quiet --log-output=none scripts/loadtest/k6_2k_online.js -e SCENARIO=S1 -e API_BASE=http://127.0.0.1:8080 --summary-export=reports/k6_s1_summary.json`

### 3.4 Run S2 (2000 online steady)
- `k6 run --quiet --log-output=none scripts/loadtest/k6_2k_online.js -e SCENARIO=S2 -e API_BASE=http://127.0.0.1:8080 --summary-export=reports/k6_s2_summary.json`

### 3.5 Run S3 (0->2200 burst / full check)
- `k6 run --quiet --log-output=none scripts/loadtest/k6_2k_online.js -e SCENARIO=S3 -e API_BASE=http://127.0.0.1:8080 --summary-export=reports/k6_s3_summary.json`

### 3.6 Generate Unified Result
- `go run ./scripts/loadtest/k6_report -s1 reports/k6_s1_summary.json -s2 reports/k6_s2_summary.json -s3 reports/k6_s3_summary.json -out reports/k6_gate_report.md`

## 4. Key Metrics to Verify

### 4.1 k6 metrics
- `login_ok_rate`
- `ws_connect_ok_rate`
- `ws_auth_ok_rate`
- `ws_biz_ack_ok_rate`
- `ws_biz_rtt_ms` (P95/P99)
- `ws_server_full_events` (S3 must be `> 0`)

### 4.2 Server metrics (`/metricsz`)
- `ws_connections`
- `ws_auth_success`
- `ws_auth_failed`
- `ws_rate_limited`
- `ws_queue_kick`
- `ws_biz_requests`
- `ws_biz_duration_p95_ms` / `ws_biz_duration_p99_ms`
- `db_requests` / `db_duration_p95_ms` / `db_duration_p99_ms`
- `redis_requests` / `redis_duration_p95_ms` / `redis_duration_p99_ms`

### 4.3 Minimal dashboard and alert check

持续查看本地节点：

```bash
go run ./scripts/monitoring/metrics_dashboard
```

staging/prod 使用管理 Token：

```bash
go run ./scripts/monitoring/metrics_dashboard \
  -url http://127.0.0.1:8082/metricsz \
  -token "${GAME_ADMIN_TOKEN}"
```

单次检查使用 `-once`。无告警退出码为 `0`，接口访问失败为 `1`，命中告警规则为 `2`，可直接交给 cron、发布脚本或部署平台判断。首次采样和单次检查使用进程启动以来的累计计数；持续模式从第二次采样开始使用相邻采样增量。

默认告警规则：

| Level | Rule |
|---|---|
| WARN | 连接数达到 `max_connections` 的 `90%` |
| CRITICAL | 连接数达到 `max_connections` |
| WARN | 业务处理 P95 `>= 50ms` |
| CRITICAL | 业务处理 P99 `>= 120ms` |
| WARN | DB P95 `>= 20ms` |
| WARN | Redis P95 `>= 5ms` |
| WARN | 相邻采样期间认证失败率 `>= 0.1%` |
| WARN | 相邻采样期间出现发送队列满踢人 |

若正式配置修改了连接数，必须通过 `-max-connections` 传入相同值。延迟阈值也可以通过对应命令行参数覆盖。

## 5. Acceptance Gate (S1~S3)
- S1/S2:
- `login_ok_rate >= 99.9%`
- `ws_connect_ok_rate >= 99.5%`
- `ws_auth_ok_rate >= 99.9%`
- `ws_biz_ack_ok_rate >= 99.0%`
- `ws_biz_rtt_ms p95 < 50ms`
- `ws_biz_rtt_ms p99 < 120ms`
- `ws_server_full_events == 0`

- S3:
- `ws_connect_ok_rate >= 90%`
- `ws_auth_ok_rate >= 90%`
- `ws_server_full_events > 0`
- service process keeps running (no crash)

## 6. Drain
- staging/prod 的 `/admin/*` 和 `/metricsz` 请求都必须携带 `Authorization: Bearer ${GAME_ADMIN_TOKEN}`；本地配置关闭校验时该请求头可省略。
- Enable drain mode at runtime:
- `curl -X POST http://127.0.0.1:8082/admin/drain -H "Authorization: Bearer ${GAME_ADMIN_TOKEN}" -H 'Content-Type: application/json' -d '{"enabled":true}'`
- Check drain state:
- `curl http://127.0.0.1:8082/admin/drain -H "Authorization: Bearer ${GAME_ADMIN_TOKEN}"`
- Check active sessions:
- `curl http://127.0.0.1:8082/admin/sessions -H "Authorization: Bearer ${GAME_ADMIN_TOKEN}"`
- During drain:
- new WS connections/auth should receive `SERVER_FULL`
- existing sessions continue until client disconnect or server stop
- wait until `active_sessions` approaches `0`, then stop process
- Stop process; authoritative player data has already been committed by business transactions

## 7. Rollback
- Restore the binary and matching process config for the affected LoginServer or GameServer
- Keep ticket and Redis shared settings consistent across both processes
- Restart and verify:
- `curl http://127.0.0.1:8082/healthz`
- `curl http://127.0.0.1:8082/metricsz -H "Authorization: Bearer ${GAME_ADMIN_TOKEN}"`

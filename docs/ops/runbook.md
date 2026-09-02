# Runbook

## 1. Start Service
- 先确认 MySQL 数据库已创建，并设置 `GAME_DB_DSN`。应用启动时执行当前 MVP 表的 `AutoMigrate`，不会创建数据库本身。
- 先确认共享 Redis 可用：`redis-cli -h 127.0.0.1 -p 6379 ping`
- `GAME_CONFIG=configs/config.local.yaml GAME_DB_DSN='game:password@tcp(127.0.0.1:3306)/game_demo?charset=utf8mb4&parseTime=True&loc=Local' GAME_TICKET_SECRET=local-dev-ticket-secret go run ./cmd/gameserver`
- 本地配置只启动 `node-a`，但运行时仍会注册到 Redis，LoginService 不使用静态单节点列表。
- 本地配置允许不校验管理 Token；staging/prod 启动前必须设置 `GAME_ADMIN_TOKEN`，否则服务拒绝启动。
- 本地 `ws.allowed_origins: ["*"]` 只用于开发；staging/prod 接入 Web 客户端前必须配置准确的 `https://域名`，原生客户端无 `Origin` 不受此项影响。
- 该启动方式只用于单节点 Demo。增加第二个 GameServer 前先拆出独立单实例 LoginServer；之后每个纯 GameServer 使用唯一的 `server.node_id` 和客户端可访问的 `server.advertised_ws_addr`，并连接同一个 Redis。
- 正式环境中的 Redis 地址和 `advertised_ws_addr` 必须由部署配置覆盖，不能沿用仓库内的本地地址。
- `GAME_DB_DSN` 必须包含 `charset=utf8mb4&parseTime=True&loc=Local`；账号密码只放部署环境变量或密钥系统，不写入仓库配置。

## 2. Baseline Smoke
- `curl http://127.0.0.1:8080/healthz`
- `curl http://127.0.0.1:8080/metricsz -H "Authorization: Bearer ${GAME_ADMIN_TOKEN}"`
- `curl -X POST http://127.0.0.1:8080/api/login -H 'Content-Type: application/json' -d '{"account":"u1001","password":"x","client_ip":"127.0.0.1","client_ver":"1.0.0"}'`
- `go run ./scripts/loadtest/ws_auth_smoke.go`
- `go run ./scripts/loadtest/ws_biz_smoke`
- `go run ./scripts/loadtest/ws_reconnect_smoke`

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
  -url http://127.0.0.1:8080/metricsz \
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
- `curl -X POST http://127.0.0.1:8080/admin/drain -H "Authorization: Bearer ${GAME_ADMIN_TOKEN}" -H 'Content-Type: application/json' -d '{"enabled":true}'`
- Check drain state:
- `curl http://127.0.0.1:8080/admin/drain -H "Authorization: Bearer ${GAME_ADMIN_TOKEN}"`
- Check active sessions:
- `curl http://127.0.0.1:8080/admin/sessions -H "Authorization: Bearer ${GAME_ADMIN_TOKEN}"`
- During drain:
- new WS connections/auth should receive `SERVER_FULL`
- existing sessions continue until client disconnect or server stop
- wait until `active_sessions` approaches `0`, then stop process
- Stop process; authoritative player data has already been committed by business transactions

## 7. Rollback
- Restore previous binary
- Restore previous config
- Restart and verify:
- `curl http://127.0.0.1:8080/healthz`
- `curl http://127.0.0.1:8080/metricsz -H "Authorization: Bearer ${GAME_ADMIN_TOKEN}"`

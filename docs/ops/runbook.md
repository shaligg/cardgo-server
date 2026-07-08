# Runbook

## 1. Start Service
- `GAME_CONFIG=configs/config.local.yaml go run ./cmd/gameserver`

## 2. Baseline Smoke
- `curl http://127.0.0.1:8080/healthz`
- `curl http://127.0.0.1:8080/metricsz`
- `curl -X POST http://127.0.0.1:8080/api/login -H 'Content-Type: application/json' -d '{"account":"u1001","password":"x","client_ip":"127.0.0.1","client_ver":"1.0.0"}'`
- `go run ./scripts/loadtest/ws_auth_smoke.go`
- `go run ./scripts/loadtest/ws_biz_smoke`
- `go run ./scripts/loadtest/ws_reconnect_smoke`

## 3. P5 Load Test (k6)

### 3.1 Prerequisites
- k6 installed and available in PATH
- Local API endpoint reachable: `http://127.0.0.1:8080`
- WS endpoint reachable: `ws://127.0.0.1:8081/ws`

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
- `ws_queue_drop`
- `ws_queue_kick`
- `flush_enqueued`
- `flush_queue_len`
- `flush_saved`

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
- Enable drain mode at runtime:
- `curl -X POST http://127.0.0.1:8080/admin/drain -H 'Content-Type: application/json' -d '{"enabled":true}'`
- Check drain state:
- `curl http://127.0.0.1:8080/admin/drain`
- Check active sessions:
- `curl http://127.0.0.1:8080/admin/sessions`
- During drain:
- new WS connections/auth should receive `SERVER_FULL`
- existing sessions continue until client disconnect or server stop
- wait until `active_sessions` approaches `0`, then stop process
- Stop process after final flush

## 7. Rollback
- Restore previous binary
- Restore previous config
- Restart and verify:
- `curl http://127.0.0.1:8080/healthz`
- `curl http://127.0.0.1:8080/metricsz`

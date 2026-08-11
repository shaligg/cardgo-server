# Load Test Report (Short Run)

## 1. Metadata
- Date: 2026-03-24
- Mode: short-duration validation (not the full default stages defined in `docs/ops/runbook.md`)
- Server: `GAME_CONFIG=configs/config.local.yaml go run ./cmd/gameserver`
- Script: `scripts/loadtest/k6_2k_online.js`
- Summary parser: `scripts/loadtest/k6_report`

## 2. Commands
- S1:
- `k6 run --quiet --log-output=none scripts/loadtest/k6_2k_online.js -e SCENARIO=S1 -e API_BASE=http://127.0.0.1:8080 -e S1_RAMP_UP=20s -e S1_HOLD=40s -e S1_RAMP_DOWN=10s --summary-export=reports/k6_s1_summary.json`
- S2:
- `k6 run --quiet --log-output=none scripts/loadtest/k6_2k_online.js -e SCENARIO=S2 -e API_BASE=http://127.0.0.1:8080 -e S2_RAMP_UP=40s -e S2_HOLD=50s -e S2_RAMP_DOWN=15s --summary-export=reports/k6_s2_summary.json`
- S3:
- `k6 run --quiet --log-output=none scripts/loadtest/k6_2k_online.js -e SCENARIO=S3 -e API_BASE=http://127.0.0.1:8080 -e S3_RAMP_UP=40s -e S3_HOLD=20s -e S3_RAMP_DOWN=10s --summary-export=reports/k6_s3_summary.json`

## 3. k6 Gate Summary

| Scenario | login_ok_rate | ws_connect_ok_rate | ws_auth_ok_rate | ws_biz_ack_ok_rate | ws_biz_rtt_p95(ms) | ws_biz_rtt_p99(ms) | ws_server_full_events | Result |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| S1 | 100.00% | 100.00% | 100.00% | 100.00% | 3 | 5 | 0 | PASS |
| S2 | 100.00% | 100.00% | 100.00% | 99.48% | 4 | 7 | 0 | PASS |
| S3 | 47.33% | 8.19% | 8.18% | 84.23% | 13527.35 | 18329.14 | 13882 | FAIL |

Gate failure details:
- S3: `ws_connect_ok_rate < 90%`, `ws_auth_ok_rate < 90%`

## 4. Server Metrics Delta (`/metricsz`)
- Before:
```json
{"ws_connections":0,"ws_auth_success":6463,"ws_auth_failed":6,"ws_rate_limited":14854,"ws_queue_drop":0,"ws_queue_kick":0,"flush_enqueued":6463,"flush_queue_len":0,"flush_processed":6463,"flush_saved":6462,"flush_failed":0,"flush_retried":0,"flush_dropped":0}
```
- After:
```json
{"ws_connections":0,"ws_auth_success":12835,"ws_auth_failed":9,"ws_rate_limited":28314,"ws_queue_drop":0,"ws_queue_kick":0,"flush_enqueued":12835,"flush_queue_len":0,"flush_processed":12835,"flush_saved":12834,"flush_failed":0,"flush_retried":0,"flush_dropped":0}
```

## 5. Bottlenecks Observed
1. S3 connect/auth collapse
- Symptom: connect/auth rate around 8% while `server_full_events` is high.
- Inference: local client host became the bottleneck first (ephemeral port / local socket resources), not pure server-side capacity boundary.

## 6. Next Actions
1. Re-run official full-duration acceptance on dedicated load generator host (separate from game server host).
2. For S3 validity, run with OS socket tuning (`ulimit`, ephemeral port range, TIME_WAIT reuse) before interpreting connect/auth rates as server limits.

## 7. Follow-up Fix Verification
- Script fix: `BIZ_STOP_BEFORE_CLOSE_MS` default changed to `15000` in `k6_2k_online.js`.
- Recheck result (same S1 stages): `ws_biz_ack_ok_rate=100.00%` (gate passed).
- Conclusion: previous S1 gate failure was primarily caused by near-close sampling bias in load script.

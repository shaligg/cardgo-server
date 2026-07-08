# k6 Summary Report

- Generated at: 2026-03-24T15:16:57+08:00

## Scenario Metrics

| Scenario | login_ok_rate | ws_connect_ok_rate | ws_auth_ok_rate | ws_biz_ack_ok_rate | ws_biz_rtt_p95(ms) | ws_biz_rtt_p99(ms) | ws_server_full_events |
|---|---:|---:|---:|---:|---:|---:|---:|
| S1 | 100.00% | 100.00% | 100.00% | 100.00% | 3 | 5 | 0 |
| S2 | 100.00% | 100.00% | 100.00% | 99.48% | 4 | 7 | 0 |
| S3 | 47.33% | 8.19% | 8.18% | 84.23% | 13527.35 | 18329.14 | 13882 |

## Gate Decision

| Scenario | Result | Failed Gates |
|---|---|---|
| S1 | PASS | - |
| S2 | PASS | - |
| S3 | FAIL | ws_connect_ok_rate<90%; ws_auth_ok_rate<90% |

## Thresholds

- S1/S2: login>=99.9%, ws_connect>=99.5%, ws_auth>=99.9%, ws_biz_ack>=99.0%, ws_biz_rtt_p95<50ms, ws_biz_rtt_p99<120ms, ws_server_full=0
- S3: ws_connect>=90%, ws_auth>=90%, ws_server_full>0

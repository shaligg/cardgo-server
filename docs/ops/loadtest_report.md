# Load Test Report (S1~S3)

> 可选自动汇总：`go run ./scripts/loadtest/k6_report -s1 reports/k6_s1_summary.json -s2 reports/k6_s2_summary.json -s3 reports/k6_s3_summary.json -out reports/k6_gate_report.md`

## 1. Test Metadata
- Date:
- Commit/branch:
- Tester:
- Environment:
- Game server config:
- DB/Redis config:

## 2. Workload Inputs

### S1
- Command:
- VU target:
- Ramp/Hold:
- Biz interval:
- Summary file:

### S2
- Command:
- VU target:
- Ramp/Hold:
- Biz interval:
- Summary file:

### S3
- Command:
- VU target:
- Ramp/Hold:
- Biz interval:
- Summary file:

## 3. k6 Results

| Scenario | login_ok_rate | ws_connect_ok_rate | ws_auth_ok_rate | ws_biz_ack_ok_rate | ws_biz_rtt_p95 | ws_biz_rtt_p99 | ws_server_full_events |
|---|---:|---:|---:|---:|---:|---:|---:|
| S1 |  |  |  |  |  |  |  |
| S2 |  |  |  |  |  |  |  |
| S3 |  |  |  |  |  |  |  |

## 4. Server Metrics Snapshot (`/metricsz`)

### Before
```json
{}
```

### After S1
```json
{}
```

### After S2
```json
{}
```

### After S3
```json
{}
```

## 5. Acceptance Decision

| Gate | S1 | S2 | S3 |
|---|---|---|---|
| login_ok_rate >= 99.9% |  |  | N/A |
| ws_connect_ok_rate >= 99.5% |  |  | N/A |
| ws_auth_ok_rate >= 99.9% |  |  | N/A |
| ws_biz_ack_ok_rate >= 99.0% |  |  | N/A |
| ws_biz_rtt_p95 < 50ms |  |  | N/A |
| ws_biz_rtt_p99 < 120ms |  |  | N/A |
| ws_server_full_events == 0 (S1/S2) |  |  | N/A |
| ws_connect_ok_rate >= 90% (S3) | N/A | N/A |  |
| ws_auth_ok_rate >= 90% (S3) | N/A | N/A |  |
| ws_server_full_events > 0 (S3) | N/A | N/A |  |
| process crash / OOM |  |  |  |

## 6. Bottleneck Analysis
- CPU hotspots:
- DB slow query:
- Redis latency spikes:
- Queue overflow/rate limit:

## 7. Actions
1. Immediate fixes:
2. Next iteration targets:
3. Retest plan:

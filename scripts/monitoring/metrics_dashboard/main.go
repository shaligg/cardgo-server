package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bigfish/go_orm_1/internal/infra/metrics"
)

type thresholds struct {
	maxConnections int64
	bizP95MS       int64
	bizP99MS       int64
	dbP95MS        int64
	redisP95MS     int64
}

type alert struct {
	level   string
	message string
}

func main() {
	limits := defaultThresholds()
	var (
		metricsURL = flag.String("url", "http://127.0.0.1:8080/metricsz", "metrics endpoint")
		token      = flag.String("token", os.Getenv("GAME_ADMIN_TOKEN"), "admin bearer token")
		interval   = flag.Duration("interval", 5*time.Second, "refresh interval")
		once       = flag.Bool("once", false, "fetch once and exit")
	)
	flag.Int64Var(&limits.maxConnections, "max-connections", limits.maxConnections, "configured connection limit")
	flag.Int64Var(&limits.bizP95MS, "biz-p95-ms", limits.bizP95MS, "business P95 warning threshold")
	flag.Int64Var(&limits.bizP99MS, "biz-p99-ms", limits.bizP99MS, "business P99 critical threshold")
	flag.Int64Var(&limits.dbP95MS, "db-p95-ms", limits.dbP95MS, "database P95 warning threshold")
	flag.Int64Var(&limits.redisP95MS, "redis-p95-ms", limits.redisP95MS, "Redis P95 warning threshold")
	flag.Parse()

	if *interval <= 0 {
		fmt.Fprintln(os.Stderr, "interval must be positive")
		os.Exit(1)
	}

	client := &http.Client{Timeout: 3 * time.Second}
	var previous *metrics.Snapshot
	for {
		current, err := fetchMetrics(client, *metricsURL, *token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s CRITICAL fetch metrics: %v\n", time.Now().Format(time.RFC3339), err)
			if *once {
				os.Exit(1)
			}
			time.Sleep(*interval)
			continue
		}

		alerts := evaluate(current, previous, limits)
		printDashboard(current, previous, limits, alerts)
		if *once {
			if len(alerts) > 0 {
				os.Exit(2)
			}
			return
		}
		previous = &current
		time.Sleep(*interval)
	}
}

func defaultThresholds() thresholds {
	return thresholds{
		maxConnections: 2000,
		bizP95MS:       50,
		bizP99MS:       120,
		dbP95MS:        20,
		redisP95MS:     5,
	}
}

func fetchMetrics(client *http.Client, url string, token string) (metrics.Snapshot, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return metrics.Snapshot{}, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return metrics.Snapshot{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return metrics.Snapshot{}, fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}

	var snapshot metrics.Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return metrics.Snapshot{}, fmt.Errorf("decode metrics: %w", err)
	}
	return snapshot, nil
}

func evaluate(current metrics.Snapshot, previous *metrics.Snapshot, limits thresholds) []alert {
	alerts := make([]alert, 0)
	if limits.maxConnections > 0 {
		usage := float64(current.WSConnections) / float64(limits.maxConnections)
		if usage >= 1 {
			alerts = append(alerts, alert{"CRITICAL", "connections reached configured limit"})
		} else if usage >= 0.9 {
			alerts = append(alerts, alert{"WARN", "connections reached 90% of configured limit"})
		}
	}
	if current.WSBizRequests > 0 && current.WSBizP95MS >= limits.bizP95MS {
		alerts = append(alerts, alert{"WARN", fmt.Sprintf("business P95 %dms >= %dms", current.WSBizP95MS, limits.bizP95MS)})
	}
	if current.WSBizRequests > 0 && current.WSBizP99MS >= limits.bizP99MS {
		alerts = append(alerts, alert{"CRITICAL", fmt.Sprintf("business P99 %dms >= %dms", current.WSBizP99MS, limits.bizP99MS)})
	}
	if current.DBRequests > 0 && current.DBP95MS >= limits.dbP95MS {
		alerts = append(alerts, alert{"WARN", fmt.Sprintf("database P95 %dms >= %dms", current.DBP95MS, limits.dbP95MS)})
	}
	if current.RedisRequests > 0 && current.RedisP95MS >= limits.redisP95MS {
		alerts = append(alerts, alert{"WARN", fmt.Sprintf("Redis P95 %dms >= %dms", current.RedisP95MS, limits.redisP95MS)})
	}
	previousSnapshot := metrics.Snapshot{}
	if previous != nil {
		previousSnapshot = *previous
	}
	authSuccess := counterDelta(current.WSAuthSuccess, previousSnapshot.WSAuthSuccess)
	authFailed := counterDelta(current.WSAuthFailed, previousSnapshot.WSAuthFailed)
	if total := authSuccess + authFailed; total > 0 && float64(authFailed)/float64(total) >= 0.001 {
		alerts = append(alerts, alert{"WARN", fmt.Sprintf("authentication failure rate %.2f%% >= 0.10%%", 100*float64(authFailed)/float64(total))})
	}
	if counterDelta(current.WSQueueKick, previousSnapshot.WSQueueKick) > 0 {
		alerts = append(alerts, alert{"WARN", "slow clients were disconnected because send queues were full"})
	}
	return alerts
}

func counterDelta(current int64, previous int64) int64 {
	if current < previous {
		return current
	}
	return current - previous
}

func printDashboard(current metrics.Snapshot, previous *metrics.Snapshot, limits thresholds, alerts []alert) {
	status := "OK"
	for _, item := range alerts {
		if item.level == "CRITICAL" {
			status = "CRITICAL"
			break
		}
		status = "WARN"
	}

	fmt.Printf("\nGameServer Metrics  %s  status=%s\n", time.Now().Format(time.RFC3339), status)
	fmt.Println(strings.Repeat("-", 72))
	fmt.Printf("connections       %8d / %-8d\n", current.WSConnections, limits.maxConnections)
	fmt.Printf("auth total        success=%-10d failed=%-10d\n", current.WSAuthSuccess, current.WSAuthFailed)
	fmt.Printf("business latency  requests=%-10d p95=%4dms p99=%4dms\n", current.WSBizRequests, current.WSBizP95MS, current.WSBizP99MS)
	fmt.Printf("database latency  requests=%-10d p95=%4dms p99=%4dms\n", current.DBRequests, current.DBP95MS, current.DBP99MS)
	fmt.Printf("Redis latency     requests=%-10d p95=%4dms p99=%4dms\n", current.RedisRequests, current.RedisP95MS, current.RedisP99MS)
	if previous != nil {
		fmt.Printf("last interval     auth_failed=+%d queue_kick=+%d\n",
			counterDelta(current.WSAuthFailed, previous.WSAuthFailed),
			counterDelta(current.WSQueueKick, previous.WSQueueKick),
		)
	}
	if len(alerts) == 0 {
		fmt.Println("alerts            none")
		return
	}
	for _, item := range alerts {
		fmt.Printf("%-8s          %s\n", item.level, item.message)
	}
}

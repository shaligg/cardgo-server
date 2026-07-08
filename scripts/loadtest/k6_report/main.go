package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

type summary struct {
	Metrics map[string]map[string]interface{} `json:"metrics"`
}

type scenarioResult struct {
	Name             string
	LoginOKRate      float64
	WSConnectOKRate  float64
	WSAuthOKRate     float64
	WSBizAckOKRate   float64
	WSBizRTTP95      float64
	WSBizRTTP99      float64
	WSServerFull     float64
	AvailableMetrics map[string]bool
}

func main() {
	var (
		s1Path string
		s2Path string
		s3Path string
		out    string
	)

	flag.StringVar(&s1Path, "s1", "reports/k6_s1_summary.json", "path to S1 k6 summary json")
	flag.StringVar(&s2Path, "s2", "reports/k6_s2_summary.json", "path to S2 k6 summary json")
	flag.StringVar(&s3Path, "s3", "reports/k6_s3_summary.json", "path to S3 k6 summary json")
	flag.StringVar(&out, "out", "", "output markdown path; empty means stdout")
	flag.Parse()

	results := make([]scenarioResult, 0, 3)
	missing := make([]string, 0)

	for _, item := range []struct {
		name string
		path string
	}{
		{name: "S1", path: s1Path},
		{name: "S2", path: s2Path},
		{name: "S3", path: s3Path},
	} {
		r, err := loadScenario(item.name, item.path)
		if err != nil {
			missing = append(missing, fmt.Sprintf("%s(%s)", item.name, item.path))
			continue
		}
		results = append(results, r)
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })

	report := buildMarkdown(results, missing)
	if out == "" {
		fmt.Print(report)
		return
	}
	if err := os.WriteFile(out, []byte(report), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write report failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("report written: %s\n", out)
}

func loadScenario(name string, path string) (scenarioResult, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return scenarioResult{}, err
	}
	var s summary
	if err := json.Unmarshal(b, &s); err != nil {
		return scenarioResult{}, err
	}

	m := func(metricName, key string) float64 {
		return metricNumber(s.Metrics, metricName, key)
	}

	avail := map[string]bool{}
	for k := range s.Metrics {
		avail[k] = true
	}

	return scenarioResult{
		Name:             name,
		LoginOKRate:      coalesce(m("login_ok_rate", "value"), m("login_ok_rate", "rate")),
		WSConnectOKRate:  coalesce(m("ws_connect_ok_rate", "value"), m("ws_connect_ok_rate", "rate")),
		WSAuthOKRate:     coalesce(m("ws_auth_ok_rate", "value"), m("ws_auth_ok_rate", "rate")),
		WSBizAckOKRate:   coalesce(m("ws_biz_ack_ok_rate", "value"), m("ws_biz_ack_ok_rate", "rate")),
		WSBizRTTP95:      m("ws_biz_rtt_ms", "p(95)"),
		WSBizRTTP99:      m("ws_biz_rtt_ms", "p(99)"),
		WSServerFull:     coalesce(m("ws_server_full_events", "count"), m("ws_server_full_events", "value")),
		AvailableMetrics: avail,
	}, nil
}

func buildMarkdown(results []scenarioResult, missing []string) string {
	var b strings.Builder
	b.WriteString("# k6 Summary Report\n\n")
	b.WriteString(fmt.Sprintf("- Generated at: %s\n", time.Now().Format(time.RFC3339)))
	if len(missing) > 0 {
		b.WriteString(fmt.Sprintf("- Missing inputs: %s\n", strings.Join(missing, ", ")))
	}
	b.WriteString("\n")

	b.WriteString("## Scenario Metrics\n\n")
	b.WriteString("| Scenario | login_ok_rate | ws_connect_ok_rate | ws_auth_ok_rate | ws_biz_ack_ok_rate | ws_biz_rtt_p95(ms) | ws_biz_rtt_p99(ms) | ws_server_full_events |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, r := range results {
		b.WriteString(fmt.Sprintf(
			"| %s | %s | %s | %s | %s | %s | %s | %s |\n",
			r.Name,
			formatRate(r.LoginOKRate),
			formatRate(r.WSConnectOKRate),
			formatRate(r.WSAuthOKRate),
			formatRate(r.WSBizAckOKRate),
			formatFloat(r.WSBizRTTP95),
			formatFloat(r.WSBizRTTP99),
			formatFloat(r.WSServerFull),
		))
	}
	b.WriteString("\n")

	b.WriteString("## Gate Decision\n\n")
	b.WriteString("| Scenario | Result | Failed Gates |\n")
	b.WriteString("|---|---|---|\n")
	for _, r := range results {
		failed := evaluate(r)
		result := "PASS"
		failedText := "-"
		if len(failed) > 0 {
			result = "FAIL"
			failedText = strings.Join(failed, "; ")
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", r.Name, result, failedText))
	}
	b.WriteString("\n")

	b.WriteString("## Thresholds\n\n")
	b.WriteString("- S1/S2: login>=99.9%, ws_connect>=99.5%, ws_auth>=99.9%, ws_biz_ack>=99.0%, ws_biz_rtt_p95<50ms, ws_biz_rtt_p99<120ms, ws_server_full=0\n")
	b.WriteString("- S3: ws_connect>=90%, ws_auth>=90%, ws_server_full>0\n")

	return b.String()
}

func evaluate(r scenarioResult) []string {
	failed := make([]string, 0)
	if r.Name == "S3" {
		if isNaN(r.WSConnectOKRate) {
			failed = append(failed, "ws_connect_ok_rate missing")
		}
		if isNaN(r.WSAuthOKRate) {
			failed = append(failed, "ws_auth_ok_rate missing")
		}
		if isNaN(r.WSServerFull) {
			failed = append(failed, "ws_server_full_events missing")
		}
		if !isNaN(r.WSConnectOKRate) && r.WSConnectOKRate < 0.90 {
			failed = append(failed, "ws_connect_ok_rate<90%")
		}
		if !isNaN(r.WSAuthOKRate) && r.WSAuthOKRate < 0.90 {
			failed = append(failed, "ws_auth_ok_rate<90%")
		}
		if !isNaN(r.WSServerFull) && r.WSServerFull <= 0 {
			failed = append(failed, "ws_server_full_events<=0")
		}
		return failed
	}

	if isNaN(r.LoginOKRate) {
		failed = append(failed, "login_ok_rate missing")
	}
	if isNaN(r.WSConnectOKRate) {
		failed = append(failed, "ws_connect_ok_rate missing")
	}
	if isNaN(r.WSAuthOKRate) {
		failed = append(failed, "ws_auth_ok_rate missing")
	}
	if isNaN(r.WSBizAckOKRate) {
		failed = append(failed, "ws_biz_ack_ok_rate missing")
	}
	if isNaN(r.WSBizRTTP95) {
		failed = append(failed, "ws_biz_rtt_p95 missing")
	}
	if isNaN(r.WSBizRTTP99) {
		failed = append(failed, "ws_biz_rtt_p99 missing")
	}
	if isNaN(r.WSServerFull) {
		failed = append(failed, "ws_server_full_events missing")
	}
	if !isNaN(r.LoginOKRate) && r.LoginOKRate < 0.999 {
		failed = append(failed, "login_ok_rate<99.9%")
	}
	if !isNaN(r.WSConnectOKRate) && r.WSConnectOKRate < 0.995 {
		failed = append(failed, "ws_connect_ok_rate<99.5%")
	}
	if !isNaN(r.WSAuthOKRate) && r.WSAuthOKRate < 0.999 {
		failed = append(failed, "ws_auth_ok_rate<99.9%")
	}
	if !isNaN(r.WSBizAckOKRate) && r.WSBizAckOKRate < 0.99 {
		failed = append(failed, "ws_biz_ack_ok_rate<99.0%")
	}
	if !isNaN(r.WSBizRTTP95) && r.WSBizRTTP95 >= 50 {
		failed = append(failed, "ws_biz_rtt_p95>=50ms")
	}
	if !isNaN(r.WSBizRTTP99) && r.WSBizRTTP99 >= 120 {
		failed = append(failed, "ws_biz_rtt_p99>=120ms")
	}
	if !isNaN(r.WSServerFull) && r.WSServerFull != 0 {
		failed = append(failed, "ws_server_full_events!=0")
	}
	return failed
}

func formatRate(v float64) string {
	if isNaN(v) {
		return "N/A"
	}
	return fmt.Sprintf("%.2f%%", v*100)
}

func formatFloat(v float64) string {
	if isNaN(v) {
		return "N/A"
	}
	if math.Mod(v, 1.0) == 0 {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

func isNaN(v float64) bool {
	return math.IsNaN(v)
}

func coalesce(values ...float64) float64 {
	for _, v := range values {
		if !isNaN(v) {
			return v
		}
	}
	return math.NaN()
}

func metricNumber(metrics map[string]map[string]interface{}, metricName string, key string) float64 {
	metricMap, ok := metrics[metricName]
	if !ok || metricMap == nil {
		return math.NaN()
	}
	if v, ok := toFloat(metricMap[key]); ok {
		return v
	}
	// Backward compatibility for alternative summary shape:
	// metric: { values: { ... } }
	if nested, ok := metricMap["values"].(map[string]interface{}); ok {
		if v, ok := toFloat(nested[key]); ok {
			return v
		}
	}
	return math.NaN()
}

func toFloat(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case int32:
		return float64(t), true
	case uint64:
		return float64(t), true
	case uint32:
		return float64(t), true
	case uint:
		return float64(t), true
	default:
		return 0, false
	}
}

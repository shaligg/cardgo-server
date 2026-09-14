package loginserver

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/bigfish/go_orm_1/internal/platform/account"
)

const requestTimeout = 10 * time.Second
const maxRateEntries = 10000

// HTTPSecurityConfig 定义公开账号 API 的可信代理、请求大小和来源限流。
type HTTPSecurityConfig struct {
	TrustedProxies    []string `yaml:"trusted_proxies"`
	RequestsPerMinute int      `yaml:"requests_per_minute"`
	MaxBodyBytes      int64    `yaml:"max_body_bytes"`
}
type requestSecurity struct {
	proxies []netip.Prefix
	limit   int
	mu      sync.Mutex
	minute  int64
	counts  map[string]int
}

func newRequestSecurity(cfg HTTPSecurityConfig) *requestSecurity {
	s := &requestSecurity{limit: cfg.RequestsPerMinute, counts: make(map[string]int)}
	for _, cidr := range cfg.TrustedProxies {
		if p, err := netip.ParsePrefix(cidr); err == nil {
			s.proxies = append(s.proxies, p)
		}
	}
	return s
}
func (s *requestSecurity) trusted(ip netip.Addr) bool {
	for _, p := range s.proxies {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// clientIP 从直连地址开始，仅沿受信任代理链向左追溯。
func (s *requestSecurity) clientIP(r *http.Request) (string, error) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", account.ErrBadRequest
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return "", account.ErrBadRequest
	}
	ip = ip.Unmap()
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" && s.trusted(ip) {
		chain := strings.Split(forwarded, ",")
		if len(chain) > 64 {
			return "", account.ErrBadRequest
		}
		for i := len(chain) - 1; i >= 0 && s.trusted(ip); i-- {
			ip, err = netip.ParseAddr(strings.TrimSpace(chain[i]))
			if err != nil {
				return "", account.ErrBadRequest
			}
			ip = ip.Unmap()
		}
	}
	return ip.String(), nil
}

// allow 采用有界的分钟窗口；容量满时拒绝新来源，防止来源枚举撑大内存。
func (s *requestSecurity) allow(ip string) bool {
	minute := time.Now().Unix() / 60
	s.mu.Lock()
	defer s.mu.Unlock()
	if minute != s.minute {
		s.counts = make(map[string]int)
		s.minute = minute
	}
	count, exists := s.counts[ip]
	if !exists && len(s.counts) >= maxRateEntries {
		return false
	}
	if count >= s.limit {
		return false
	}
	s.counts[ip] = count + 1
	return true
}

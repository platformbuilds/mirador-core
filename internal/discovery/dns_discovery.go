package discovery

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// DNSConfig defines a simple DNS-based discovery target
type DNSConfig struct {
	Enabled        bool
	Service        string // e.g. vm-select.vm-select.svc.cluster.local (or short form in same namespace)
	Port           int
	Scheme         string // http | https
	RefreshSeconds int    // how often to re-resolve
	UseSRV         bool   // if true, query _http._tcp.<service>
}

// EndpointsSink is implemented by services that can accept updated endpoint lists
type EndpointsSink interface {
	ReplaceEndpoints([]string)
}

// StartDNSDiscovery periodically resolves service name to pod IPs and updates sink
func StartDNSDiscovery(ctx context.Context, cfg DNSConfig, sink EndpointsSink, log logger.Logger) {
	if !cfg.Enabled || sink == nil {
		return
	}
	if cfg.RefreshSeconds <= 0 {
		cfg.RefreshSeconds = 30
	}
	if cfg.Scheme == "" {
		cfg.Scheme = "http"
	}

	ticker := time.NewTicker(time.Duration(cfg.RefreshSeconds) * time.Second)
	// run once immediately
	resolveAndPush := func() {
		eps := resolveEndpoints(cfg)
		if len(eps) > 0 {
			sink.ReplaceEndpoints(eps)
		} else {
			log.Warn("DNS discovery resolved no endpoints", "service", cfg.Service)
		}
	}
	resolveAndPush()

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				resolveAndPush()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func resolveEndpoints(cfg DNSConfig) []string {
	var out []string
	if cfg.UseSRV {
		// Construct _http._tcp.service form
		service := cfg.Service
		if !strings.HasPrefix(service, "_") {
			service = fmt.Sprintf("_http._tcp.%s", service)
		}
		_, addrs, err := net.LookupSRV("", "", service)
		if err == nil {
			for _, a := range addrs {
				host := strings.TrimSuffix(a.Target, ".")
				port := a.Port
				out = append(out, fmt.Sprintf("%s://%s:%d", cfg.Scheme, host, port))
			}
		}
	} else {
		// A/AAAA records (works with headless services to list pods)
		ips, err := net.LookupIP(cfg.Service)
		if err == nil {
			for _, ip := range ips {
				out = append(out, fmt.Sprintf("%s://%s:%d", cfg.Scheme, ip.String(), cfg.Port))
			}
		}
	}
	// de-duplicate + stable order
	m := map[string]struct{}{}
	uniq := make([]string, 0, len(out))
	for _, e := range out {
		if _, ok := m[e]; ok {
			continue
		}
		m[e] = struct{}{}
		uniq = append(uniq, e)
	}
	sort.Strings(uniq)
	return uniq
}

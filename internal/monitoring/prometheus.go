package monitoring

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// SetupPrometheusMetrics configures Prometheus metrics endpoint for MIRADOR-CORE
func SetupPrometheusMetrics(router *gin.Engine) {
	// Custom registry for MIRADOR-CORE metrics
	registry := prometheus.NewRegistry()

	// Register custom metrics
	registry.MustRegister(
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "mirador_core_build_info",
			Help: "Build information for MIRADOR-CORE",
			ConstLabels: prometheus.Labels{
				"version":    "v2.1.3",
				"component":  "mirador-core",
				"go_version": "1.21",
			},
		}, func() float64 { return 1 }),
	)

	// Expose metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		Registry: registry,
	})))
}

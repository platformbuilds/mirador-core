package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestTelemetryUnmarshalFromYAML(t *testing.T) {
	yaml := `engine:
  telemetry:
    connectors:
      spanmetrics:
        kind: connector
        metrics:
          - name: test_span_calls
            type: counter
            description: "test"
            labels:
              - service_name
              - span_name
    processors:
      isolationforest:
        kind: processor
        labels:
          - iforest_is_anomaly
          - iforest_anomaly_score
`

	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(strings.NewReader(yaml)); err != nil {
		t.Fatalf("failed to read yaml: %v", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if cfg.Engine.Telemetry.Connectors == nil {
		t.Fatalf("expected connectors map populated")
	}
	if _, ok := cfg.Engine.Telemetry.Connectors["spanmetrics"]; !ok {
		t.Fatalf("expected spanmetrics connector present")
	}

	if cfg.Engine.Telemetry.Processors == nil {
		t.Fatalf("expected processors map populated")
	}
	if _, ok := cfg.Engine.Telemetry.Processors["isolationforest"]; !ok {
		t.Fatalf("expected isolationforest processor present")
	}
}

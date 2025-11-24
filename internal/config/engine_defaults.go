package config

// MergeEngineConfigWithDefaults overlays empty fields in provided EngineConfig
// with package defaults from GetDefaultConfig(). This keeps defaults centralized
// in the config package rather than in engine implementation.
func MergeEngineConfigWithDefaults(cfg EngineConfig) EngineConfig {
	def := GetDefaultConfig().Engine

	// Simple scalar fields
	if cfg.MinWindow == 0 {
		cfg.MinWindow = def.MinWindow
	}
	if cfg.MaxWindow == 0 {
		cfg.MaxWindow = def.MaxWindow
	}
	if cfg.DefaultGraphHops == 0 {
		cfg.DefaultGraphHops = def.DefaultGraphHops
	}
	if cfg.DefaultMaxWhys == 0 {
		cfg.DefaultMaxWhys = def.DefaultMaxWhys
	}
	if cfg.RingStrategy == "" {
		cfg.RingStrategy = def.RingStrategy
	}

	// Buckets: only fill zero-valued entries
	if cfg.Buckets.CoreWindowSize == 0 {
		cfg.Buckets.CoreWindowSize = def.Buckets.CoreWindowSize
	}
	if cfg.Buckets.PreRings == 0 {
		cfg.Buckets.PreRings = def.Buckets.PreRings
	}
	if cfg.Buckets.PostRings == 0 {
		cfg.Buckets.PostRings = def.Buckets.PostRings
	}
	if cfg.Buckets.RingStep == 0 {
		cfg.Buckets.RingStep = def.Buckets.RingStep
	}

	if cfg.MinCorrelation == 0 {
		cfg.MinCorrelation = def.MinCorrelation
	}
	if cfg.MinAnomalyScore == 0 {
		cfg.MinAnomalyScore = def.MinAnomalyScore
	}

	// Slices: only set if nil or empty
	if len(cfg.Probes) == 0 {
		cfg.Probes = make([]string, len(def.Probes))
		copy(cfg.Probes, def.Probes)
	}
	if len(cfg.ServiceCandidates) == 0 {
		cfg.ServiceCandidates = make([]string, len(def.ServiceCandidates))
		copy(cfg.ServiceCandidates, def.ServiceCandidates)
	}
	if cfg.DefaultQueryLimit == 0 {
		cfg.DefaultQueryLimit = def.DefaultQueryLimit
	}

	// Labels: if any canonical slice is empty, copy from defaults
	if len(cfg.Labels.Service) == 0 {
		cfg.Labels.Service = make([]string, len(def.Labels.Service))
		copy(cfg.Labels.Service, def.Labels.Service)
	}
	if len(cfg.Labels.Pod) == 0 {
		cfg.Labels.Pod = make([]string, len(def.Labels.Pod))
		copy(cfg.Labels.Pod, def.Labels.Pod)
	}
	if len(cfg.Labels.Namespace) == 0 {
		cfg.Labels.Namespace = make([]string, len(def.Labels.Namespace))
		copy(cfg.Labels.Namespace, def.Labels.Namespace)
	}
	if len(cfg.Labels.Deployment) == 0 {
		cfg.Labels.Deployment = make([]string, len(def.Labels.Deployment))
		copy(cfg.Labels.Deployment, def.Labels.Deployment)
	}
	if len(cfg.Labels.Container) == 0 {
		cfg.Labels.Container = make([]string, len(def.Labels.Container))
		copy(cfg.Labels.Container, def.Labels.Container)
	}
	if len(cfg.Labels.Host) == 0 {
		cfg.Labels.Host = make([]string, len(def.Labels.Host))
		copy(cfg.Labels.Host, def.Labels.Host)
	}
	if len(cfg.Labels.Level) == 0 {
		cfg.Labels.Level = make([]string, len(def.Labels.Level))
		copy(cfg.Labels.Level, def.Labels.Level)
	}

	return cfg
}

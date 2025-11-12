package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"

	"github.com/platformbuilds/mirador-core/internal/version"
)

// resolveOpenAPIPath returns a readable path to openapi.yaml by checking common
// locations when tests change the working directory. It honors
// MIRADOR_OPENAPI_PATH if set, then tries relative fallbacks.
func resolveOpenAPIPath() string {
	if p := os.Getenv("MIRADOR_OPENAPI_PATH"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	candidates := []string{
		"api/openapi.yaml",                              // repo root
		filepath.FromSlash("../../api/openapi.yaml"),    // from internal/api
		filepath.FromSlash("../../../api/openapi.yaml"), // from internal/api/handlers
		filepath.FromSlash("../../../../api/openapi.yaml"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "api/openapi.yaml"
}

func GetOpenAPISpec(c *gin.Context) {
	path := resolveOpenAPIPath()
	data, err := os.ReadFile(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "failed to load openapi.yaml"})
		return
	}
	var obj any
	if err := yaml.Unmarshal(data, &obj); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "failed to parse openapi.yaml"})
		return
	}

	// Inject dynamic examples for VictoriaMetrics-compatible timestamps using build time
	if m, ok := obj.(map[string]any); ok {
		// compute RFC3339 times
		bt := version.BuildTime
		end := time.Now().UTC()
		if bt != "" {
			if t, err := time.Parse(time.RFC3339, bt); err == nil {
				end = t
			}
		}
		start := end.Add(-5 * time.Minute)
		endStr := end.Format(time.RFC3339)
		startStr := start.Format(time.RFC3339)

		if paths, ok := m["paths"].(map[string]any); ok {
			// /query body -> properties.time.example
			if p, ok := paths["/query"].(map[string]any); ok {
				if post, ok := p["post"].(map[string]any); ok {
					if rb, ok := post["requestBody"].(map[string]any); ok {
						if content, ok := rb["content"].(map[string]any); ok {
							if appjson, ok := content["application/json"].(map[string]any); ok {
								if schema, ok := appjson["schema"].(map[string]any); ok {
									if props, ok := schema["properties"].(map[string]any); ok {
										if tprop, ok := props["time"].(map[string]any); ok {
											tprop["example"] = endStr
										}
									}
								}
							}
						}
					}
				}
			}
			// /query_range body -> properties.start/end examples
			if p, ok := paths["/query_range"].(map[string]any); ok {
				if post, ok := p["post"].(map[string]any); ok {
					if rb, ok := post["requestBody"].(map[string]any); ok {
						if content, ok := rb["content"].(map[string]any); ok {
							if appjson, ok := content["application/json"].(map[string]any); ok {
								if schema, ok := appjson["schema"].(map[string]any); ok {
									if props, ok := schema["properties"].(map[string]any); ok {
										if sp, ok := props["start"].(map[string]any); ok {
											sp["example"] = startStr
										}
										if ep, ok := props["end"].(map[string]any); ok {
											ep["example"] = endStr
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, obj)
}

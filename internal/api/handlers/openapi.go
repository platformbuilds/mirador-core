package handlers

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

func GetOpenAPISpec(c *gin.Context) {
	data, err := os.ReadFile("api/openapi.yaml")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "failed to load openapi.yaml"})
		return
	}
	var obj any
	if err := yaml.Unmarshal(data, &obj); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "failed to parse openapi.yaml"})
		return
	}
	c.JSON(http.StatusOK, obj)
}

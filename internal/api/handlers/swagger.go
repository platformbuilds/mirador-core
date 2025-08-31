package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ServeSwaggerUI serves a minimal Swagger UI bound to our OpenAPI spec.
// It pulls swagger-ui assets from a CDN and points to /api/openapi.yaml.
func ServeSwaggerUI(c *gin.Context) {
	const html = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>MIRADOR-CORE API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <style>
    html, body, #swagger-ui { height: 100%; margin: 0; }
    .topbar { display: none; } /* optional: hide swagger topbar */
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.addEventListener('load', () => {
      window.ui = SwaggerUIBundle({
        url: '/api/openapi.yaml',     // use YAML source of truth
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [SwaggerUIBundle.presets.apis],
        layout: "BaseLayout"
      });
    });
  </script>
</body>
</html>`
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

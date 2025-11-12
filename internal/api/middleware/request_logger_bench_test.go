package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// Performance Test Cases: request logger fast-path
func BenchmarkRequestLoggerWithBody(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(RequestLoggerWithBody(logger.New("error")))
	r.POST("/echo", func(c *gin.Context) {
		io.Copy(io.Discard, c.Request.Body)
		c.Status(204)
	})
	srv := httptest.NewServer(r)
	defer srv.Close()
	client := &http.Client{}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/echo", strings.NewReader("hello"))
		resp, err := client.Do(req)
		if err != nil {
			b.Fatalf("err: %v", err)
		}
		resp.Body.Close()
	}
}

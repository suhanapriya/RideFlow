package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/richxcame/ride-hailing/pkg/middleware"
	"github.com/stretchr/testify/assert"
)

func init() { gin.SetMode(gin.TestMode) }

func TestAuditLog_SkipsGetRequests(t *testing.T) {
	router := gin.New()
	router.Use(middleware.AuditLog())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
}

func TestAuditLog_ProcessesPostRequest(t *testing.T) {
	router := gin.New()
	router.Use(middleware.AuditLog())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"created": true})
	})

	body := `{"name":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "created")
}

func TestAuditLog_PreservesRequestBody(t *testing.T) {
	originalBody := `{"email":"user@example.com","password":"secret"}`

	var capturedBody string
	router := gin.New()
	router.Use(middleware.AuditLog())
	router.POST("/test", func(c *gin.Context) {
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read body"})
			return
		}
		capturedBody = string(bodyBytes)
		c.JSON(http.StatusOK, gin.H{"received": true})
	})

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(originalBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, originalBody, capturedBody, "handler should receive the original body after audit middleware")
}

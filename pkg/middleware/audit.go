package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/richxcame/ride-hailing/pkg/logger"
	"go.uber.org/zap"
)

// AuditLog middleware logs state-changing operations
func AuditLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		if method != "POST" && method != "PUT" && method != "PATCH" && method != "DELETE" {
			c.Next()
			return
		}

		start := time.Now()

		// Read and hash request body for privacy
		var bodyHash string
		if c.Request.Body != nil {
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err == nil {
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // Restore body
				hash := sha256.Sum256(bodyBytes)
				bodyHash = hex.EncodeToString(hash[:])
			}
		}

		c.Next()

		duration := time.Since(start)
		
		userID, _ := GetUserID(c)
		userRole, _ := GetUserRole(c)
		
		correlationID := logger.CorrelationIDFromContext(c.Request.Context())

		logger.Info("Audit Log",
			zap.String("user_id", userID.String()),
			zap.String("user_role", string(userRole)),
			zap.String("method", method),
			zap.String("path", c.Request.URL.Path),
			zap.String("ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.String("correlation_id", correlationID),
			zap.String("body_hash", bodyHash),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("duration", duration),
		)
	}
}

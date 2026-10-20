package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// LogEntry структура согласно пункту 8 ТЗ
type LogEntry struct {
	RequestID  string `json:"request_id"`
	Method     string `json:"method"`
	Endpoint   string `json:"endpoint"`
	StatusCode int    `json:"status_code"`
	DurationMS float64 `json:"duration_ms"`
	UserID     *string `json:"user_id"`
	Timestamp  string `json:"timestamp"`
	Body       any    `json:"body,omitempty"`
}

func JSONLoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		requestID := c.GetHeader("X-Request-Id")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Header("X-Request-Id", requestID)
		c.Set("request_id", requestID)

		var requestBody any
		if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut || c.Request.Method == http.MethodDelete {
			if c.Request.Body != nil {
				bodyBytes, err := io.ReadAll(c.Request.Body)
				if err == nil {
					c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
					var parsedJSON any
					if err := json.Unmarshal(bodyBytes, &parsedJSON); err == nil {
						requestBody = parsedJSON
					} else if len(bodyBytes) > 0 {
						requestBody = string(bodyBytes)
					}
				}
			}
		}

		c.Next()

		duration := time.Since(start)
		durationMS := float64(duration.Microseconds()) / 1000.0

		var userID *string
		if uVal, exists := c.Get("user_id"); exists {
			if strVal, ok := uVal.(string); ok && strVal != "" {
				userID = &strVal
			}
		}
		if userID == nil {
			headerUser := c.GetHeader("X-User-Id")
			if headerUser != "" {
				userID = &headerUser
			}
		}

		entry := LogEntry{
			RequestID:  requestID,
			Method:     c.Request.Method,
			Endpoint:   c.Request.URL.Path,
			StatusCode: c.Writer.Status(),
			DurationMS: durationMS,
			UserID:     userID,
			Timestamp:  start.UTC().Format(time.RFC3339),
			Body:       requestBody,
		}

		logBytes, err := json.Marshal(entry)
		if err == nil {
			fmt.Println(string(logBytes))
		}
	}
}

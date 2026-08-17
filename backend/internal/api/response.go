package api

import "github.com/gin-gonic/gin"

// errorBody is the single error envelope every endpoint returns, so the frontend
// can render failures uniformly.
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	// Code is a stable machine-readable identifier, e.g. "starrocks_unavailable".
	Code string `json:"code"`
	// Message is safe to display to an operator.
	Message string `json:"message"`
	// Detail carries the underlying cause. It is included because StarLens is an
	// operator tool where the driver error ("dial tcp ...: connection refused")
	// is usually the actual answer.
	Detail string `json:"detail,omitempty"`
}

func respondError(c *gin.Context, status int, code, message string, cause error) {
	detail := ""
	if cause != nil {
		detail = cause.Error()
		// Surface the cause in the server log as well as the response.
		_ = c.Error(cause)
	}

	c.AbortWithStatusJSON(status, errorBody{
		Error: errorDetail{Code: code, Message: message, Detail: detail},
	})
}

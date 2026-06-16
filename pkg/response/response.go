package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// Envelope is the standard API response wrapper.
type Envelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
}

// APIError provides structured error detail.
type APIError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Envelope{Success: true, Data: data})
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Envelope{Success: true, Data: data})
}

func BadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, Envelope{
		Success: false,
		Error:   &APIError{Code: "BAD_REQUEST", Message: msg},
	})
}

func Unauthorized(c *gin.Context, msg string) {
	c.JSON(http.StatusUnauthorized, Envelope{
		Success: false,
		Error:   &APIError{Code: "UNAUTHORIZED", Message: msg},
	})
}

func Forbidden(c *gin.Context, msg string) {
	c.JSON(http.StatusForbidden, Envelope{
		Success: false,
		Error:   &APIError{Code: "FORBIDDEN", Message: msg},
	})
}

func NotFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, Envelope{
		Success: false,
		Error:   &APIError{Code: "NOT_FOUND", Message: msg},
	})
}

func Conflict(c *gin.Context, msg string) {
	c.JSON(http.StatusConflict, Envelope{
		Success: false,
		Error:   &APIError{Code: "CONFLICT", Message: msg},
	})
}

func UnprocessableEntity(c *gin.Context, msg string) {
	c.JSON(http.StatusUnprocessableEntity, Envelope{
		Success: false,
		Error:   &APIError{Code: "UNPROCESSABLE_ENTITY", Message: msg},
	})
}

func TooManyRequests(c *gin.Context, msg string) {
	c.JSON(http.StatusTooManyRequests, Envelope{
		Success: false,
		Error:   &APIError{Code: "TOO_MANY_REQUESTS", Message: msg},
	})
}

func InternalError(c *gin.Context, msg string) {
	c.JSON(http.StatusInternalServerError, Envelope{
		Success: false,
		Error:   &APIError{Code: "INTERNAL_ERROR", Message: msg},
	})
}

// ValidationError formats go-playground/validator errors into structured details.
func ValidationError(c *gin.Context, err error) {
	var details []map[string]string

	if verrs, ok := err.(validator.ValidationErrors); ok {
		for _, fe := range verrs {
			details = append(details, map[string]string{
				"field":   fe.Field(),
				"tag":     fe.Tag(),
				"message": fieldErrorMessage(fe),
			})
		}
	}

	c.JSON(http.StatusBadRequest, Envelope{
		Success: false,
		Error: &APIError{
			Code:    "VALIDATION_ERROR",
			Message: "request validation failed",
			Details: details,
		},
	})
}

func fieldErrorMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return fe.Field() + " is required"
	case "email":
		return fe.Field() + " must be a valid email"
	case "min":
		return fe.Field() + " must be at least " + fe.Param() + " characters"
	case "max":
		return fe.Field() + " must be at most " + fe.Param() + " characters"
	case "e164":
		return fe.Field() + " must be a valid E.164 phone number (e.g. +1234567890)"
	case "url":
		return fe.Field() + " must be a valid URL"
	case "gt":
		return fe.Field() + " must be greater than " + fe.Param()
	default:
		return fe.Field() + " failed validation: " + fe.Tag()
	}
}

package model

import (
	"errors"
	"net/http"
	"strings"

	apiclient "github.com/Gradient-Linux/concave-tui/internal/client"
)

func isUnavailableAPIError(err error) bool {
	var apiErr *apiclient.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.Status {
	case http.StatusNotFound, http.StatusNotImplemented, http.StatusServiceUnavailable:
		return true
	}
	message := strings.ToLower(strings.TrimSpace(apiErr.Message))
	return strings.Contains(message, "not configured") || strings.Contains(message, "not implemented") || strings.Contains(message, "unavailable")
}

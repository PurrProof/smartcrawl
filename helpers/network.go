package helpers

import "strings"

func IsNetworkError(err error) bool {
	if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "EOF") ||
		strings.Contains(err.Error(), "connection reset by peer") ||
		strings.Contains(err.Error(), "server closed idle connection") ||
		strings.Contains(err.Error(), "504") ||
		strings.Contains(err.Error(), "status code: 429. could not decode body to rpc response: invalid character '<' looking for beginning of value") ||
		strings.Contains(err.Error(), "502") {
		return true
	}
	return false
}

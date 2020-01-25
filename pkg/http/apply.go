package http

import (
	"net/http"
)

type middleware func(http.Handler) http.Handler

// Apply applies a chain of middleware in order
func Apply(handler http.Handler, middlewares ...middleware) http.Handler {
	if len(middlewares) < 1 {
		return handler
	}
	wrapped := handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		wrapped = middlewares[i](wrapped)
	}
	return wrapped
}

package middleware

import (
	"log"
	"net/http"
	"runtime/debug"
)

// RecoveryMiddleware recovers from panics and returns 500 Internal Server Error
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic with stack trace
				log.Printf(
					"[PANIC RECOVERED] %v\nStack trace:\n%s",
					err,
					debug.Stack(),
				)

				// Return 500 Internal Server Error
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"code":"INTERNAL_ERROR","message":"An internal server error occurred"}`))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

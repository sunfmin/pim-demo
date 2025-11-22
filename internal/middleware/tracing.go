package middleware

import (
	"fmt"
	"net/http"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

// TracingMiddleware creates OpenTracing spans for HTTP requests
func TracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract span context from headers if present
		wireContext, _ := opentracing.GlobalTracer().Extract(
			opentracing.HTTPHeaders,
			opentracing.HTTPHeadersCarrier(r.Header),
		)

		// Create span with operation name from HTTP method and path
		operationName := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		span := opentracing.GlobalTracer().StartSpan(
			operationName,
			ext.RPCServerOption(wireContext),
		)
		defer span.Finish()

		// Set standard OpenTracing tags
		ext.HTTPMethod.Set(span, r.Method)
		ext.HTTPUrl.Set(span, r.URL.String())
		ext.Component.Set(span, "http")

		// Inject span context into request context
		ctx := opentracing.ContextWithSpan(r.Context(), span)
		r = r.WithContext(ctx)

		// Create response writer wrapper to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Call next handler
		next.ServeHTTP(rw, r)

		// Set HTTP status code tag
		ext.HTTPStatusCode.Set(span, uint16(rw.statusCode))

		// Mark span as error if status code >= 400
		if rw.statusCode >= 400 {
			ext.Error.Set(span, true)
		}
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}


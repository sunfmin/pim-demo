package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// ContextKey type for context values
type ContextKey string

const (
	// OrganizationIDKey is the context key for organization ID
	OrganizationIDKey ContextKey = "organization_id"
)

// TenantMiddleware extracts organization ID from request and adds to context
// In a real implementation, this would extract from JWT claims or auth header
func TenantMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Extract organization ID from JWT token or auth header
		// For now, we'll get it from a header for development
		orgIDStr := r.Header.Get("X-Organization-ID")

		var orgID uuid.UUID
		if orgIDStr != "" {
			parsedID, err := uuid.Parse(orgIDStr)
			if err != nil {
				// Invalid organization ID
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"code":"INVALID_REQUEST","message":"Invalid organization ID"}`))
				return
			}
			orgID = parsedID
		}

		// Add organization ID to context
		ctx := context.WithValue(r.Context(), OrganizationIDKey, orgID)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// GetOrganizationID retrieves organization ID from context
func GetOrganizationID(ctx context.Context) (uuid.UUID, bool) {
	orgID, ok := ctx.Value(OrganizationIDKey).(uuid.UUID)
	return orgID, ok
}

package handlers

import (
	"encoding/json"
	"net/http"

	pb "github.com/yourorg/pim-demo/api/gen/v1"
)

// WriteJSON writes a JSON response with the given status code
func WriteJSON(w http.ResponseWriter, statusCode int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	return json.NewEncoder(w).Encode(data)
}

// WriteError writes a JSON error response with the given error code
func WriteError(w http.ResponseWriter, errorCode ErrorCode, requestID string) error {
	errorResponse := &pb.ErrorResponse{
		Code:      errorCode.Code,
		Message:   errorCode.Message,
		RequestId: requestID,
	}

	return WriteJSON(w, errorCode.StatusCode, errorResponse)
}

// HandleServiceError automatically maps service errors to HTTP responses
func HandleServiceError(w http.ResponseWriter, err error, requestID string) error {
	errorCode := MapServiceError(err)
	return WriteError(w, errorCode, requestID)
}

// WriteValidationError writes a JSON error response with field-level validation errors
func WriteValidationError(w http.ResponseWriter, requestID string, fieldErrors []*pb.FieldError) error {
	errorResponse := &pb.ErrorResponse{
		Code:        ErrCodeInvalidRequest.Code,
		Message:     ErrCodeInvalidRequest.Message,
		FieldErrors: fieldErrors,
		RequestId:   requestID,
	}

	return WriteJSON(w, http.StatusBadRequest, errorResponse)
}


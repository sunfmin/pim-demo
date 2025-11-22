package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	pb "github.com/yourorg/pim-demo/api/gen/v1"
)

// WriteJSON writes a JSON response with the given status code
func WriteJSON(w http.ResponseWriter, statusCode int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if msg, ok := data.(proto.Message); ok {
		marshalOptions := protojson.MarshalOptions{
			EmitUnpopulated: true,
			UseProtoNames:   true,
		}
		bytes, err := marshalOptions.Marshal(msg)
		if err != nil {
			return err
		}
		_, err = w.Write(bytes)
		return err
	}

	return json.NewEncoder(w).Encode(data)
}

// ReadJSON reads a JSON request body into a Protobuf message
func ReadJSON(r *http.Request, msg proto.Message) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	unmarshalOptions := protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
	return unmarshalOptions.Unmarshal(body, msg)
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

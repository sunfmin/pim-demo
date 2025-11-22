package testutil

import (
	"bytes"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// UnmarshalProtoResponse unmarshals a JSON response body into a protobuf message
func UnmarshalProtoResponse(t *testing.T, body *bytes.Buffer, msg proto.Message) {
	t.Helper()
	if err := protojson.Unmarshal(body.Bytes(), msg); err != nil {
		t.Fatalf("Failed to unmarshal proto response: %v. Body: %s", err, body.String())
	}
}

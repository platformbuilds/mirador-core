package predict

import (
    "testing"
    "google.golang.org/protobuf/proto"
)

func TestPredictProto_MarshalUnmarshal(t *testing.T) {
    m := &GetModelsRequest{}
    b, err := proto.Marshal(m)
    if err != nil { t.Fatalf("marshal: %v", err) }
    var out GetModelsRequest
    if err := proto.Unmarshal(b, &out); err != nil { t.Fatalf("unmarshal: %v", err) }
}

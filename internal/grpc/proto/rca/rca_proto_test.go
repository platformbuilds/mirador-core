package rca

import (
    "testing"
    "google.golang.org/protobuf/proto"
)

func TestRCAProto_MarshalUnmarshal(t *testing.T) {
    m := &InvestigateRequest{}
    b, err := proto.Marshal(m)
    if err != nil { t.Fatalf("marshal: %v", err) }
    var out InvestigateRequest
    if err := proto.Unmarshal(b, &out); err != nil { t.Fatalf("unmarshal: %v", err) }
}

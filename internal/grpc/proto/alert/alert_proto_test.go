package alert

import (
    "testing"
    "google.golang.org/protobuf/proto"
)

func TestAlertProto_MarshalUnmarshal(t *testing.T) {
    m := &Alert{Id: "", Severity: "", Message: ""}
    b, err := proto.Marshal(m)
    if err != nil { t.Fatalf("marshal: %v", err) }
    var out Alert
    if err := proto.Unmarshal(b, &out); err != nil { t.Fatalf("unmarshal: %v", err) }
}


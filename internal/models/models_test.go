package models

import (
    "encoding/json"
    "testing"
    "time"
)

func TestFlexibleTime_JSONRoundtrip(t *testing.T) {
    ft := FlexibleTime{Time: time.Unix(1700000000, 0).UTC()}
    b, err := json.Marshal(ft)
    if err != nil { t.Fatalf("marshal: %v", err) }
    var out FlexibleTime
    if err := json.Unmarshal(b, &out); err != nil { t.Fatalf("unmarshal: %v", err) }
}


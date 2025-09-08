package version

import "testing"

func TestVersionVars_Defined(t *testing.T) {
    _ = Version
    _ = CommitHash
    _ = BuildTime
}


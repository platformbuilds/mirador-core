package utils

import (
	"testing"
)

func TestQueryValidator_ValidateLucene(t *testing.T) {
	v := NewQueryValidator()

	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{
			name:    "empty query",
			query:   "",
			wantErr: true,
		},
		{
			name:    "simple term",
			query:   "error",
			wantErr: false,
		},
		{
			name:    "field term",
			query:   "level:error",
			wantErr: false,
		},
		{
			name:    "phrase",
			query:   `"connection timeout"`,
			wantErr: false,
		},
		{
			name:    "boolean AND",
			query:   "error AND timeout",
			wantErr: false,
		},
		{
			name:    "wildcard",
			query:   "error*",
			wantErr: false,
		},
		{
			name:    "numeric range",
			query:   "duration:[100 TO 500]",
			wantErr: false,
		},
		{
			name:    "dangerous script",
			query:   "<script>alert('xss')</script>",
			wantErr: true,
		},
		{
			name:    "dangerous eval",
			query:   "field:eval(something)",
			wantErr: true,
		},
		{
			name:    "unsupported fuzzy",
			query:   "error~0.8",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateLucene(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("QueryValidator.ValidateLucene() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
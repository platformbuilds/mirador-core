package models

import (
	"testing"
)

func TestUQLParser_Parse(t *testing.T) {
	parser := NewUQLParser()

	tests := []struct {
		name     string
		query    string
		wantType UQLQueryType
		wantErr  bool
	}{
		{
			name:     "SELECT query",
			query:    "SELECT service, level FROM logs:error WHERE level='error'",
			wantType: UQLQueryTypeSelect,
			wantErr:  false,
		},
		{
			name:     "correlation query",
			query:    "logs:error AND metrics:high_latency",
			wantType: UQLQueryTypeCorrelation,
			wantErr:  false,
		},
		{
			name:     "aggregation query",
			query:    "COUNT(*) FROM logs:error",
			wantType: UQLQueryTypeAggregation,
			wantErr:  false,
		},
		{
			name:     "invalid query",
			query:    "",
			wantType: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.Parse(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("UQLParser.Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.Type != tt.wantType {
				t.Errorf("UQLParser.Parse() = %v, want %v", got.Type, tt.wantType)
			}
		})
	}
}

func TestUQLQuery_Validate(t *testing.T) {
	parser := NewUQLParser()

	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{
			name:    "valid SELECT query",
			query:   "SELECT service, level FROM logs:error WHERE level='error'",
			wantErr: false,
		},
		{
			name:    "valid correlation query",
			query:   "logs:error AND metrics:high_latency",
			wantErr: false,
		},
		{
			name:    "invalid empty query",
			query:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.Parse(tt.query)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("UQLParser.Parse() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if err := got.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("UQLQuery.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

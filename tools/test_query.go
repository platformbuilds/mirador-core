//go:build ignore

package main

import (
	"fmt"

	"github.com/platformbuilds/mirador-core/internal/utils"
)

func main() {
	query := "_time:[now-1d TO now] AND service:otelgen"
	validator := &utils.QueryValidator{}

	// Test Lucene validation
	if err := validator.ValidateLucene(query); err != nil {
		fmt.Printf("Lucene validation failed: %v\n", err)
		return
	}
	fmt.Println("Lucene validation passed")

	// Test Bleve validation
	if err := validator.ValidateBleve(query); err != nil {
		fmt.Printf("Bleve validation failed: %v\n", err)
		return
	}
	fmt.Println("Bleve validation passed")
}

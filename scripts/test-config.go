// config-validation-test.go - Simple test script to validate API key configuration loading

package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/viper"

	"github.com/platformbuilds/mirador-core/internal/config"
)

const (
	// MinArgsRequired represents the minimum number of command line arguments required
	MinArgsRequired = 2
	// ExitCodeError represents the exit code for errors
	ExitCodeError = 1
)

func main() {
	if len(os.Args) < MinArgsRequired {
		fmt.Println("Usage: go run config-validation-test.go <config-file>")
		fmt.Println("Example: go run config-validation-test.go configs/config.yaml")
		os.Exit(ExitCodeError)
	}

	configFile := os.Args[1]
	fmt.Printf("Testing configuration file: %s\n", configFile)

	// Initialize viper
	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	// Load configuration
	var cfg config.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Fatalf("Failed to unmarshal config: %v", err)
	}

	fmt.Println("\n=== API Key Configuration Test ===")
	fmt.Println("API Key support has been removed from MIRADOR-CORE core. Manage keys via external gateway or separate service.")

	fmt.Println("\n✅ Configuration loaded successfully!")
	fmt.Printf("Environment: %s\n", cfg.Environment)
}

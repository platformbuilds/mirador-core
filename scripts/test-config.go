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
	fmt.Printf("Enabled: %t\n", cfg.APIKeys.Enabled)

	if cfg.APIKeys.Enabled {
		fmt.Println("\nDefault Limits:")
		fmt.Printf("  Users: %d keys\n", cfg.APIKeys.DefaultLimits.MaxKeysPerUser)
		fmt.Printf("  Tenant Admins: %d keys\n", cfg.APIKeys.DefaultLimits.MaxKeysPerTenantAdmin)
		fmt.Printf("  Global Admins: %d keys\n", cfg.APIKeys.DefaultLimits.MaxKeysPerGlobalAdmin)

		fmt.Println("\nPermission Settings:")
		fmt.Printf("  Allow Tenant Override: %t\n", cfg.APIKeys.AllowTenantOverride)
		fmt.Printf("  Allow Admin Override: %t\n", cfg.APIKeys.AllowAdminOverride)

		fmt.Println("\nExpiry Settings:")
		fmt.Printf("  Enforce Expiry: %t\n", cfg.APIKeys.EnforceExpiry)
		fmt.Printf("  Min Expiry Days: %d\n", cfg.APIKeys.MinExpiryDays)
		fmt.Printf("  Max Expiry Days: %d\n", cfg.APIKeys.MaxExpiryDays)

		if len(cfg.APIKeys.TenantLimits) > 0 {
			fmt.Println("\nTenant-Specific Limits:")
			for _, tenant := range cfg.APIKeys.TenantLimits {
				fmt.Printf("  %s: %d/%d/%d (user/tenant_admin/global_admin)\n",
					tenant.TenantID,
					tenant.MaxKeysPerUser,
					tenant.MaxKeysPerTenantAdmin,
					tenant.MaxKeysPerGlobalAdmin)
			}
		}

		if cfg.APIKeys.GlobalLimitsOverride != nil {
			fmt.Println("\nGlobal Overrides:")
			fmt.Printf("  Users: %d keys\n", cfg.APIKeys.GlobalLimitsOverride.MaxKeysPerUser)
			fmt.Printf("  Tenant Admins: %d keys\n", cfg.APIKeys.GlobalLimitsOverride.MaxKeysPerTenantAdmin)
			fmt.Printf("  Global Admins: %d keys\n", cfg.APIKeys.GlobalLimitsOverride.MaxKeysPerGlobalAdmin)
			fmt.Printf("  Total System Limit: %d keys\n", cfg.APIKeys.GlobalLimitsOverride.MaxTotalKeys)
		}
	}

	fmt.Println("\n✅ Configuration loaded successfully!")
	fmt.Printf("Environment: %s\n", cfg.Environment)
}

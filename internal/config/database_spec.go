package config

import (
	"fmt"
	"strings"
)

// DatabaseSpec declares where configuration is stored for enterprise deployments (declarative; process may still use file/HTTP).
type DatabaseSpec struct {
	// ConfigStorage is one of: yaml (local CONFIG_PATH), http (CONFIG_HTTP_URL), postgres (planned / external tooling).
	ConfigStorage string `yaml:"config_storage" json:"config_storage"`
	Postgres      *PostgresSpec `yaml:"postgres,omitempty" json:"postgres,omitempty"`
}

// PostgresSpec holds non-secret connection settings; use password_env_var + environment or a secrets manager.
type PostgresSpec struct {
	Host           string `yaml:"host" json:"host"`
	Port           int    `yaml:"port" json:"port"`
	User           string `yaml:"user" json:"user"`
	Database       string `yaml:"database" json:"database"`
	SSLMode        string `yaml:"ssl_mode" json:"ssl_mode"`
	PasswordEnvVar string `yaml:"password_env_var,omitempty" json:"password_env_var,omitempty"`
	Schema         string `yaml:"schema,omitempty" json:"schema,omitempty"`
}

// ValidateDatabaseSpec validates spec.database when present.
func ValidateDatabaseSpec(d *DatabaseSpec) error {
	if d == nil {
		return nil
	}
	cs := strings.ToLower(strings.TrimSpace(d.ConfigStorage))
	switch cs {
	case "", "yaml", "http", "postgres":
	default:
		return fmt.Errorf("spec.database.config_storage must be yaml, http, or postgres")
	}
	if cs == "postgres" || d.Postgres != nil {
		p := d.Postgres
		if p == nil {
			if cs == "postgres" {
				return fmt.Errorf("spec.database.postgres is required when config_storage is postgres")
			}
			return nil
		}
		if strings.TrimSpace(p.Host) == "" {
			return fmt.Errorf("spec.database.postgres.host is required")
		}
		if p.Port <= 0 || p.Port > 65535 {
			return fmt.Errorf("spec.database.postgres.port must be between 1 and 65535")
		}
		if strings.TrimSpace(p.User) == "" {
			return fmt.Errorf("spec.database.postgres.user is required")
		}
		if strings.TrimSpace(p.Database) == "" {
			return fmt.Errorf("spec.database.postgres.database is required")
		}
		ssl := strings.TrimSpace(strings.ToLower(p.SSLMode))
		switch ssl {
		case "", "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
		default:
			return fmt.Errorf("spec.database.postgres.ssl_mode must be a valid libpq sslmode")
		}
	}
	return nil
}

package config

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleReadonly Role = "readonly"
)

type LocalAuthUser struct {
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
	Role     Role   `yaml:"role" json:"role"`
}

type LocalAuthSpec struct {
	Enabled bool            `yaml:"enabled" json:"enabled"`
	Users   []LocalAuthUser `yaml:"users,omitempty" json:"users,omitempty"`
}

type ADLDSSpec struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	// Examples: ldaps://adlds.example.com:636 or ldap://10.1.2.3:389
	URL string `yaml:"url,omitempty" json:"url,omitempty"`
	// Service account used for search bind.
	BindDN string `yaml:"bind_dn,omitempty" json:"bind_dn,omitempty"`
	// Environment variable name that contains service account password.
	BindPasswordEnvVar string `yaml:"bind_password_env_var,omitempty" json:"bind_password_env_var,omitempty"`
	// Search base for user lookup.
	BaseDN string `yaml:"base_dn,omitempty" json:"base_dn,omitempty"`
	// LDAP filter template; %s is escaped username. Default: (sAMAccountName=%s)
	UserFilter string `yaml:"user_filter,omitempty" json:"user_filter,omitempty"`
	// Group DN -> role mapping; first matched group wins by precedence admin > operator > readonly.
	GroupRoleMap map[string]Role `yaml:"group_role_map,omitempty" json:"group_role_map,omitempty"`
}

type AuthSpec struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	// Session TTL in minutes for issued bearer tokens. Default 480.
	SessionTTLMinutes int `yaml:"session_ttl_minutes,omitempty" json:"session_ttl_minutes,omitempty"`
	Local             *LocalAuthSpec `yaml:"local,omitempty" json:"local,omitempty"`
	ADLDS             *ADLDSSpec     `yaml:"adlds,omitempty" json:"adlds,omitempty"`
}

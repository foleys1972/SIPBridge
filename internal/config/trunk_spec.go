package config

type SIPTrunkSpec struct {
	ID   string `yaml:"id" json:"id"`
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Signaling destination for this trunk.
	ProxyAddr string `yaml:"proxy_addr" json:"proxy_addr"`
	ProxyPort int    `yaml:"proxy_port" json:"proxy_port"`
	Transport string `yaml:"transport,omitempty" json:"transport,omitempty"` // udp | tls

	// Optional TLS profile for this trunk.
	TLSRootCAFile         string `yaml:"tls_root_ca_file,omitempty" json:"tls_root_ca_file,omitempty"`
	TLSClientCertFile     string `yaml:"tls_client_cert_file,omitempty" json:"tls_client_cert_file,omitempty"`
	TLSClientKeyFile      string `yaml:"tls_client_key_file,omitempty" json:"tls_client_key_file,omitempty"`
	TLSInsecureSkipVerify bool   `yaml:"tls_insecure_skip_verify,omitempty" json:"tls_insecure_skip_verify,omitempty"`
	TLSServerName         string `yaml:"tls_server_name,omitempty" json:"tls_server_name,omitempty"`
}

// DialPlanRule decides which SIP trunk should carry an outbound target URI.
type DialPlanRule struct {
	ID string `yaml:"id" json:"id"`
	// Enabled defaults to true when omitted.
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	// Optional user-part prefix match (sip:user@domain)
	UserPrefix string `yaml:"user_prefix,omitempty" json:"user_prefix,omitempty"`
	// Optional exact domain match.
	Domain string `yaml:"domain,omitempty" json:"domain,omitempty"`
	// Optional regex against full target URI.
	URIRegex string `yaml:"uri_regex,omitempty" json:"uri_regex,omitempty"`
	// Trunk ID to route to when this rule matches.
	TargetTrunkID string `yaml:"target_trunk_id" json:"target_trunk_id"`
}

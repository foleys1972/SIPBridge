package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	SIP SIPConfig
	API APIConfig
	ConfigPath string

	// HTTP config source (optional): load the same YAML from a URL on all nodes for shared config.
	// When set, CONFIG_PATH is not read for initial load; config API writes are disabled unless the
	// HTTP source is updated out-of-band (GitOps, object store, etc.).
	ConfigHTTPURL        string
	ConfigHTTPBearerToken string
	ConfigHTTPPollSeconds int
	ConfigHTTPTLSInsecure bool

	// Cluster capacity limits (merged with spec.cluster from YAML at startup).
	Cluster ClusterLimits
}

type SIPConfig struct {
	BindAddr string `json:"bind_addr"`
	UDPPort  int    `json:"udp_port"`

	OutboundProxyAddr string `json:"outbound_proxy_addr"`
	OutboundProxyPort int    `json:"outbound_proxy_port"`
	// OutboundTransport is how SIPBridge reaches the outbound proxy / SBC: "udp" or "tls" (SIPS to Oracle / AudioCodes).
	OutboundTransport string `json:"outbound_transport"`

	// AdvertiseAddr optional host/IP for Contact / SDP / Via when auto-detection is wrong behind NAT.
	AdvertiseAddr string `json:"advertise_addr"`

	// Outbound TLS to SBC (Oracle / AudioCodes)
	TLSRootCAFile         string `json:"tls_root_ca_file"`
	TLSClientCertFile     string `json:"tls_client_cert_file"`
	TLSClientKeyFile      string `json:"tls_client_key_file"`
	TLSInsecureSkipVerify bool   `json:"tls_insecure_skip_verify"`
	TLSServerName         string `json:"tls_server_name"` // SNI; set to SBC hostname when cert does not match IP

	// SessionTimer adds Min-SE / Session-Expires on INVITE (often required by SBCs).
	SessionTimerEnabled bool `json:"session_timer_enabled"`
}

type APIConfig struct {
	BindAddr string
	Port     int
}

func LoadFromEnv() (Config, error) {
	cfg := Config{
		SIP: SIPConfig{
			BindAddr:          envOr("SIP_BIND_ADDR", "0.0.0.0"),
			UDPPort:           envOrInt("SIP_UDP_PORT", 5060),
			OutboundProxyAddr: envOr("SIP_OUTBOUND_PROXY_ADDR", ""),
			OutboundProxyPort: envOrInt("SIP_OUTBOUND_PROXY_PORT", 0),
			OutboundTransport: strings.ToLower(strings.TrimSpace(envOr("SIP_OUTBOUND_TRANSPORT", "udp"))),
			AdvertiseAddr:     strings.TrimSpace(envOr("SIP_ADVERTISE_ADDR", "")),

			TLSRootCAFile:         envOr("SIP_TLS_ROOT_CA_FILE", ""),
			TLSClientCertFile:     envOr("SIP_TLS_CLIENT_CERT_FILE", ""),
			TLSClientKeyFile:      envOr("SIP_TLS_CLIENT_KEY_FILE", ""),
			TLSInsecureSkipVerify: envBool("SIP_TLS_INSECURE_SKIP_VERIFY", false),
			TLSServerName:       envOr("SIP_TLS_SERVER_NAME", ""),
			SessionTimerEnabled: envBool("SIP_SESSION_TIMER_ENABLED", false),
		},
		API: APIConfig{
			BindAddr: envOr("API_BIND_ADDR", "127.0.0.1"),
			Port:     envOrInt("API_PORT", 8080),
		},
		ConfigPath: envOr("CONFIG_PATH", "config.yaml"),

		ConfigHTTPURL:         strings.TrimSpace(envOr("CONFIG_HTTP_URL", "")),
		ConfigHTTPBearerToken: strings.TrimSpace(envOr("CONFIG_HTTP_BEARER_TOKEN", "")),
		ConfigHTTPPollSeconds: envOrInt("CONFIG_HTTP_POLL_SECONDS", 0),
		ConfigHTTPTLSInsecure:   envBool("CONFIG_HTTP_TLS_INSECURE", false),

		Cluster: ClusterLimits{
			MaxConcurrentCalls:       envOrInt("SIPBRIDGE_MAX_CONCURRENT_CALLS", 0),
			SoftMaxConcurrentCalls:   envOrInt("SIPBRIDGE_SOFT_MAX_CONCURRENT_CALLS", 0),
			OverflowRedirectEnabled:  envBool("SIPBRIDGE_OVERFLOW_REDIRECT_ENABLED", false),
			OverflowRedirectSIPURI:     strings.TrimSpace(envOr("SIPBRIDGE_OVERFLOW_REDIRECT_SIP_URI", "")),
		},
	}

	if cfg.ConfigHTTPURL != "" {
		u, err := url.Parse(cfg.ConfigHTTPURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return Config{}, fmt.Errorf("CONFIG_HTTP_URL must be a valid http or https URL with a host")
		}
	}
	if cfg.ConfigHTTPPollSeconds < 0 {
		return Config{}, fmt.Errorf("CONFIG_HTTP_POLL_SECONDS must be >= 0")
	}

	if ip := net.ParseIP(cfg.API.BindAddr); ip == nil {
		return Config{}, fmt.Errorf("invalid API_BIND_ADDR: %q", cfg.API.BindAddr)
	}
	if cfg.API.Port <= 0 || cfg.API.Port > 65535 {
		return Config{}, fmt.Errorf("invalid API_PORT: %d", cfg.API.Port)
	}

	return cfg, nil
}

// ValidateSIPConfig checks SIP listener and SBC settings (after env + file merge).
func ValidateSIPConfig(s SIPConfig) error {
	if ip := net.ParseIP(s.BindAddr); ip == nil {
		return fmt.Errorf("invalid SIP bind address: %q", s.BindAddr)
	}
	if s.UDPPort <= 0 || s.UDPPort > 65535 {
		return fmt.Errorf("invalid SIP UDP port: %d", s.UDPPort)
	}
	if s.OutboundProxyAddr != "" {
		if ip := net.ParseIP(s.OutboundProxyAddr); ip == nil {
			return fmt.Errorf("invalid SIP_OUTBOUND_PROXY_ADDR: %q (use IP address)", s.OutboundProxyAddr)
		}
		if s.OutboundProxyPort <= 0 || s.OutboundProxyPort > 65535 {
			return fmt.Errorf("invalid SIP_OUTBOUND_PROXY_PORT: %d", s.OutboundProxyPort)
		}
	}
	switch s.OutboundTransport {
	case "udp", "tls":
	default:
		return fmt.Errorf("invalid outbound transport: %q (want udp or tls)", s.OutboundTransport)
	}
	if s.OutboundTransport == "tls" && s.OutboundProxyAddr == "" {
		return fmt.Errorf("outbound transport tls requires outbound proxy address and port")
	}
	return nil
}

func envOr(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

func envOrInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envBool(k string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(k)))
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}


package config

import "strings"

// SIPStackSpec holds optional SIP/SBC settings saved in config.yaml (spec.sipStack).
// Non-nil fields override the corresponding value from environment variables at startup.
type SIPStackSpec struct {
	BindAddr              *string `yaml:"bind_addr,omitempty" json:"bind_addr,omitempty"`
	UDPPort               *int    `yaml:"udp_port,omitempty" json:"udp_port,omitempty"`
	OutboundProxyAddr     *string `yaml:"outbound_proxy_addr,omitempty" json:"outbound_proxy_addr,omitempty"`
	OutboundProxyPort     *int    `yaml:"outbound_proxy_port,omitempty" json:"outbound_proxy_port,omitempty"`
	OutboundTransport     *string `yaml:"outbound_transport,omitempty" json:"outbound_transport,omitempty"`
	AdvertiseAddr         *string `yaml:"advertise_addr,omitempty" json:"advertise_addr,omitempty"`
	TLSRootCAFile         *string `yaml:"tls_root_ca_file,omitempty" json:"tls_root_ca_file,omitempty"`
	TLSClientCertFile     *string `yaml:"tls_client_cert_file,omitempty" json:"tls_client_cert_file,omitempty"`
	TLSClientKeyFile      *string `yaml:"tls_client_key_file,omitempty" json:"tls_client_key_file,omitempty"`
	TLSInsecureSkipVerify *bool   `yaml:"tls_insecure_skip_verify,omitempty" json:"tls_insecure_skip_verify,omitempty"`
	TLSServerName         *string `yaml:"tls_server_name,omitempty" json:"tls_server_name,omitempty"`
	SessionTimerEnabled   *bool   `yaml:"session_timer_enabled,omitempty" json:"session_timer_enabled,omitempty"`
}

// MergeSIPFromSpec overlays saved spec onto base (typically from env). Nil fields leave base unchanged.
func MergeSIPFromSpec(base SIPConfig, spec *SIPStackSpec) SIPConfig {
	if spec == nil {
		return base
	}
	out := base
	if spec.BindAddr != nil {
		out.BindAddr = *spec.BindAddr
	}
	if spec.UDPPort != nil {
		out.UDPPort = *spec.UDPPort
	}
	if spec.OutboundProxyAddr != nil {
		out.OutboundProxyAddr = *spec.OutboundProxyAddr
	}
	if spec.OutboundProxyPort != nil {
		out.OutboundProxyPort = *spec.OutboundProxyPort
	}
	if spec.OutboundTransport != nil {
		out.OutboundTransport = stringsToLowerTrim(*spec.OutboundTransport)
	}
	if spec.AdvertiseAddr != nil {
		out.AdvertiseAddr = stringsTrim(*spec.AdvertiseAddr)
	}
	if spec.TLSRootCAFile != nil {
		out.TLSRootCAFile = *spec.TLSRootCAFile
	}
	if spec.TLSClientCertFile != nil {
		out.TLSClientCertFile = *spec.TLSClientCertFile
	}
	if spec.TLSClientKeyFile != nil {
		out.TLSClientKeyFile = *spec.TLSClientKeyFile
	}
	if spec.TLSInsecureSkipVerify != nil {
		out.TLSInsecureSkipVerify = *spec.TLSInsecureSkipVerify
	}
	if spec.TLSServerName != nil {
		out.TLSServerName = stringsTrim(*spec.TLSServerName)
	}
	if spec.SessionTimerEnabled != nil {
		out.SessionTimerEnabled = *spec.SessionTimerEnabled
	}
	return out
}

func stringsTrim(s string) string {
	return strings.TrimSpace(s)
}

func stringsToLowerTrim(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

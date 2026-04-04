package config

import "fmt"

// ClusterSpec is optional HA / capacity tuning (merged with SIPBRIDGE_* env at startup).
type ClusterSpec struct {
	MaxConcurrentCalls *int    `yaml:"max_concurrent_calls,omitempty" json:"max_concurrent_calls,omitempty"`
	SoftMaxConcurrentCalls *int `yaml:"soft_max_concurrent_calls,omitempty" json:"soft_max_concurrent_calls,omitempty"`
	OverflowRedirectEnabled *bool `yaml:"overflow_redirect_enabled,omitempty" json:"overflow_redirect_enabled,omitempty"`
	OverflowRedirectSIPURI *string `yaml:"overflow_redirect_sip_uri,omitempty" json:"overflow_redirect_sip_uri,omitempty"`
}

// ClusterLimits defines admission control for new SIP dialogs (merged env + spec.cluster).
type ClusterLimits struct {
	MaxConcurrentCalls       int    `json:"max_concurrent_calls"`
	SoftMaxConcurrentCalls   int    `json:"soft_max_concurrent_calls"`
	OverflowRedirectEnabled  bool   `json:"overflow_redirect_enabled"`
	OverflowRedirectSIPURI   string `json:"overflow_redirect_sip_uri"`
}

// MergeClusterLimits overlays spec.cluster onto env-derived defaults.
func MergeClusterLimits(base ClusterLimits, spec *ClusterSpec) ClusterLimits {
	if spec == nil {
		return base
	}
	out := base
	if spec.MaxConcurrentCalls != nil {
		out.MaxConcurrentCalls = *spec.MaxConcurrentCalls
	}
	if spec.SoftMaxConcurrentCalls != nil {
		out.SoftMaxConcurrentCalls = *spec.SoftMaxConcurrentCalls
	}
	if spec.OverflowRedirectEnabled != nil {
		out.OverflowRedirectEnabled = *spec.OverflowRedirectEnabled
	}
	if spec.OverflowRedirectSIPURI != nil {
		out.OverflowRedirectSIPURI = stringsTrim(*spec.OverflowRedirectSIPURI)
	}
	return out
}

// EffectiveSoftMax returns the soft threshold; if unset but Max>0, defaults to 80% of max.
func EffectiveSoftMax(c ClusterLimits) int {
	if c.MaxConcurrentCalls <= 0 {
		return 0
	}
	if c.SoftMaxConcurrentCalls > 0 {
		return c.SoftMaxConcurrentCalls
	}
	return c.MaxConcurrentCalls * 8 / 10
}

// ValidateClusterLimits checks merged cluster settings.
func ValidateClusterLimits(c ClusterLimits) error {
	if c.MaxConcurrentCalls < 0 {
		return fmt.Errorf("cluster max_concurrent_calls must be >= 0")
	}
	if c.SoftMaxConcurrentCalls < 0 {
		return fmt.Errorf("cluster soft_max_concurrent_calls must be >= 0")
	}
	if c.MaxConcurrentCalls > 0 && c.SoftMaxConcurrentCalls > c.MaxConcurrentCalls {
		return fmt.Errorf("cluster soft_max_concurrent_calls cannot exceed max_concurrent_calls")
	}
	if c.OverflowRedirectEnabled && stringsTrim(c.OverflowRedirectSIPURI) == "" {
		return fmt.Errorf("cluster overflow_redirect_sip_uri is required when overflow_redirect_enabled is true")
	}
	return nil
}

// ValidateClusterSpec validates spec.cluster before merge with env.
func ValidateClusterSpec(spec *ClusterSpec) error {
	if spec == nil {
		return nil
	}
	max := -1
	if spec.MaxConcurrentCalls != nil {
		max = *spec.MaxConcurrentCalls
		if max < 0 {
			return fmt.Errorf("spec.cluster.max_concurrent_calls must be >= 0")
		}
	}
	if spec.SoftMaxConcurrentCalls != nil {
		s := *spec.SoftMaxConcurrentCalls
		if s < 0 {
			return fmt.Errorf("spec.cluster.soft_max_concurrent_calls must be >= 0")
		}
		if max >= 0 && s > max {
			return fmt.Errorf("spec.cluster.soft_max_concurrent_calls cannot exceed max_concurrent_calls")
		}
	}
	if spec.OverflowRedirectEnabled != nil && *spec.OverflowRedirectEnabled {
		if spec.OverflowRedirectSIPURI == nil || stringsTrim(*spec.OverflowRedirectSIPURI) == "" {
			return fmt.Errorf("spec.cluster.overflow_redirect_sip_uri is required when overflow_redirect_enabled is true")
		}
	}
	return nil
}

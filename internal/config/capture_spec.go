package config

// CaptureSpec controls local audio capture to WAV + metadata JSON.
// It is separate from SIPREC: SIPREC sends audio to an external recorder;
// local capture writes files directly to disk on this host.
type CaptureSpec struct {
	// Enabled turns local audio capture on or off globally.
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Directory is the root path under which per-call files are written.
	// Subdirectory layout: <directory>/<YYYY-MM-DD>/<files>
	// Defaults to "captures" relative to the working directory.
	Directory string `yaml:"directory,omitempty" json:"directory,omitempty"`

	// CaptureBridges enables capture for bridge (point-to-point) calls.
	// Default true when Enabled is true.
	CaptureBridges *bool `yaml:"capture_bridges,omitempty" json:"capture_bridges,omitempty"`

	// CaptureConferences enables capture for conference group calls (ARD/MRD/HOOT).
	// Default true when Enabled is true.
	CaptureConferences *bool `yaml:"capture_conferences,omitempty" json:"capture_conferences,omitempty"`
}

// CaptureDirectory returns the effective capture root directory.
func (c *CaptureSpec) CaptureDirectory() string {
	if c == nil || c.Directory == "" {
		return "captures"
	}
	return c.Directory
}

// BridgeCaptureEnabled reports whether bridge calls should be captured.
func (c *CaptureSpec) BridgeCaptureEnabled() bool {
	if c == nil || !c.Enabled {
		return false
	}
	if c.CaptureBridges == nil {
		return true
	}
	return *c.CaptureBridges
}

// ConferenceCaptureEnabled reports whether conference group calls should be captured.
func (c *CaptureSpec) ConferenceCaptureEnabled() bool {
	if c == nil || !c.Enabled {
		return false
	}
	if c.CaptureConferences == nil {
		return true
	}
	return *c.CaptureConferences
}

package config

// IPTVSourceSpec defines one multicast IPTV RTP source that can be bridged
// into conference/hoot media.
type IPTVSourceSpec struct {
	ID string `yaml:"id" json:"id"`
	// Friendly UI label.
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	// IPv4 multicast group address, e.g. 239.10.10.10.
	MulticastIP string `yaml:"multicast_ip" json:"multicast_ip"`
	// UDP port carrying RTP.
	Port int `yaml:"port" json:"port"`
	// RTP payload type to accept. Defaults to 0 (PCMU).
	PayloadType int `yaml:"payload_type,omitempty" json:"payload_type,omitempty"`
	// When true, run ffmpeg to extract audio from a video transport stream.
	ExtractAudioFromVideo bool `yaml:"extract_audio_from_video,omitempty" json:"extract_audio_from_video,omitempty"`
	// Jitter buffer delay in ms before forwarding extracted RTP to bridges.
	JitterBufferMs int  `yaml:"jitter_buffer_ms,omitempty" json:"jitter_buffer_ms,omitempty"`
	Enabled        bool `yaml:"enabled" json:"enabled"`
}

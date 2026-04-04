package api

import (
	"encoding/json"
	"net/http"

	"sipbridge/internal/config"
	"gopkg.in/yaml.v3"
)

func (s *Server) handleRecordingSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleRecordingSettingsGet(w, r)
	case http.MethodPut:
		s.handleRecordingSettingsPut(w, r)
	case http.MethodDelete:
		s.handleRecordingSettingsDelete(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleRecordingSettingsGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var saved *config.RecordingSpec
	if s.cm != nil {
		saved = s.cm.Current().Spec.Recording
	}
	note := "When global recording and SIPREC are enabled, the SIP stack sends a multipart INVITE (SDP + metadata) toward the configured recorder URI or regional trunk. Your SBC/recorder must accept this dialog; RTP fork/mix may still require additional integration."
	_ = json.NewEncoder(w).Encode(map[string]any{
		"saved": saved,
		"note":  note,
	})
}

func (s *Server) handleRecordingSettingsPut(w http.ResponseWriter, r *http.Request) {
	if s.cm == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	var req config.RecordingSpec
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "invalid JSON"})
		return
	}
	if err := config.ValidateRecordingSpec(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	cur := s.cm.Current()
	cur.Spec.Recording = &req
	yamlBytes, err := yaml.Marshal(&cur)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if _, err := s.cm.ApplyYAML(yamlBytes); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *Server) handleRecordingSettingsDelete(w http.ResponseWriter, r *http.Request) {
	if s.cm == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	cur := s.cm.Current()
	cur.Spec.Recording = nil
	yamlBytes, err := yaml.Marshal(&cur)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if _, err := s.cm.ApplyYAML(yamlBytes); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

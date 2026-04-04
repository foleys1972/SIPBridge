package api

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"

	"sipbridge/internal/config"
	"gopkg.in/yaml.v3"
)

func maskParticipantPIN(pin string) string {
	pin = strings.TrimSpace(pin)
	if pin == "" {
		return ""
	}
	n := len(pin)
	if n > 12 {
		n = 12
	}
	return strings.Repeat("•", n)
}

func (s *Server) handleUsersV1(w http.ResponseWriter, r *http.Request) {
	if s.cm == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/users")
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if path == "" {
		switch r.Method {
		case http.MethodGet:
			s.handleUserList(w, r)
		case http.MethodPost:
			s.handleUserCreate(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}
	id := parts[0]
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			s.handleUserGet(w, r, id)
		case http.MethodPut:
			s.handleUserPut(w, r, id)
		case http.MethodDelete:
			s.handleUserDelete(w, r, id)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}
	if len(parts) == 2 && parts[1] == "reset-pin" {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		s.handleUserResetPIN(w, r, id)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func (s *Server) handleUserList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	cur := s.cm.Current()
	out := make([]map[string]any, 0, len(cur.Spec.Users))
	for _, u := range cur.Spec.Users {
		out = append(out, map[string]any{
			"id":            u.ID,
			"employee_id":   u.ID,
			"display_name":  u.DisplayName,
			"region":        u.Region,
			"pin_set":       strings.TrimSpace(u.ParticipantID) != "",
			"recording_opt_in": u.RecordingOptIn,
			"device_count":     len(u.Devices),
		})
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"users": out})
}

func userIndexByID(users []config.User, id string) int {
	for i := range users {
		if users[i].ID == id {
			return i
		}
	}
	return -1
}

func participantIDTaken(users []config.User, participantID string, exceptIdx int) bool {
	p := strings.TrimSpace(participantID)
	if p == "" {
		return false
	}
	for i, u := range users {
		if i == exceptIdx {
			continue
		}
		if strings.TrimSpace(u.ParticipantID) == p {
			return true
		}
	}
	return false
}

func (s *Server) handleUserGet(w http.ResponseWriter, r *http.Request, id string) {
	w.Header().Set("Content-Type", "application/json")
	cur := s.cm.Current()
	idx := userIndexByID(cur.Spec.Users, id)
	if idx < 0 {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "user not found"})
		return
	}
	u := cur.Spec.Users[idx]
	_ = json.NewEncoder(w).Encode(map[string]any{
		"user": map[string]any{
			"id":            u.ID,
			"employee_id":   u.ID,
			"display_name":  u.DisplayName,
			"region":        u.Region,
			"pin_masked":    maskParticipantPIN(u.ParticipantID),
			"pin_set":       strings.TrimSpace(u.ParticipantID) != "",
			"recording_opt_in":             u.RecordingOptIn,
			"devices":                      u.Devices,
			"allowed_bridge_ids":           u.AllowedBridgeIDs,
			"allowed_conference_group_ids": u.AllowedConferenceGroupIDs,
		},
	})
}

type userPutBody struct {
	DisplayName               string              `json:"display_name"`
	Region                    string              `json:"region"`
	AllowedBridgeIDs          []string            `json:"allowed_bridge_ids"`
	AllowedConferenceGroupIDs []string            `json:"allowed_conference_group_ids"`
	ParticipantID           *string               `json:"participant_id"` // optional: set new PIN (digits only)
	RecordingOptIn          *bool                 `json:"recording_opt_in"`
	Devices                   []config.UserDevice `json:"devices"`
}

func (s *Server) handleUserPut(w http.ResponseWriter, r *http.Request, id string) {
	w.Header().Set("Content-Type", "application/json")
	if s.cm.ConfigReadOnly() {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "config is read-only (CONFIG_HTTP_URL)"})
		return
	}
	cur := s.cm.Current()
	idx := userIndexByID(cur.Spec.Users, id)
	if idx < 0 {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "user not found"})
		return
	}
	body, _ := io.ReadAll(r.Body)
	var req userPutBody
	if err := json.Unmarshal(body, &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "invalid JSON"})
		return
	}
	u := cur.Spec.Users[idx]
	u.DisplayName = req.DisplayName
	u.Region = req.Region
	u.AllowedBridgeIDs = req.AllowedBridgeIDs
	u.AllowedConferenceGroupIDs = req.AllowedConferenceGroupIDs
	if req.RecordingOptIn != nil {
		u.RecordingOptIn = *req.RecordingOptIn
	}
	if req.Devices != nil {
		if err := config.ValidateUserDeviceList(req.Devices); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
			return
		}
		u.Devices = req.Devices
	}
	if req.ParticipantID != nil {
		pin := strings.TrimSpace(*req.ParticipantID)
		pin = strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, pin)
		if pin == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "participant_id must be non-empty digits"})
			return
		}
		if participantIDTaken(cur.Spec.Users, pin, idx) {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "participant_id already in use"})
			return
		}
		u.ParticipantID = pin
	}
	cur.Spec.Users[idx] = u
	yamlBytes, err := yaml.Marshal(&cur)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if _, err := s.cm.ApplyYAML(yamlBytes); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

type userCreateBody struct {
	ID                        string              `json:"id"`
	DisplayName               string              `json:"display_name"`
	Region                    string              `json:"region"`
	AllowedBridgeIDs          []string            `json:"allowed_bridge_ids"`
	AllowedConferenceGroupIDs []string            `json:"allowed_conference_group_ids"`
	ParticipantID           string                `json:"participant_id"`
	RecordingOptIn          bool                  `json:"recording_opt_in"`
	Devices                   []config.UserDevice `json:"devices"`
}

func (s *Server) handleUserCreate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.cm.ConfigReadOnly() {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "config is read-only (CONFIG_HTTP_URL)"})
		return
	}
	body, _ := io.ReadAll(r.Body)
	var req userCreateBody
	if err := json.Unmarshal(body, &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "invalid JSON"})
		return
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "invalid id"})
		return
	}
	cur := s.cm.Current()
	if userIndexByID(cur.Spec.Users, id) >= 0 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "user id already exists"})
		return
	}
	pin := strings.TrimSpace(req.ParticipantID)
	pin = strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, pin)
	if pin == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "participant_id required (digits)"})
		return
	}
	if participantIDTaken(cur.Spec.Users, pin, -1) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "participant_id already in use"})
		return
	}
	if err := config.ValidateUserDeviceList(req.Devices); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	devices := req.Devices
	if devices == nil {
		devices = []config.UserDevice{}
	}
	cur.Spec.Users = append(cur.Spec.Users, config.User{
		ID:                        id,
		DisplayName:               req.DisplayName,
		Region:                    req.Region,
		ParticipantID:             pin,
		AllowedBridgeIDs:          req.AllowedBridgeIDs,
		AllowedConferenceGroupIDs: req.AllowedConferenceGroupIDs,
		RecordingOptIn:            req.RecordingOptIn,
		Devices:                   devices,
	})
	yamlBytes, err := yaml.Marshal(&cur)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if _, err := s.cm.ApplyYAML(yamlBytes); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *Server) handleUserDelete(w http.ResponseWriter, r *http.Request, id string) {
	w.Header().Set("Content-Type", "application/json")
	if s.cm.ConfigReadOnly() {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "config is read-only (CONFIG_HTTP_URL)"})
		return
	}
	cur := s.cm.Current()
	idx := userIndexByID(cur.Spec.Users, id)
	if idx < 0 {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "user not found"})
		return
	}
	cur.Spec.Users = append(cur.Spec.Users[:idx], cur.Spec.Users[idx+1:]...)
	yamlBytes, err := yaml.Marshal(&cur)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if _, err := s.cm.ApplyYAML(yamlBytes); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func randomNumericPIN(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("invalid length")
	}
	const digits = "0123456789"
	b := make([]byte, length)
	ten := big.NewInt(10)
	for i := range b {
		n, err := rand.Int(rand.Reader, ten)
		if err != nil {
			return "", err
		}
		b[i] = digits[n.Int64()]
	}
	return string(b), nil
}

func (s *Server) handleUserResetPIN(w http.ResponseWriter, r *http.Request, id string) {
	w.Header().Set("Content-Type", "application/json")
	if s.cm.ConfigReadOnly() {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "config is read-only (CONFIG_HTTP_URL)"})
		return
	}
	cur := s.cm.Current()
	idx := userIndexByID(cur.Spec.Users, id)
	if idx < 0 {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "user not found"})
		return
	}
	var newPin string
	for attempts := 0; attempts < 50; attempts++ {
		p, err := randomNumericPIN(6)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
			return
		}
		if !participantIDTaken(cur.Spec.Users, p, idx) {
			newPin = p
			break
		}
	}
	if newPin == "" {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "could not allocate unique PIN"})
		return
	}
	u := cur.Spec.Users[idx]
	u.ParticipantID = newPin
	cur.Spec.Users[idx] = u
	yamlBytes, err := yaml.Marshal(&cur)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if _, err := s.cm.ApplyYAML(yamlBytes); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "new_pin": newPin})
}

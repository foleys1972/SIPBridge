package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"sipbridge/internal/config"
	"sipbridge/internal/sip"
	"gopkg.in/yaml.v3"
)

type Server struct {
	cfg         config.APIConfig
	sipSrv      *sip.Server
	runtimeSIP  config.SIPConfig
	rootCfg     config.RootConfig
	cm          *config.Manager
	http        *http.Server
}

func (s *Server) bridgeFromConfig(bridgeID string) (config.Bridge, bool) {
	if s.cm == nil {
		return config.Bridge{}, false
	}
	for _, b := range s.cm.Current().Spec.Bridges {
		if b.ID == bridgeID {
			return b, true
		}
	}
	return config.Bridge{}, false
}

// handleBridgesV1 routes:
//   GET  /v1/bridges              — list bridges from config + active call counts
//   GET  /v1/bridges/{id}         — bridge definition + active calls
//   GET  /v1/bridges/{id}/calls   — active calls only
//   POST /v1/bridges/{id}/calls/drop — disconnect one participant
//   POST /v1/bridges/{id}/reset   — disconnect all participants on the bridge
func (s *Server) handleBridgesV1(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/bridges")
	path = strings.Trim(path, "/")
	if path == "" {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		s.handleBridgeList(w, r)
		return
	}
	parts := strings.Split(path, "/")
	bridgeID := parts[0]
	if bridgeID == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		s.handleBridgeDetail(w, r, bridgeID)
		return
	}

	if len(parts) == 2 && parts[1] == "reset" {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if _, ok := s.bridgeFromConfig(bridgeID); !ok {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "bridge not found"})
			return
		}
		err := s.sipSrv.ResetBridge(bridgeID)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		return
	}

	if len(parts) >= 2 && parts[1] == "calls" {
		if len(parts) == 2 {
			if r.Method != http.MethodGet {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"bridge_id": bridgeID, "calls": s.sipSrv.ListBridgeCalls(bridgeID)})
			return
		}
		if len(parts) == 3 && parts[2] == "drop" {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			var req struct {
				CallID  string `json:"call_id"`
				FromTag string `json:"from_tag"`
			}
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &req)
			if strings.TrimSpace(req.CallID) == "" || strings.TrimSpace(req.FromTag) == "" {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "missing call_id/from_tag"})
				return
			}
			err := s.sipSrv.DropBridgeCall(bridgeID, req.CallID, req.FromTag)
			w.Header().Set("Content-Type", "application/json")
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
}

func (s *Server) handleBridgeList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.cm == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"bridges": []any{}})
		return
	}
	bridges := s.cm.Current().Spec.Bridges
	out := make([]map[string]any, 0, len(bridges))
	for _, b := range bridges {
		calls := s.sipSrv.ListBridgeCalls(b.ID)
		out = append(out, map[string]any{
			"id":            b.ID,
			"name":          b.Name,
			"type":          b.Type,
			"active_calls":  len(calls),
			"participants":  b.Participants,
		})
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"bridges": out})
}

func (s *Server) handleBridgeDetail(w http.ResponseWriter, r *http.Request, bridgeID string) {
	b, ok := s.bridgeFromConfig(bridgeID)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "bridge not found"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"bridge": b,
		"calls":  s.sipSrv.ListBridgeCalls(bridgeID),
	})
}

func NewServer(cfg config.APIConfig, sipSrv *sip.Server, runtimeSIP config.SIPConfig, rootCfg config.RootConfig, cm *config.Manager) *Server {
	mux := http.NewServeMux()
	s := &Server{cfg: cfg, sipSrv: sipSrv, runtimeSIP: runtimeSIP, rootCfg: rootCfg, cm: cm}

	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v1/sip/stats", s.handleSIPStats)
	mux.HandleFunc("/v1/settings/sip", s.handleSIPSettings)
	mux.HandleFunc("/v1/settings/database", s.handleDatabaseSettings)
	mux.HandleFunc("/v1/settings/recording", s.handleRecordingSettings)
	mux.HandleFunc("/v1/settings/cluster", s.handleClusterSettings)
	mux.HandleFunc("/v1/config", s.handleConfig)
	mux.HandleFunc("/v1/config/status", s.handleConfigStatus)
	mux.HandleFunc("/v1/config/schema", s.handleSchema)
	mux.HandleFunc("/v1/config/validate", s.handleValidate)
	mux.HandleFunc("/v1/config/reload", s.handleReload)
	mux.HandleFunc("/v1/bridges", s.handleBridgesV1)
	mux.HandleFunc("/v1/bridges/", s.handleBridgesV1)
	mux.HandleFunc("/v1/servers", s.handleServersV1)
	mux.HandleFunc("/v1/servers/", s.handleServersV1)
	mux.HandleFunc("/v1/capacity", s.handleCapacity)
	mux.HandleFunc("/v1/cluster/summary", s.handleClusterSummary)
	mux.HandleFunc("/v1/cluster/config", s.handleClusterConfig)
	mux.HandleFunc("/v1/users", s.handleUsersV1)
	mux.HandleFunc("/v1/users/", s.handleUsersV1)
	mux.HandleFunc("/v1/mi/attendance", s.handleMIAttendance)
	mux.HandleFunc("/v1/conference-groups/usage", s.handleConferenceGroupsUsage)

	s.http = &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.Port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s
}

func (s *Server) Start(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.http.Shutdown(shutdownCtx)
	}()

	if s.http != nil {
		log.Printf("HTTP API listening on %s", s.http.Addr)
	}
	err := s.http.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (s *Server) Stop(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok": true,
	})
}

func (s *Server) handleSIPStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.sipSrv.Stats())
}

func (s *Server) handleSIPSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleSIPSettingsGet(w, r)
	case http.MethodPut:
		s.handleSIPSettingsPut(w, r)
	case http.MethodDelete:
		s.handleSIPSettingsDelete(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSIPSettingsGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	effective := s.sipSrv.SIPConfig()
	var saved *config.SIPStackSpec
	if s.cm != nil {
		saved = s.cm.Current().Spec.SIPStack
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"effective": effective,
		"saved":     saved,
		"note":      "effective reflects the running process; restart after saving to apply file-backed changes.",
	})
}

func (s *Server) handleSIPSettingsPut(w http.ResponseWriter, r *http.Request) {
	if s.cm == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	var req config.SIPStackSpec
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "invalid JSON"})
		return
	}
	envCfg, err := config.LoadFromEnv()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	merged := config.MergeSIPFromSpec(envCfg.SIP, &req)
	if err := config.ValidateSIPConfig(merged); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	cur := s.cm.Current()
	cur.Spec.SIPStack = &req
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
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":               true,
		"restart_required": true,
		"message":          "Restart the sipbridge process to apply SIP/SBC settings.",
	})
}

func (s *Server) handleSIPSettingsDelete(w http.ResponseWriter, r *http.Request) {
	if s.cm == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	cur := s.cm.Current()
	cur.Spec.SIPStack = nil
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
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":               true,
		"restart_required": true,
		"message":          "Removed spec.sipStack from config; restart to use environment variables only.",
	})
}

func (s *Server) handleConfigStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if s.cm == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"config_path":          "",
			"config_http_url":      "",
			"config_read_only":     false,
			"config_http_poll_sec": 0,
		})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"config_path":          s.cm.ConfigPath(),
		"config_http_url":      s.cm.ConfigHTTPURL(),
		"config_read_only":     s.cm.ConfigReadOnly(),
		"config_http_poll_sec": s.cm.ConfigHTTPPollSeconds(),
	})
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut {
		s.handleConfigPut(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if s.cm != nil {
		_ = json.NewEncoder(w).Encode(s.cm.Current())
		return
	}
	_ = json.NewEncoder(w).Encode(s.rootCfg)
}

func (s *Server) handleConfigPut(w http.ResponseWriter, r *http.Request) {
	if s.cm == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	body, _ := io.ReadAll(r.Body)
	root, err := s.cm.ApplyYAML(body)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "config": root})
}

func (s *Server) handleSchema(w http.ResponseWriter, r *http.Request) {
	if s.cm == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	b, err := s.cm.SchemaBytes()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/schema+json")
	_, _ = w.Write(b)
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	if s.cm == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	body, _ := io.ReadAll(r.Body)
	err := s.cm.ValidateYAML(body)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	if s.cm == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	root, err := s.cm.LoadFromFile()
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "config": root})
}

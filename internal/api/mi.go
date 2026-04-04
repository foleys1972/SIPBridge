package api

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleMIAttendance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	rows := s.sipSrv.ListMIAttendance()
	_ = json.NewEncoder(w).Encode(map[string]any{"attendance": rows})
}

package api

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleConferenceGroupsUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	sessions := s.sipSrv.ListConferenceGroupUsage()
	byGroup := make(map[string]int)
	for _, x := range sessions {
		if gid := x.GroupID; gid != "" {
			byGroup[gid]++
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"sessions": sessions,
		"by_group": byGroup,
	})
}

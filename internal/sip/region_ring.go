package sip

import (
	"sort"
	"strings"

	"sipbridge/internal/config"
)

// sortEndpointsByPreferredRegion moves endpoints whose Location matches preferred first (case-insensitive).
func sortEndpointsByPreferredRegion(ring []config.Endpoint, preferred string) {
	preferred = strings.TrimSpace(preferred)
	if preferred == "" || len(ring) <= 1 {
		return
	}
	p := strings.ToLower(preferred)
	sort.SliceStable(ring, func(i, j int) bool {
		li := strings.ToLower(strings.TrimSpace(ring[i].Location))
		lj := strings.ToLower(strings.TrimSpace(ring[j].Location))
		mi := li != "" && li == p
		mj := lj != "" && lj == p
		if mi == mj {
			return false
		}
		return mi && !mj
	})
}

func userRegionForParticipant(cfg config.RootConfig, participantID string) string {
	id := strings.TrimSpace(participantID)
	if id == "" {
		return ""
	}
	for _, u := range cfg.Spec.Users {
		if strings.TrimSpace(u.ParticipantID) == id {
			return strings.TrimSpace(u.Region)
		}
	}
	return ""
}

func userRegionForUserID(cfg config.RootConfig, userOrParticipantID string) string {
	key := strings.TrimSpace(userOrParticipantID)
	if key == "" {
		return ""
	}
	for _, u := range cfg.Spec.Users {
		if strings.TrimSpace(u.ID) == key {
			return strings.TrimSpace(u.Region)
		}
	}
	return ""
}

func preferredRegionForBridgeCaller(cfg config.RootConfig, bridge config.Bridge, fromUser string) string {
	if fromUser == "" {
		return ""
	}
	for _, p := range bridge.Participants {
		if p.SIPURI == "" {
			continue
		}
		if ExtractUserFromURI(p.SIPURI) != fromUser {
			continue
		}
		if loc := strings.TrimSpace(p.Location); loc != "" {
			return loc
		}
		if r := userRegionForUserID(cfg, p.ID); r != "" {
			return r
		}
	}
	return ""
}

func preferredRegionForConferenceCaller(cfg config.RootConfig, g config.ConferenceGroup, fromUser string) string {
	if fromUser == "" {
		return ""
	}
	for _, ep := range g.SideA {
		if ep.SIPURI == "" {
			continue
		}
		if ExtractUserFromURI(ep.SIPURI) != fromUser {
			continue
		}
		if loc := strings.TrimSpace(ep.Location); loc != "" {
			return loc
		}
		if r := userRegionForUserID(cfg, ep.ID); r != "" {
			return r
		}
	}
	for _, ep := range g.SideB {
		if ep.SIPURI == "" {
			continue
		}
		if ExtractUserFromURI(ep.SIPURI) != fromUser {
			continue
		}
		if loc := strings.TrimSpace(ep.Location); loc != "" {
			return loc
		}
		if r := userRegionForUserID(cfg, ep.ID); r != "" {
			return r
		}
	}
	return ""
}

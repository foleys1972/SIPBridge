package sip

import (
	"strings"

	"sipbridge/internal/config"
)

type Router struct {
	getCfg func() config.RootConfig
}

func NewRouter(getCfg func() config.RootConfig) *Router {
	return &Router{getCfg: getCfg}
}

type InviteTargetKind string

const (
	InviteTargetKindBridge          InviteTargetKind = "bridge"
	InviteTargetKindConferenceGroup InviteTargetKind = "conferenceGroup"
	InviteTargetKindIVR             InviteTargetKind = "ivr"
)

type InviteTarget struct {
	Kind InviteTargetKind

	Bridge config.Bridge
	Group  config.ConferenceGroup
}

func (r *Router) CurrentConfig() config.RootConfig {
	if r == nil || r.getCfg == nil {
		return config.RootConfig{}
	}
	return r.getCfg()
}

func (r *Router) MatchInvite(msg *Message) (t InviteTarget, ok bool) {
	cfg := config.RootConfig{}
	if r.getCfg != nil {
		cfg = r.getCfg()
	}
	user := ExtractUserFromURI(msg.RequestURI)
	if user == "" {
		return InviteTarget{}, false
	}
	ivrUser := strings.TrimSpace(cfg.Spec.IVR.EntryUser)
	if ivrUser != "" && user == ivrUser {
		return InviteTarget{Kind: InviteTargetKindIVR}, true
	}
	for _, rt := range cfg.Spec.Routes {
		if rt.MatchUser == user {
			kind := strings.TrimSpace(rt.TargetKind)
			if kind == "" {
				kind = string(InviteTargetKindBridge)
			}
			switch kind {
			case string(InviteTargetKindConferenceGroup):
				g, ok := conferenceGroupByID(cfg, rt.TargetID)
				if !ok {
					return InviteTarget{}, false
				}
				return InviteTarget{Kind: InviteTargetKindConferenceGroup, Group: g}, true
			case string(InviteTargetKindBridge):
				fallthrough
			default:
				b, ok := bridgeByID(cfg, rt.TargetID)
				if !ok {
					return InviteTarget{}, false
				}
				return InviteTarget{Kind: InviteTargetKindBridge, Bridge: b}, true
			case string(InviteTargetKindIVR):
				return InviteTarget{Kind: InviteTargetKindIVR}, true
			}
		}
	}
	return InviteTarget{}, false
}

func bridgeByID(cfg config.RootConfig, id string) (config.Bridge, bool) {
	for _, b := range cfg.Spec.Bridges {
		if b.ID == id {
			return b, true
		}
	}
	return config.Bridge{}, false
}

func conferenceGroupByID(cfg config.RootConfig, id string) (config.ConferenceGroup, bool) {
	for _, g := range cfg.Spec.ConferenceGroups {
		if g.ID == id {
			return g, true
		}
	}
	return config.ConferenceGroup{}, false
}

func ExtractUserFromURI(uri string) string {
	// Very small parser: sip:user@host;params or sips:user@...
	u := strings.TrimSpace(uri)
	lower := strings.ToLower(u)
	if strings.HasPrefix(lower, "sip:") {
		u = u[4:]
	} else if strings.HasPrefix(lower, "sips:") {
		u = u[5:]
	}
	if i := strings.IndexAny(u, "@;"); i >= 0 {
		u = u[:i]
	}
	u = strings.Trim(u, "<>")
	return strings.TrimSpace(u)
}

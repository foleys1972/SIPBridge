package domain

type BridgeType string

const (
	BridgeTypeMRD  BridgeType = "mrd"
	BridgeTypeARD  BridgeType = "ard"
	BridgeTypeHoot BridgeType = "hoot"
)

type LineState string

const (
	LineStateIdle       LineState = "idle"
	LineStateRinging    LineState = "ringing"
	LineStateInBridge   LineState = "in_bridge"
	LineStateFailed     LineState = "failed"
	LineStateRecovering LineState = "recovering"
)

type ParticipantRole string

const (
	ParticipantRoleTrader   ParticipantRole = "trader"
	ParticipantRoleListener ParticipantRole = "listener"
	ParticipantRoleMixer    ParticipantRole = "mixer"
)

type Bridge struct {
	ID           string
	Name         string
	Type         BridgeType
	Participants []Participant
}

type Participant struct {
	ID          string
	DisplayName string
	SIPURI      string
	Role        ParticipantRole
	Location    string
}

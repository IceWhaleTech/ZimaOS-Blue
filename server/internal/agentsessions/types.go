package agentsessions

import publicagentcore "github.com/IceWhaleTech/ZimaOS-Blue/server/agentcore"

type ProtocolKind = publicagentcore.ProtocolKind

const (
	ProtocolACP ProtocolKind = publicagentcore.ProtocolACP
	ProtocolA2A ProtocolKind = publicagentcore.ProtocolA2A
)

func ParseProtocolKind(raw string) (ProtocolKind, error) {
	return publicagentcore.ParseProtocolKind(raw)
}

type SessionStatus = publicagentcore.SessionStatus

const (
	SessionStatusCreating   SessionStatus = publicagentcore.SessionStatusCreating
	SessionStatusIdle       SessionStatus = publicagentcore.SessionStatusIdle
	SessionStatusRunning    SessionStatus = publicagentcore.SessionStatusRunning
	SessionStatusCancelling SessionStatus = publicagentcore.SessionStatusCancelling
	SessionStatusClosed     SessionStatus = publicagentcore.SessionStatusClosed
	SessionStatusError      SessionStatus = publicagentcore.SessionStatusError
)

type RunStatus = publicagentcore.RunStatus

const (
	RunStatusQueued    RunStatus = publicagentcore.RunStatusQueued
	RunStatusRunning   RunStatus = publicagentcore.RunStatusRunning
	RunStatusCompleted RunStatus = publicagentcore.RunStatusCompleted
	RunStatusFailed    RunStatus = publicagentcore.RunStatusFailed
	RunStatusCancelled RunStatus = publicagentcore.RunStatusCancelled
)

var (
	ErrUnsupportedProtocol = publicagentcore.ErrUnsupportedProtocol
	ErrProfileNotFound     = publicagentcore.ErrProfileNotFound
	ErrSessionNotFound     = publicagentcore.ErrSessionNotFound
	ErrRunNotFound         = publicagentcore.ErrRunNotFound
)

const (
	ProfileVerifyMessageCodeProfileVerified = publicagentcore.ProfileVerifyMessageCodeProfileVerified
	ProfileHealthMessageCodeRuntimeHealthy  = publicagentcore.ProfileHealthMessageCodeRuntimeHealthy
)

type AgentProfile = publicagentcore.AgentProfile
type ExternalSession = publicagentcore.ExternalSession
type ExternalRun = publicagentcore.ExternalRun
type RunEvent = publicagentcore.RunEvent
type SessionHistoryItem = publicagentcore.SessionHistoryItem
type ProfileVerifyResult = publicagentcore.ProfileVerifyResult
type ProfileHealthResult = publicagentcore.ProfileHealthResult
type EnsureSessionRequest = publicagentcore.EnsureSessionRequest
type EnsureSessionResult = publicagentcore.EnsureSessionResult
type SubmitRunRequest = publicagentcore.SubmitRunRequest
type SubmitRunResult = publicagentcore.SubmitRunResult
type StreamRunRequest = publicagentcore.StreamRunRequest
type RuntimeEvent = publicagentcore.RuntimeEvent
type RuntimeStream = publicagentcore.RuntimeStream
type ProtocolRuntime = publicagentcore.ProtocolAdapter
type ClientAuthority = publicagentcore.ClientAuthority

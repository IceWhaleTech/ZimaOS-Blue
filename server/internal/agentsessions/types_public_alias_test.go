package agentsessions

import (
	"reflect"
	"testing"

	publicagentcore "github.com/IceWhaleTech/ZimaOS-Blue/server/agentcore"
)

func TestAgentSessionTypesUsePublicAgentcoreSourceOfTruth(t *testing.T) {
	if got, want := reflect.TypeOf(ProtocolKind("")).PkgPath(), reflect.TypeOf(publicagentcore.ProtocolKind("")).PkgPath(); got != want {
		t.Fatalf("ProtocolKind pkg path = %q, want %q", got, want)
	}
	if got, want := reflect.TypeOf(AgentProfile{}).PkgPath(), reflect.TypeOf(publicagentcore.AgentProfile{}).PkgPath(); got != want {
		t.Fatalf("AgentProfile pkg path = %q, want %q", got, want)
	}
	if got, want := reflect.TypeOf((*ProtocolRuntime)(nil)).Elem().PkgPath(), reflect.TypeOf((*publicagentcore.ProtocolAdapter)(nil)).Elem().PkgPath(); got != want {
		t.Fatalf("ProtocolRuntime pkg path = %q, want %q", got, want)
	}
	if ErrUnsupportedProtocol != publicagentcore.ErrUnsupportedProtocol {
		t.Fatal("ErrUnsupportedProtocol should alias public agentcore error")
	}
}

package api

import (
	"testing"

	blueServer "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

func TestResolveListeningPort_PrefersActualPort(t *testing.T) {
	originalPort := blueServer.GetActualPort()
	blueServer.SetActualPort(19091)
	t.Cleanup(func() {
		blueServer.SetActualPort(originalPort)
	})

	if got := resolveListeningPort(8080); got != 19091 {
		t.Fatalf("resolveListeningPort() = %d, want %d", got, 19091)
	}
}

func TestResolveListeningPort_FallsBackToConfiguredPort(t *testing.T) {
	originalPort := blueServer.GetActualPort()
	blueServer.SetActualPort(0)
	t.Cleanup(func() {
		blueServer.SetActualPort(originalPort)
	})

	if got := resolveListeningPort(8080); got != 8080 {
		t.Fatalf("resolveListeningPort() = %d, want %d", got, 8080)
	}
}

package bootstrap

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"go.uber.org/zap"
)

func TestBindRuntimeDreamCronCreatesSystemJobWhenEnabled(t *testing.T) {
	dream, err := memory.NewDreamService(nil, t.TempDir(), memory.DreamConfig{
		Enabled:    true,
		ArchiveDir: t.TempDir(),
		Schedule:   "0 30 3 * * *",
	})
	if err != nil {
		t.Fatalf("NewDreamService: %v", err)
	}

	handler := cron.NewLazyHandler(func() *cron.Service {
		svc := cron.NewService(cron.DefaultConfig(), zap.NewNop())
		if err := svc.Start(); err != nil {
			t.Fatalf("Start cron service: %v", err)
		}
		return svc
	}, zap.NewNop())

	BindRuntimeDreamCron(handler, dream)
	svc := handler.GetService()
	if svc == nil {
		t.Fatal("GetService() returned nil")
	}

	jobs := svc.List()
	if len(jobs) != 1 {
		t.Fatalf("job count = %d, want 1", len(jobs))
	}
	if jobs[0].Schedule != "0 30 3 * * *" {
		t.Fatalf("job schedule = %q, want %q", jobs[0].Schedule, "0 30 3 * * *")
	}
	if jobs[0].Handler != dreamCronHandlerName {
		t.Fatalf("job handler = %q, want %q", jobs[0].Handler, dreamCronHandlerName)
	}
}

func TestBindRuntimeDreamCronSkipsDisabledDream(t *testing.T) {
	dream, err := memory.NewDreamService(nil, t.TempDir(), memory.DreamConfig{
		Enabled:    false,
		ArchiveDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NewDreamService: %v", err)
	}

	handler := cron.NewLazyHandler(func() *cron.Service {
		svc := cron.NewService(cron.DefaultConfig(), zap.NewNop())
		if err := svc.Start(); err != nil {
			t.Fatalf("Start cron service: %v", err)
		}
		return svc
	}, zap.NewNop())

	BindRuntimeDreamCron(handler, dream)
	svc := handler.GetService()
	if svc == nil {
		t.Fatal("GetService() returned nil")
	}
	if handlers := svc.Handlers(); len(handlers) != 0 {
		t.Fatalf("handlers = %v, want empty", handlers)
	}
	if jobs := svc.List(); len(jobs) != 0 {
		t.Fatalf("job count = %d, want 0", len(jobs))
	}
}

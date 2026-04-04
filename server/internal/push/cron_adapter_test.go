package push

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"go.uber.org/zap"
)

func TestSetCronWithLazyAdapterDoesNotResolveService(t *testing.T) {
	var resolveCalls int

	adapter := NewCronAdapter(func() *cron.Service {
		resolveCalls++
		svc := cron.NewService(cron.DefaultConfig(), zap.NewNop())
		svc.RegisterHandler("push", func(ctx context.Context, job *cron.Job) (interface{}, error) {
			return job.ID, nil
		})
		return svc
	})

	svc, _ := testService(t)
	svc.SetCron(adapter)

	if resolveCalls != 0 {
		t.Fatalf("resolveCalls after SetCron = %d, want 0", resolveCalls)
	}
}

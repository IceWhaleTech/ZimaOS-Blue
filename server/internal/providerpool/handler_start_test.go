package providerpool

import (
	"context"
	"testing"
	"time"
)

type stubStorage struct{}

func (stubStorage) SaveProvider(*Provider) error           { return nil }
func (stubStorage) LoadProvider(string) (*Provider, error) { return nil, ErrProviderNotFound }
func (stubStorage) LoadAllProviders() ([]*Provider, error) { return nil, nil }
func (stubStorage) DeleteProvider(string) error            { return nil }
func (stubStorage) SaveModels(string, []*Model) error      { return nil }
func (stubStorage) LoadModels(string) ([]*Model, error)    { return nil, nil }
func (stubStorage) AppendUsage(*UsageRecord) error         { return nil }
func (stubStorage) LoadUsage(string, time.Time, time.Time) ([]*UsageRecord, error) {
	return nil, nil
}
func (stubStorage) SavePricingConfig(*PricingConfig) error { return nil }
func (stubStorage) LoadPricingConfig() (*PricingConfig, error) {
	return DefaultPricingConfig(), nil
}
func (stubStorage) SaveConfig(*PoolConfig) error { return nil }
func (stubStorage) LoadConfig() (*PoolConfig, error) {
	return &PoolConfig{}, nil
}

func TestPoolStartIsIdempotent(t *testing.T) {
	storage := stubStorage{}
	registry, err := NewRegistry(storage)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	pool := &Pool{
		Registry:       registry,
		Discovery:      discovery,
		Router:         router,
		UsageTracker:   NewUsageTracker(storage),
		PricingManager: nil,
		Storage:        storage,
		Config: &PoolConfig{
			UsageTrackingEnabled: false,
		},
		readyCh: make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)
	pool.Start(ctx)

	if !pool.WaitReady(2 * time.Second) {
		t.Fatal("pool did not become ready after repeated Start calls")
	}
}

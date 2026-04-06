package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/embedding"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skilladvisor"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmarket"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillstore"
	"go.uber.org/zap"
)

func bindRouteRuntimeSkillMarketplace(
	handler *serverpkg.SkillHandler,
	localScanner *skillstore.LocalSkillScanner,
	options routeRuntimeContractSkillOptions,
	ctx context.Context,
) {
	if handler == nil || localScanner == nil || options.services == nil {
		return
	}

	dataDir := strings.TrimSpace(options.dataDir)
	marketCfg := skillmarket.DefaultConfig(dataDir, filepath.Join(dataDir, "workspace", ".claude", "skills"))
	marketCfg.GitHubToken = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	if options.appConfig != nil {
		marketCfg.DiscoveryPageURLs = append([]string{}, options.appConfig.SkillMarket.DiscoveryPageURLs...)
		marketCfg.ClawHubMirrorBaseURLs = append([]string{}, options.appConfig.SkillMarket.ClawHubMirrorBaseURLs...)
		marketCfg.TencentSkillHubAPIBaseURL = options.appConfig.SkillMarket.TencentSkillHubAPIBaseURL
		marketCfg.SkillHubBaseURL = options.appConfig.SkillMarket.SkillHubBaseURL
		marketCfg.SkillHubAPIKey = strings.TrimSpace(options.appConfig.SkillMarket.SkillHubAPIKey)
		marketCfg.LLMSkillsBaseURL = options.appConfig.SkillMarket.LLMSkillsBaseURL
		marketCfg.CuratedConfigPath = options.appConfig.SkillMarket.CuratedConfigPath
		marketCfg.CuratedConfigURLs = append([]string{}, options.appConfig.SkillMarket.CuratedConfigURLs...)
		marketCfg.CrawlIncrementalInterval = options.appConfig.SkillMarket.CrawlIncrementalInterval
		marketCfg.CrawlFullInterval = options.appConfig.SkillMarket.CrawlFullInterval
		marketCfg.UpdateCheckInterval = options.appConfig.SkillMarket.UpdateCheckInterval
		marketCfg.TelemetryRollupInterval = options.appConfig.SkillMarket.TelemetryRollupInterval
		marketCfg.SemanticRatio = options.appConfig.SkillMarket.SemanticRatio
		marketCfg.SearchCandidateLimit = options.appConfig.SkillMarket.SearchCandidateLimit
	}

	var marketEmbedding embedding.Provider
	var marketEmbeddingOnce sync.Once
	resolveMarketEmbedding := func() embedding.Provider {
		if options.appConfig == nil || !options.appConfig.Embedding.Enabled || !strings.EqualFold(options.appConfig.Embedding.Provider, "cybertron") {
			return nil
		}
		marketEmbeddingOnce.Do(func() {
			marketEmbedding = embedding.NewCybertronProvider(embedding.CybertronConfig{
				ModelsDir:  embedding.PrepareSharedModelCache(dataDir, options.appConfig.Memory.VectorStore.DBPath, options.appConfig.Embedding.Model),
				Model:      options.appConfig.Embedding.Model,
				Dimensions: options.appConfig.Embedding.Dimensions,
				Timeout:    options.appConfig.Embedding.Timeout,
			})
		})
		return marketEmbedding
	}

	handler.SetMarketplaceFactory(func() (*skillmarket.Service, error) {
		market, err := skillmarket.NewServiceWithDBPath(marketCfg.DBPath, skillmarket.Options{
			Config:            marketCfg,
			Logger:            options.logger,
			Registry:          options.services.SkillRegistry,
			LocalScanner:      localScanner,
			EmbeddingProvider: resolveMarketEmbedding(),
			HTTPClient:        network.NewPooledHTTPClient(5 * time.Minute),
			RemoteReader:      newSkillMarketRemoteReader(options.services.ToolRegistry),
			DiscoverEventBroadcaster: func(eventType string, data any) {
				if options.eventBroker != nil {
					options.eventBroker.Broadcast(eventType, data)
				}
			},
			Scanner: skillmarket.NewScanner(nil),
		})
		if err != nil {
			if options.logger != nil {
				options.logger.Warn("Failed to initialize skill marketplace", zap.Error(err), zap.String("db_path", marketCfg.DBPath))
			}
			return nil, err
		}
		market.Start(ctx)
		if options.logger != nil {
			options.logger.Info("Skill marketplace initialized", zap.String("db_path", marketCfg.DBPath))
		}
		return market, nil
	})

	advisor := skilladvisor.NewService(skilladvisor.SearchFunc(handler.SearchMarket))
	if options.settings != nil {
		options.settings.SetSkillAdvisor(advisor)
	}
	handler.SetSkillAdvisor(advisor)
	if options.chat != nil {
		handler.SetSkillSelector(options.chat.GetSkillSelector())
	}
	if options.settings != nil {
		handler.SetSkillSelectorOptionsProvider(func() agentcore.SelectOptions {
			return agentcore.SelectOptions{
				Mode:                options.settings.GetSkillSelectorMode(),
				EnableRerank:        options.settings.GetEffectiveSkillRerankEnabled(),
				ConfidenceThreshold: options.settings.GetSkillSelectorConfidenceThreshold(),
			}
		})
	}
}

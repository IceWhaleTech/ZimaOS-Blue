package bootstrap

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	networkapi "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/api"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/connection"
	convertsvc "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool/oauth"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/update"
	"github.com/labstack/echo/v4"
)

type stubPermissionRouteRegistrar struct{}

func (s *stubPermissionRouteRegistrar) RegisterCurrentUserRoutes(*echo.Group) {}

func (s *stubPermissionRouteRegistrar) RegisterAdminRoutes(*echo.Group) {}

func TestRouteRegistrationStateRuntimeSetters_PersistFieldAndSnapshot(t *testing.T) {
	state := &routeRegistrationState{deps: &RoutesDeps{}}
	e := echo.New()
	protected := e.Group("/api")
	apiProtected := e.Group("/api/v1")
	authPageGroup := e.Group("/api/auth")
	authMiddleware := func(next echo.HandlerFunc) echo.HandlerFunc { return next }
	authHandler := &stubPermissionRouteRegistrar{}
	connManager := new(connection.Manager)
	authSurface := routeRuntimeContractAuthSurfaceResult{
		protected:        protected,
		apiProtected:     apiProtected,
		authPageV1Group:  func(string) *echo.Group { return authPageGroup },
		authPageAPIGroup: func(string) *echo.Group { return authPageGroup },
		requirePagePermission: func(string) echo.MiddlewareFunc {
			return authMiddleware
		},
		authMiddleware:    authMiddleware,
		permissionHandler: authHandler,
	}

	bootstrap := routeRuntimeContractBootstrapSupportResult{
		settingsHandler:   new(serverpkg.SettingsHandler),
		smallModelManager: new(smallmodel.Manager),
		approvalHandler:   new(networkapi.ApprovalHandler),
	}
	startupAuth := routeRuntimeContractStartupAuthResult{
		startupBound: true,
		auth:         authSurface,
		accountBound: true,
	}
	bootstrapPhase := routeRuntimeContractBootstrapPhaseResult{
		mediaDir:            "/tmp/media",
		shellBound:          true,
		bootstrap:           bootstrap,
		earlyReadyTriggered: true,
	}
	media := routeRuntimeContractMediaResult{ipcServer: new(sockipc.Server)}
	gateway := routeRuntimeContractGatewayResult{routesRegistered: true}
	infrastructure := routeRuntimeContractInfrastructureResult{
		tlsConfigured:    true,
		media:            media,
		gateway:          gateway,
		chatSurfaceBound: true,
	}
	chat := routeRuntimeContractChatBindingResult{autoHarnessHookBound: true}
	skill := routeRuntimeContractSkillResult{routesRegistered: true}
	reranker := new(agentcore.AutoSkillReranker)
	experience := routeRuntimeContractExperienceResult{
		skillAutoReranker: reranker,
		chat:              chat,
		skill:             skill,
	}
	coreTooling := routeRuntimeContractCoreToolingResult{
		schedulerBound: true,
		tooling: routeRuntimeContractToolingResult{
			uiReviewerTool: new(tools.UIReviewerTool),
		},
		analyzeTool: new(tools.AnalyzeTool),
	}
	ask := runtimeAskSupportBundle{
		QuestionManager:          new(tools.QuestionManager),
		BrowserCheckpointManager: new(tools.BrowserCheckpointManager),
		BrowserSiteStore:         new(tools.BrowserSiteAllowlistStore),
	}
	exec := runtimeExecSupportBundle{
		Approvals:      new(tools.ApprovalManager),
		DirStore:       new(tools.DirAllowlistStore),
		AuditStore:     new(tools.ExecAuditStore),
		ConvertHandler: new(convertsvc.Handler),
	}
	coreSupport := routeRuntimeContractCoreSupportResult{
		ask:        ask,
		exec:       exec,
		capability: routeRuntimeContractCapabilitySupportResult{convertRegistered: true},
	}
	provider := routeRuntimeContractProviderPoolResult{oauthManager: new(oauth.LazyManager)}
	coreTooling.provider = provider
	management := routeRuntimeContractManagementSupportResult{
		updateHandler:    new(update.Handler),
		otaChecker:       new(update.OTAChecker),
		providerSettings: new(serverpkg.ProviderSettingsHandler),
	}
	mgmtTool := new(tools.MgmtTool)
	managementRuntime := routeRuntimeContractManagementRuntimeResult{
		mgmtTool:         mgmtTool,
		heartbeatBound:   true,
		support:          management,
		userSurfaceBound: true,
		upgradeBound:     true,
		channelBound:     true,
	}
	operational := routeRuntimeContractOperationalResult{
		taskSurface: runtimeTaskSurfaceRegistration{
			research: runtimeTaskResearchSurfaceRegistration{
				deepResearchRegistered:    true,
				harnessResearchRegistered: true,
			},
		},
	}

	state.setHTTPBootstrap(connManager, e.Group("/api/v1"), e.Group("/api"))
	state.setStartupAuthRuntime(startupAuth)
	state.setBootstrapPhaseRuntime(bootstrapPhase)
	state.setInfrastructureRuntime(infrastructure)
	state.setExperienceRuntime(experience)
	state.setCoreToolingRuntime(coreTooling)
	state.setCoreSupportRuntime(coreSupport)
	state.setManagementRuntime(managementRuntime)
	state.setOperationalRuntime(operational)

	if state.connManager != connManager || state.runtimeSnapshot.entry.connManager != connManager {
		t.Fatalf("expected http bootstrap snapshot to mirror connection manager, field=%#v snapshot=%#v", state.connManager, state.runtimeSnapshot.entry.connManager)
	}
	if state.v1 == nil || state.runtimeSnapshot.entry.v1 == nil || state.api == nil || state.runtimeSnapshot.entry.api == nil {
		t.Fatalf("expected http bootstrap snapshot to retain v1/api groups, state=%#v snapshot=%#v", state.v1, state.runtimeSnapshot.entry)
	}
	if state.startupAuthRuntime.startupBound != startupAuth.startupBound || state.runtimeSnapshot.startupAuth.startupBound != startupAuth.startupBound {
		t.Fatalf("expected startup/auth runtime startup flag snapshot to mirror field, field=%#v snapshot=%#v", state.startupAuthRuntime.startupBound, state.runtimeSnapshot.startupAuth.startupBound)
	}
	if state.startupAuthRuntime.accountBound != startupAuth.accountBound || state.runtimeSnapshot.startupAuth.accountBound != startupAuth.accountBound {
		t.Fatalf("expected startup/auth runtime account flag snapshot to mirror field, field=%#v snapshot=%#v", state.startupAuthRuntime.accountBound, state.runtimeSnapshot.startupAuth.accountBound)
	}
	if state.startupAuthRuntime.auth.protected != protected || state.runtimeSnapshot.startupAuth.auth.protected != protected {
		t.Fatalf("expected startup/auth runtime auth projection snapshot to mirror field, field=%#v snapshot=%#v", state.startupAuthRuntime.auth.protected, state.runtimeSnapshot.startupAuth.auth.protected)
	}
	if state.mediaDir != "/tmp/media" || state.runtimeSnapshot.entry.mediaDir != "/tmp/media" {
		t.Fatalf("expected media dir snapshot to mirror field, field=%q snapshot=%q", state.mediaDir, state.runtimeSnapshot.entry.mediaDir)
	}
	if state.skillAutoReranker != reranker || state.runtimeSnapshot.utility.skillAutoReranker != reranker {
		t.Fatalf("expected skill auto reranker snapshot to mirror field, field=%#v snapshot=%#v", state.skillAutoReranker, state.runtimeSnapshot.utility.skillAutoReranker)
	}
	if state.authSurface.protected != protected || state.runtimeSnapshot.auth.protected != protected {
		t.Fatalf("expected auth surface protected group snapshot to mirror field, field=%#v snapshot=%#v", state.authSurface.protected, state.runtimeSnapshot.auth.protected)
	}
	if state.protected != protected || state.apiProtected != apiProtected {
		t.Fatalf("expected auth setter to project protected groups onto state, protected=%#v apiProtected=%#v", state.protected, state.apiProtected)
	}
	fieldHandler, ok := state.authSurface.permissionHandler.(*stubPermissionRouteRegistrar)
	if !ok || fieldHandler != authHandler {
		t.Fatalf("expected auth surface permission handler to mirror field, field=%#v want=%#v", state.authSurface.permissionHandler, authHandler)
	}
	snapshotHandler, ok := state.runtimeSnapshot.auth.permissionHandler.(*stubPermissionRouteRegistrar)
	if !ok || snapshotHandler != authHandler {
		t.Fatalf("expected auth surface permission handler snapshot to mirror field, snapshot=%#v want=%#v", state.runtimeSnapshot.auth.permissionHandler, authHandler)
	}
	if state.authSurface.authPageV1Group == nil || state.runtimeSnapshot.auth.authPageV1Group == nil {
		t.Fatalf("expected auth surface page-group resolver to remain populated, field=%t snapshot=%t", state.authSurface.authPageV1Group != nil, state.runtimeSnapshot.auth.authPageV1Group != nil)
	}
	if state.authSurface.requirePagePermission == nil || state.runtimeSnapshot.auth.requirePagePermission == nil {
		t.Fatalf("expected auth surface permission middleware resolver to remain populated, field=%t snapshot=%t", state.authSurface.requirePagePermission != nil, state.runtimeSnapshot.auth.requirePagePermission != nil)
	}
	if state.bootstrapPhaseRuntime != bootstrapPhase || state.runtimeSnapshot.bootstrapPhase != bootstrapPhase {
		t.Fatalf("expected bootstrap phase runtime snapshot to mirror field, field=%#v snapshot=%#v", state.bootstrapPhaseRuntime, state.runtimeSnapshot.bootstrapPhase)
	}
	if state.bootstrapSupport != bootstrap || state.runtimeSnapshot.bootstrap != bootstrap {
		t.Fatalf("expected bootstrap support snapshot to mirror field, field=%#v snapshot=%#v", state.bootstrapSupport, state.runtimeSnapshot.bootstrap)
	}
	if state.infrastructureRuntime != infrastructure || state.runtimeSnapshot.infrastructure != infrastructure {
		t.Fatalf("expected infrastructure runtime snapshot to mirror field, field=%#v snapshot=%#v", state.infrastructureRuntime, state.runtimeSnapshot.infrastructure)
	}
	if state.mediaRuntime != media || state.runtimeSnapshot.media != media {
		t.Fatalf("expected media runtime snapshot to mirror field, field=%#v snapshot=%#v", state.mediaRuntime, state.runtimeSnapshot.media)
	}
	if state.gatewayRuntime != gateway || state.runtimeSnapshot.gateway != gateway {
		t.Fatalf("expected gateway runtime snapshot to mirror field, field=%#v snapshot=%#v", state.gatewayRuntime, state.runtimeSnapshot.gateway)
	}
	if state.experienceRuntime != experience || state.runtimeSnapshot.experience != experience {
		t.Fatalf("expected experience runtime snapshot to mirror field, field=%#v snapshot=%#v", state.experienceRuntime, state.runtimeSnapshot.experience)
	}
	if state.coreToolingRuntime != coreTooling || state.runtimeSnapshot.coreTooling != coreTooling {
		t.Fatalf("expected core tooling runtime snapshot to mirror field, field=%#v snapshot=%#v", state.coreToolingRuntime, state.runtimeSnapshot.coreTooling)
	}
	if state.deps.UIReviewerTool != coreTooling.tooling.uiReviewerTool || state.deps.AnalyzeTool != coreTooling.analyzeTool {
		t.Fatalf("expected core tooling runtime to project ui/analyze tools onto deps, ui=%#v analyze=%#v", state.deps.UIReviewerTool, state.deps.AnalyzeTool)
	}
	if state.coreSupportRuntime.capability != coreSupport.capability || state.runtimeSnapshot.coreSupport.capability != coreSupport.capability {
		t.Fatalf("expected core support runtime capability snapshot to mirror field, field=%#v snapshot=%#v", state.coreSupportRuntime.capability, state.runtimeSnapshot.coreSupport.capability)
	}
	if state.coreSupportRuntime.ask != ask || state.runtimeSnapshot.coreSupport.ask != ask {
		t.Fatalf("expected core support runtime ask snapshot to mirror field, field=%#v snapshot=%#v", state.coreSupportRuntime.ask, state.runtimeSnapshot.coreSupport.ask)
	}
	if state.coreSupportRuntime.exec.Approvals != exec.Approvals || state.runtimeSnapshot.coreSupport.exec.Approvals != exec.Approvals {
		t.Fatalf("expected core support runtime exec approvals snapshot to mirror field, field=%#v snapshot=%#v", state.coreSupportRuntime.exec.Approvals, state.runtimeSnapshot.coreSupport.exec.Approvals)
	}
	if state.chatRuntime != chat || state.runtimeSnapshot.chat != chat {
		t.Fatalf("expected chat runtime snapshot to mirror field, field=%#v snapshot=%#v", state.chatRuntime, state.runtimeSnapshot.chat)
	}
	if state.skillRuntime != skill || state.runtimeSnapshot.skill != skill {
		t.Fatalf("expected skill runtime snapshot to mirror field, field=%#v snapshot=%#v", state.skillRuntime, state.runtimeSnapshot.skill)
	}
	if state.askSupport != ask || state.runtimeSnapshot.ask != ask {
		t.Fatalf("expected ask support snapshot to mirror field, field=%#v snapshot=%#v", state.askSupport, state.runtimeSnapshot.ask)
	}
	if state.questionMgr != ask.QuestionManager || state.runtimeSnapshot.utility.questionMgr != ask.QuestionManager {
		t.Fatalf("expected question manager snapshot to mirror field, field=%#v snapshot=%#v", state.questionMgr, state.runtimeSnapshot.utility.questionMgr)
	}
	if state.execSupport.Approvals != exec.Approvals || state.runtimeSnapshot.exec.Approvals != exec.Approvals {
		t.Fatalf("expected exec approvals snapshot to mirror field, field=%#v snapshot=%#v", state.execSupport.Approvals, state.runtimeSnapshot.exec.Approvals)
	}
	if state.execApprovals != exec.Approvals || state.runtimeSnapshot.utility.execApprovals != exec.Approvals {
		t.Fatalf("expected exec approvals utility snapshot to mirror field, field=%#v snapshot=%#v", state.execApprovals, state.runtimeSnapshot.utility.execApprovals)
	}
	if state.execSupport.DirStore != exec.DirStore || state.runtimeSnapshot.exec.DirStore != exec.DirStore {
		t.Fatalf("expected exec dir store snapshot to mirror field, field=%#v snapshot=%#v", state.execSupport.DirStore, state.runtimeSnapshot.exec.DirStore)
	}
	if state.execSupport.AuditStore != exec.AuditStore || state.runtimeSnapshot.exec.AuditStore != exec.AuditStore {
		t.Fatalf("expected exec audit snapshot to mirror field, field=%#v snapshot=%#v", state.execSupport.AuditStore, state.runtimeSnapshot.exec.AuditStore)
	}
	if state.execSupport.ConvertHandler != exec.ConvertHandler || state.runtimeSnapshot.exec.ConvertHandler != exec.ConvertHandler {
		t.Fatalf("expected exec convert snapshot to mirror field, field=%#v snapshot=%#v", state.execSupport.ConvertHandler, state.runtimeSnapshot.exec.ConvertHandler)
	}
	if state.providerRuntime != provider || state.runtimeSnapshot.provider != provider {
		t.Fatalf("expected provider runtime snapshot to mirror field, field=%#v snapshot=%#v", state.providerRuntime, state.runtimeSnapshot.provider)
	}
	if state.managementRuntime != managementRuntime || state.runtimeSnapshot.managementRuntime != managementRuntime {
		t.Fatalf("expected management runtime snapshot to mirror field, field=%#v snapshot=%#v", state.managementRuntime, state.runtimeSnapshot.managementRuntime)
	}
	if state.managementSupport != management || state.runtimeSnapshot.management != management {
		t.Fatalf("expected management support snapshot to mirror field, field=%#v snapshot=%#v", state.managementSupport, state.runtimeSnapshot.management)
	}
	if state.mgmtTool != mgmtTool || state.runtimeSnapshot.utility.mgmtTool != mgmtTool {
		t.Fatalf("expected management tool snapshot to mirror field, field=%#v snapshot=%#v", state.mgmtTool, state.runtimeSnapshot.utility.mgmtTool)
	}
	if state.operationalRuntime != operational || state.runtimeSnapshot.operational != operational {
		t.Fatalf("expected operational runtime snapshot to mirror field, field=%#v snapshot=%#v", state.operationalRuntime, state.runtimeSnapshot.operational)
	}
}

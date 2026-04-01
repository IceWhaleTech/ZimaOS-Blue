package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"

func (binding *runtimeContractBinding) BindGatewayRuntime(options routeRuntimeContractGatewayOptions) routeRuntimeContractGatewayResult {
	if binding == nil {
		return routeRuntimeContractGatewayResult{}
	}
	return bindRouteRuntimeGateway(options)
}

func bindRouteRuntimeGateway(options routeRuntimeContractGatewayOptions) routeRuntimeContractGatewayResult {
	result := routeRuntimeContractGatewayResult{}
	if options.gateway == nil {
		return result
	}

	tools.RegisterGatewayTool(options.toolRegistry, options.gateway)
	result.toolRegistered = options.toolRegistry != nil

	if options.handler == nil || options.e == nil || options.protected == nil {
		return result
	}

	registerRouteRuntimeGatewayMethods(options.gateway, options)
	result.methodsRegistered = true
	options.handler.RegisterRoutes(options.e, options.protected)
	result.routesRegistered = true
	if options.closers != nil {
		*options.closers = append(*options.closers, gatewayStopper{gateway: options.gateway})
		result.closerRegistered = true
	}
	return result
}

func (g gatewayStopper) Close() error {
	if g.gateway != nil {
		g.gateway.Stop()
	}
	return nil
}

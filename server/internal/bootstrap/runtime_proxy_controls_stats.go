package bootstrap

import (
	"database/sql"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/metrics"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
)

func newRuntimeProxyPipelineStatsCollector(
	metricsWriter *metrics.MetricsWriter,
	fallbackWriteDB *sql.DB,
	fallbackReadDB *sql.DB,
	handler *proxy.ProxyHandler,
	smartFailover *proxy.SmartFailoverHandler,
	closers *[]interface{ Close() error },
) *proxy.PipelineStatsCollector {
	if handler == nil || smartFailover == nil {
		return nil
	}

	var collector *proxy.PipelineStatsCollector
	switch {
	case metricsWriter != nil && metricsWriter.GetDB() != nil:
		collector = proxy.NewPipelineStatsCollectorWithReadDB(
			metricsWriter.GetDB(),
			metricsWriter.GetReadDB(),
			handler.GetRoutingStatsRef(),
		)
	case fallbackWriteDB != nil && fallbackReadDB != nil:
		collector = proxy.NewPipelineStatsCollectorWithReadDB(
			fallbackWriteDB,
			fallbackReadDB,
			handler.GetRoutingStatsRef(),
		)
	case fallbackWriteDB != nil:
		collector = proxy.NewPipelineStatsCollector(
			fallbackWriteDB,
			handler.GetRoutingStatsRef(),
		)
	}
	if collector == nil {
		return nil
	}

	collector.SetSmartFailoverMetrics(smartFailover.GetMetrics())
	collector.SetFailoverHandler(smartFailover.FailoverHandler)
	collector.LoadSmartMetrics()
	collector.LoadBreakerState()
	collector.Start()
	handler.SetPipelineStats(collector)
	if closers != nil {
		*closers = append(*closers, collector)
	}
	return collector
}

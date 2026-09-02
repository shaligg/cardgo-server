package db

import (
	"context"
	"time"

	"github.com/bigfish/go_orm_1/internal/infra/metrics"
	"gorm.io/gorm/logger"
)

// metricsLogger 在 GORM 的统一 SQL 日志入口采集数据库耗时。
type metricsLogger struct {
	logger.Interface
	registry *metrics.Registry
}

func newMetricsLogger(base logger.Interface, registry *metrics.Registry) logger.Interface {
	if registry == nil {
		return base
	}
	return metricsLogger{Interface: base, registry: registry}
}

func (l metricsLogger) LogMode(level logger.LogLevel) logger.Interface {
	return metricsLogger{Interface: l.Interface.LogMode(level), registry: l.registry}
}

func (l metricsLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	l.registry.ObserveDBDuration(time.Since(begin))
	l.Interface.Trace(ctx, begin, fc, err)
}

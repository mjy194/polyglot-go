package logger

import (
	"polyglot/internal/config"
	"go.uber.org/zap"
)

// Logger 日志接口
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
	Fatal(msg string, keysAndValues ...interface{})
	Sync() error
}

// New 创建日志实例
func New(cfg config.LogConfig) Logger {
	var zapCfg zap.Config

	if cfg.Level == "debug" {
		zapCfg = zap.NewDevelopmentConfig()
	} else {
		zapCfg = zap.NewProductionConfig()
	}

	logger, _ := zapCfg.Build()
	return &zapLogger{logger: logger.Sugar()}
}

type zapLogger struct {
	logger *zap.SugaredLogger
}

func (l *zapLogger) Debug(msg string, keysAndValues ...interface{}) {
	l.logger.Debugw(msg, keysAndValues...)
}

func (l *zapLogger) Info(msg string, keysAndValues ...interface{}) {
	l.logger.Infow(msg, keysAndValues...)
}

func (l *zapLogger) Warn(msg string, keysAndValues ...interface{}) {
	l.logger.Warnw(msg, keysAndValues...)
}

func (l *zapLogger) Error(msg string, keysAndValues ...interface{}) {
	l.logger.Errorw(msg, keysAndValues...)
}

func (l *zapLogger) Fatal(msg string, keysAndValues ...interface{}) {
	l.logger.Fatalw(msg, keysAndValues...)
}

func (l *zapLogger) Sync() error {
	return l.logger.Sync()
}

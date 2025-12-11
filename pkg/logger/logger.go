package logger

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Fatal(msg string, fields ...Field)
	WithContext(ctx context.Context) Logger
	WithField(key string, value interface{}) Logger
	WithFields(fields Fields) Logger
}

type Field struct {
	Key   string
	Value interface{}
}

type Fields map[string]interface{}

type logrusLogger struct {
	logger *logrus.Logger
	entry  *logrus.Entry
}

func New(level string, format string) Logger {
	log := logrus.New()

	parsedLevel, err := logrus.ParseLevel(level)
	if err != nil {
		parsedLevel = logrus.InfoLevel
	}
	log.SetLevel(parsedLevel)

	switch format {
	case "json":
		log.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339Nano,
		})
	default:
		log.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: time.RFC3339Nano,
			FullTimestamp:   true,
		})
	}

	log.SetOutput(os.Stdout)

	return &logrusLogger{
		logger: log,
		entry:  log.WithFields(logrus.Fields{}),
	}
}

func (l *logrusLogger) Debug(msg string, fields ...Field) {
	l.withFields(fields).Debug(msg)
}

func (l *logrusLogger) Info(msg string, fields ...Field) {
	l.withFields(fields).Info(msg)
}

func (l *logrusLogger) Warn(msg string, fields ...Field) {
	l.withFields(fields).Warning(msg)
}

func (l *logrusLogger) Error(msg string, fields ...Field) {
	l.withFields(fields).Error(msg)
}

func (l *logrusLogger) Fatal(msg string, fields ...Field) {
	l.withFields(fields).Fatal(msg)
}

func (l *logrusLogger) WithContext(ctx context.Context) Logger {
	return &logrusLogger{
		logger: l.logger,
		entry:  l.entry.WithContext(ctx),
	}
}

func (l *logrusLogger) WithField(key string, value interface{}) Logger {
	return &logrusLogger{
		logger: l.logger,
		entry:  l.entry.WithField(key, value),
	}
}

func (l *logrusLogger) WithFields(fields Fields) Logger {
	logrusFields := logrus.Fields{}
	for k, v := range fields {
		logrusFields[k] = v
	}
	return &logrusLogger{
		logger: l.logger,
		entry:  l.entry.WithFields(logrusFields),
	}
}

func (l *logrusLogger) withFields(fields []Field) *logrus.Entry {
	if len(fields) == 0 {
		return l.entry
	}

	logrusFields := logrus.Fields{}
	for _, f := range fields {
		logrusFields[f.Key] = f.Value
	}
	return l.entry.WithFields(logrusFields)
}

var defaultLogger Logger

func init() {
	defaultLogger = New("info", "json")
}

func SetDefault(l Logger) {
	defaultLogger = l
}

func Default() Logger {
	return defaultLogger
}

func Debug(msg string, fields ...Field) {
	defaultLogger.Debug(msg, fields...)
}

func Info(msg string, fields ...Field) {
	defaultLogger.Info(msg, fields...)
}

func Warn(msg string, fields ...Field) {
	defaultLogger.Warn(msg, fields...)
}

func Error(msg string, fields ...Field) {
	defaultLogger.Error(msg, fields...)
}

func Fatal(msg string, fields ...Field) {
	defaultLogger.Fatal(msg, fields...)
}

func WithContext(ctx context.Context) Logger {
	return defaultLogger.WithContext(ctx)
}

func WithField(key string, value interface{}) Logger {
	return defaultLogger.WithField(key, value)
}

func WithFields(fields Fields) Logger {
	return defaultLogger.WithFields(fields)
}

func LogMiddleware(serviceName string) func(next func(ctx context.Context, req interface{}) (interface{}, error)) func(ctx context.Context, req interface{}) (interface{}, error) {
	return func(next func(ctx context.Context, req interface{}) (interface{}, error)) func(ctx context.Context, req interface{}) (interface{}, error) {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			start := time.Now()

			requestID := fmt.Sprintf("%d", time.Now().UnixNano())
			ctx = context.WithValue(ctx, "request_id", requestID)

			Info("Request started",
				Field{Key: "service", Value: serviceName},
				Field{Key: "request_id", Value: requestID},
				Field{Key: "request", Value: fmt.Sprintf("%T", req)},
			)

			resp, err := next(ctx, req)

			duration := time.Since(start)

			if err != nil {
				Error("Request failed",
					Field{Key: "service", Value: serviceName},
					Field{Key: "request_id", Value: requestID},
					Field{Key: "duration", Value: duration.Seconds()},
					Field{Key: "error", Value: err.Error()},
				)
			} else {
				Info("Request completed",
					Field{Key: "service", Value: serviceName},
					Field{Key: "request_id", Value: requestID},
					Field{Key: "duration", Value: duration.Seconds()},
				)
			}

			return resp, err
		}
	}
}
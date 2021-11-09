package go_logger

import (
	"context"
	"io"
	"sync"

	"go.uber.org/zap"
)

type Logger interface {
	Error(ctx context.Context, err error, fields ...zap.Field)
	Warn(ctx context.Context, msg string, fields ...zap.Field)
	Info(ctx context.Context, msg string, fields ...zap.Field)
	Debug(ctx context.Context, msg string, fields ...zap.Field)
	WrapError(ctx context.Context, err error, fields ...zap.Field) error
	Globalize()
	AddFields(fields ...zap.Field)
	ClearFields()
	ZapLogger() *zap.Logger
	io.Closer
}

type logger struct {
	sync.Mutex
	l         *zap.Logger
	name      string
	fields    []zap.Field
	tmpFields []zap.Field
	global    bool
}

// New zap logger with initial fields
func New(name string, level string, prod bool, fields ...zap.Field) (Logger, error) {
	if prod {
		return NewProduction(name, level, fields...)
	}
	return NewDevelopment(name, level, fields...)
}

// NewDevelopment logger
func NewDevelopment(name string, level string, fields ...zap.Field) (Logger, error) {
	cfg := zap.NewDevelopmentConfig()
	return newLogger(name, level, &cfg, fields...)
}

// NewProduction logger
func NewProduction(name string, level string, fields ...zap.Field) (Logger, error) {
	cfg := zap.NewProductionConfig()
	return newLogger(name, level, &cfg, fields...)
}

func newLogger(name, level string, cfg *zap.Config, fields ...zap.Field) (Logger, error) {
	err := setLevel(level, cfg)
	if err != nil {
		return nil, err
	}

	cfg.OutputPaths = []string{"stderr"}

	l, err := cfg.Build()
	if err != nil {
		return nil, err
	}

	fields = append(fields, zap.String("name", name))

	return &logger{
		l:      l,
		name:   name,
		fields: fields,
	}, nil
}

func setLevel(level string, cfg *zap.Config) error {
	al := zap.NewAtomicLevel()
	err := al.UnmarshalText([]byte(level))
	if err != nil {
		return err
	}

	cfg.Level.SetLevel(al.Level())
	return nil
}

func (l *logger) ZapLogger() *zap.Logger {
	return l.l
}

func (l *logger) Globalize() {
	zap.ReplaceGlobals(l.l)
	l.global = true
}

func (l *logger) AddFields(fields ...zap.Field) {
	l.Lock()
	defer l.Unlock()
	l.tmpFields = append(l.tmpFields, fields...)
}

func (l *logger) ClearFields() {
	l.Lock()
	defer l.Unlock()
	l.tmpFields = []zap.Field{}
}

func (l *logger) Close() error {
	// since we use stderr - no need to sync
	//return l.l.Sync()
	return nil
}

func (l *logger) Error(ctx context.Context, err error, fields ...zap.Field) {
	if l.okCtx(ctx) {
		l.logger().Error(l.withName(err.Error()), l.withFields(fields...)...)
	}
}

func (l *logger) Warn(ctx context.Context, msg string, fields ...zap.Field) {
	if l.okCtx(ctx) {
		l.logger().Warn(l.withName(msg), l.withFields(fields...)...)
	}
}

func (l *logger) Info(ctx context.Context, msg string, fields ...zap.Field) {
	if l.okCtx(ctx) {
		l.logger().Info(l.withName(msg), l.withFields(fields...)...)
	}
}

func (l *logger) Debug(ctx context.Context, msg string, fields ...zap.Field) {
	if l.okCtx(ctx) {
		l.logger().Debug(l.withName(msg), l.withFields(fields...)...)
	}
}

func (l *logger) WrapError(ctx context.Context, err error, fields ...zap.Field) error {
	l.Error(ctx, err, fields...)
	return err
}

func (l *logger) logger() *zap.Logger {
	if l.global {
		return zap.L()
	}
	return l.l
}

func (l *logger) okCtx(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		_ = l.l.Sync()
		return false
	default:
		return true
	}
}

func (l *logger) withFields(fields ...zap.Field) []zap.Field {
	l.Lock()
	defer l.Unlock()
	args := make([]zap.Field, 0, len(l.fields)+len(fields)+len(l.tmpFields))
	args = append(args, l.fields...)
	args = append(args, l.tmpFields...)
	args = append(args, fields...)
	return args
}

func (l *logger) withName(msg string) string {
	return "[" + l.name + "]: " + msg
}

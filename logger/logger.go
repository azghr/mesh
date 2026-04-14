package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// BannerService manages startup banners for services
type BannerService struct {
	mu      sync.RWMutex
	banners map[string]string
	colors  map[string]string
	enabled bool
}

// NewBannerService creates a new banner service instance
func NewBannerService() *BannerService {
	return &BannerService{
		banners: make(map[string]string),
		colors:  make(map[string]string),
		enabled: true,
	}
}

// bannerService is the default global banner service instance
var bannerService = NewBannerService()

// BannerOption configures the banner service
type BannerOption func(*BannerService)

// WithBannerEnabled enables or disables the banner
func WithBannerEnabled(enabled bool) BannerOption {
	return func(s *BannerService) {
		s.enabled = enabled
	}
}

// ConfigureBanner applies options to the global banner service
func ConfigureBanner(opts ...BannerOption) {
	for _, opt := range opts {
		opt(bannerService)
	}
}

// RegisterBanner registers a custom banner for a specific service.
// The banner should not include ANSI codes - they will be applied automatically.
func RegisterBanner(serviceName, banner string) {
	bannerService.mu.Lock()
	defer bannerService.mu.Unlock()
	bannerService.banners[strings.ToLower(serviceName)] = banner
}

// RegisterBannerColor registers a custom color for a service's banner
func RegisterBannerColor(serviceName, colorCode string) {
	bannerService.mu.Lock()
	defer bannerService.mu.Unlock()
	bannerService.colors[strings.ToLower(serviceName)] = colorCode
}

// GetBanner returns the banner for a service
func GetBanner(serviceName string) string {
	bannerService.mu.RLock()
	defer bannerService.mu.RUnlock()

	banner, exists := bannerService.banners[strings.ToLower(serviceName)]
	if !exists {
		return ""
	}
	return banner
}

// GetBannerColor returns the color code for a service
func GetBannerColor(serviceName string) string {
	bannerService.mu.RLock()
	defer bannerService.mu.RUnlock()

	color, exists := bannerService.colors[strings.ToLower(serviceName)]
	if !exists {
		return ServiceColor(serviceName)
	}
	return color
}

// PrintBanner prints the startup banner for a service
func PrintBanner(serviceName, version string) {
	if !bannerService.enabled {
		return
	}

	color := GetBannerColor(serviceName)
	banner := GetBanner(serviceName)

	coloredName := FormatServiceName(serviceName, true)
	coloredBanner := Bold + color + banner + Reset

	fmt.Println(coloredName)
	fmt.Println(coloredBanner)
	fmt.Printf("\n[%s] Version: %s | Starting up...\n\n",
		strings.ToUpper(serviceName), version)
}

// PrintBannerWithMessage prints the startup banner with a custom message
func PrintBannerWithMessage(serviceName, version, message string) {
	if !bannerService.enabled {
		return
	}

	color := GetBannerColor(serviceName)
	banner := GetBanner(serviceName)

	coloredName := FormatServiceName(serviceName, true)
	coloredBanner := Bold + color + banner + Reset

	fmt.Println(coloredName)
	fmt.Println(coloredBanner)
	fmt.Printf("\n[%s] Version: %s | %s\n\n",
		strings.ToUpper(serviceName), version, message)
}

// DisableBanner disables the banner printing
func DisableBanner() {
	bannerService.mu.Lock()
	defer bannerService.mu.Unlock()
	bannerService.enabled = false
}

// EnableBanner enables the banner printing
func EnableBanner() {
	bannerService.mu.Lock()
	defer bannerService.mu.Unlock()
	bannerService.enabled = true
}

// ANSI color codes
const (
	Reset  = "\033[0m"
	Bold   = "\033[1m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Cyan   = "\033[36m"
)

// Level represents the logging level
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// String returns the string representation of the log level
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel parses a string level into a Level
func ParseLevel(level string) Level {
	switch level {
	case "debug", "DEBUG":
		return LevelDebug
	case "info", "INFO":
		return LevelInfo
	case "warn", "warning", "WARN", "WARNING":
		return LevelWarn
	case "error", "ERROR":
		return LevelError
	default:
		return LevelInfo
	}
}

// ANSI color codes for terminal output
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorGray   = "\033[90m"
)

// LevelColor returns the ANSI color code for a given log level
func LevelColor(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return colorGray
	case slog.LevelInfo:
		return colorGreen
	case slog.LevelWarn:
		return colorYellow
	case slog.LevelError:
		return colorRed
	default:
		return colorWhite
	}
}

// ServiceColor returns a distinct color for each service name
func ServiceColor(serviceName string) string {
	// Hash the service name to get a consistent color
	hash := 0
	for _, c := range serviceName {
		hash += int(c)
	}

	colors := []string{colorCyan, colorPurple, colorBlue, colorGreen, colorYellow, Red}
	return colors[hash%len(colors)]
}

// FormatServiceName formats the service name with padding and color
func FormatServiceName(serviceName string, useColor bool) string {
	// Pad service name to 6 characters for alignment
	padded := fmt.Sprintf("%-6s", strings.ToUpper(serviceName))
	if useColor {
		return fmt.Sprintf("%s%s%s", ServiceColor(serviceName), padded, colorReset)
	}
	return padded
}

// customHandler is a custom slog handler for pretty terminal output
type customHandler struct {
	serviceName string
	useColor    bool
	minLevel    slog.Level
}

// Enabled implements slog.Handler
func (h *customHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.minLevel
}

// Handle implements slog.Handler with custom formatting
func (h *customHandler) Handle(ctx context.Context, r slog.Record) error {
	// Build the log line
	timestamp := time.Now().Format("15:04:05")
	serviceTag := FormatServiceName(h.serviceName, h.useColor)
	levelColor := LevelColor(r.Level)
	levelTag := r.Level.String()

	// Format: [TIME] SERVICE|LEVEL MESSAGE
	var line string
	if h.useColor {
		line = fmt.Sprintf("[%s] %s|%s%s%s %s",
			timestamp,
			serviceTag,
			levelColor, levelTag, colorReset,
			r.Message)
	} else {
		line = fmt.Sprintf("[%s] %s|%s %s",
			timestamp,
			strings.ToUpper(h.serviceName),
			levelTag,
			r.Message)
	}

	// Append any attributes as key=value pairs
	var attrs []string
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, fmt.Sprintf("%s=%v", a.Key, a.Value))
		return true
	})

	if len(attrs) > 0 {
		line += " " + strings.Join(attrs, " ")
	}

	fmt.Println(line)
	return nil
}

// WithAttrs implements slog.Handler
func (h *customHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h // For simplicity, we don't modify attrs in this custom handler
}

// WithGroup implements slog.Handler
func (h *customHandler) WithGroup(name string) slog.Handler {
	return h // For simplicity, we don't support groups in this custom handler
}

// Logger is the interface for logging
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Fatal(msg string, args ...any)
	With(args ...any) Logger
	WithField(key string, value any) Logger
	WithError(err error) Logger
	WithComponent(component string) Logger
	WithOperation(operation string) Logger
	WithContext(ctx context.Context) Logger
	WithFields(fields map[string]any) Logger
}

// ctxKey is the type for context keys
type ctxKey struct{}

// traceIDKey is the context key for trace IDs
var traceIDKey = ctxKey{}

// WithTraceID adds a trace ID to the context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// GetTraceID retrieves the trace ID from the context
func GetTraceID(ctx context.Context) string {
	if traceID, ok := ctx.Value(traceIDKey).(string); ok {
		return traceID
	}
	return ""
}

// logger is the concrete implementation of Logger
type logger struct {
	slog        *slog.Logger
	level       atomic.Value // Level
	component   string
	serviceName string
}

// New creates a new logger with the specified service name, level and format
func New(serviceName string, level string, jsonFormat bool) Logger {
	var slogLevel slog.Level
	switch ParseLevel(level) {
	case LevelDebug:
		slogLevel = slog.LevelDebug
	case LevelInfo:
		slogLevel = slog.LevelInfo
	case LevelWarn:
		slogLevel = slog.LevelWarn
	case LevelError:
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: slogLevel}

	var handler slog.Handler
	if jsonFormat {
		// For JSON format, use standard handler with service name as attr
		baseHandler := slog.NewJSONHandler(os.Stdout, opts)
		handler = baseHandler.WithAttrs([]slog.Attr{
			slog.String("service", serviceName),
		})
	} else {
		// Use custom pretty terminal handler
		handler = &customHandler{
			serviceName: serviceName,
			useColor:    true, // Enable colors by default for terminal output
			minLevel:    slogLevel,
		}
	}

	l := &logger{
		slog:        slog.New(handler),
		serviceName: serviceName,
	}
	l.level.Store(ParseLevel(level))

	return l
}

// MustNew creates a new logger and panics on error
func MustNew(serviceName string, level string, jsonFormat bool) Logger {
	return New(serviceName, level, jsonFormat)
}

// Default creates a logger with default settings (mesh service, info level, text format)
func Default() Logger {
	return New("mesh", "info", false)
}

// globalLogger is the default global logger instance
var globalLogger = Default()

// SetGlobal sets the global logger instance
func SetGlobal(l Logger) {
	globalLogger = l
}

// GetGlobal returns the global logger instance
func GetGlobal() Logger {
	return globalLogger
}

// Debug logs a debug message
func (l *logger) Debug(msg string, args ...any) {
	if l.getLevel() > LevelDebug {
		return
	}
	l.slog.Debug(msg, args...)
}

// Info logs an info message
func (l *logger) Info(msg string, args ...any) {
	if l.getLevel() > LevelInfo {
		return
	}
	l.slog.Info(msg, args...)
}

// Warn logs a warning message
func (l *logger) Warn(msg string, args ...any) {
	if l.getLevel() > LevelWarn {
		return
	}
	l.slog.Warn(msg, args...)
}

// Error logs an error message
func (l *logger) Error(msg string, args ...any) {
	if l.getLevel() > LevelError {
		return
	}
	l.slog.Error(msg, args...)
}

// Fatal logs an error message and exits the application (logrus compatibility)
func (l *logger) Fatal(msg string, args ...any) {
	l.slog.Error(msg, args...)
	os.Exit(1)
}

// With returns a new logger with additional fields
func (l *logger) With(args ...any) Logger {
	return &logger{
		slog:        l.slog.With(args...),
		level:       l.level,
		component:   l.component,
		serviceName: l.serviceName,
	}
}

// WithField returns a new logger with a single field (logrus compatibility)
func (l *logger) WithField(key string, value any) Logger {
	return &logger{
		slog:        l.slog.With(key, value),
		level:       l.level,
		component:   l.component,
		serviceName: l.serviceName,
	}
}

// WithError returns a new logger with an error field (logrus compatibility)
func (l *logger) WithError(err error) Logger {
	return &logger{
		slog:        l.slog.With("error", err.Error()),
		level:       l.level,
		component:   l.component,
		serviceName: l.serviceName,
	}
}

// WithComponent returns a new logger with a component field
func (l *logger) WithComponent(component string) Logger {
	return &logger{
		slog:        l.slog.With("component", component),
		level:       l.level,
		component:   component,
		serviceName: l.serviceName,
	}
}

// WithOperation returns a new logger with operation context
func (l *logger) WithOperation(operation string) Logger {
	return &logger{
		slog:        l.slog.With("operation", operation),
		level:       l.level,
		component:   l.component,
		serviceName: l.serviceName,
	}
}

// WithContext returns a new logger with context (including trace ID if present)
func (l *logger) WithContext(ctx context.Context) Logger {
	if traceID := GetTraceID(ctx); traceID != "" {
		return &logger{
			slog:        l.slog.With("trace_id", traceID),
			level:       l.level,
			component:   l.component,
			serviceName: l.serviceName,
		}
	}
	return l
}

// WithFields returns a new logger with multiple fields
func (l *logger) WithFields(fields map[string]any) Logger {
	args := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	return &logger{
		slog:        l.slog.With(args...),
		level:       l.level,
		component:   l.component,
		serviceName: l.serviceName,
	}
}

// getLevel returns the current log level
func (l *logger) getLevel() Level {
	return l.level.Load().(Level)
}

// SetLevel sets the log level
func (l *logger) SetLevel(level Level) {
	l.level.Store(level)
}

// Global convenience functions using the global logger

// Debug logs a debug message using the global logger
func Debug(msg string, args ...any) {
	globalLogger.Debug(msg, args...)
}

// Info logs an info message using the global logger
func Info(msg string, args ...any) {
	globalLogger.Info(msg, args...)
}

// Warn logs a warning message using the global logger
func Warn(msg string, args ...any) {
	globalLogger.Warn(msg, args...)
}

// Error logs an error message using the global logger
func Error(msg string, args ...any) {
	globalLogger.Error(msg, args...)
}

// With returns a new logger with additional fields using the global logger
func With(args ...any) Logger {
	return globalLogger.With(args...)
}

// WithComponent returns a new logger with component using the global logger
func WithComponent(component string) Logger {
	return globalLogger.WithComponent(component)
}

// WithOperation returns a new logger with operation using the global logger
func WithOperation(operation string) Logger {
	return globalLogger.WithOperation(operation)
}

// WithContext returns a new logger with context using the global logger
func WithContext(ctx context.Context) Logger {
	return globalLogger.WithContext(ctx)
}

// WithFields returns a new logger with fields using the global logger
func WithFields(fields map[string]any) Logger {
	return globalLogger.WithFields(fields)
}

// ContextLogger provides a chain and operation context logger
// This is for compatibility with the existing omnia/omniq logger pattern
type ContextLogger struct {
	logger    Logger
	chain     string
	operation string
}

// WithChainAndOperation creates a ContextLogger with chain and operation
// This maintains compatibility with the existing logger pattern from omnia/omniq
func WithChainAndOperation(chain, operation string) *ContextLogger {
	l := &ContextLogger{
		logger:    globalLogger.With("chain", chain).With("operation", operation),
		chain:     chain,
		operation: operation,
	}
	return l
}

// Info logs an info message with context
func (l *ContextLogger) Info(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	l.logger.Info(msg)
}

// Debug logs a debug message with context
func (l *ContextLogger) Debug(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	l.logger.Debug(msg)
}

// Warn logs a warning message with context
func (l *ContextLogger) Warn(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	l.logger.Warn(msg)
}

// Error logs an error message with context
func (l *ContextLogger) Error(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	l.logger.Error(msg)
}

// Timed logs and times an operation
func Timed(operation string, fn func()) {
	start := fmt.Sprintf("Starting %s", operation)
	Info(start)
	fn()
	completed := fmt.Sprintf("Completed %s", operation)
	Info(completed)
}

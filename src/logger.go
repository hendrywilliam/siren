package src

import (
	"encoding/json"
	"log"
	"log/slog"
	"os"
)

type Logger struct {
	levelLogger *slog.Logger
}

func NewLogger(moduleName string) *Logger {
	return &Logger{
		levelLogger: slog.New(slog.NewTextHandler(os.Stdout, nil)).With("module", moduleName),
	}
}

func (l *Logger) SetAttr(key string, value any) {
	l.levelLogger = l.levelLogger.With(key, value)
	return
}

func (l *Logger) Info(message string, args ...any) {
	l.levelLogger.Info(message, args...)
}

func (l *Logger) Debug(message string, args ...any) {
	l.levelLogger.Debug(message, args...)
}

func (l *Logger) Warn(message string, args ...any) {
	l.levelLogger.Warn(message, args...)
}

func (l *Logger) Error(err error, args ...any) {
	l.levelLogger.Error(err.Error(), args...)
}

func (l *Logger) Fatal(err error, args ...any) {
	l.Error(err, args...)
	os.Exit(1)
}

func (l *Logger) JSON(message any) {
	prettyJson, err := json.MarshalIndent(message, "  ", "  ")
	if err != nil {
		return
	}
	log.Printf("\n%s\n", string(prettyJson))
}

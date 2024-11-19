package src

import (
	"encoding/json"
	"log"
	"log/slog"
)

type Logger struct{}

func NewLogger() *Logger {
	return &Logger{}
}

func (l *Logger) Info(message string, args ...any) {
	slog.Info(message, args...)
}

func (l *Logger) Debug(message string, args ...any) {
	slog.Debug(message, args...)
}

func (l *Logger) Warn(message string, args ...any) {
	slog.Warn(message, args...)
}

func (l *Logger) Error(message string, args ...any) {
	slog.Error(message, args...)
}

func (l *Logger) JSON(message any) {
	prettyJson, err := json.MarshalIndent(message, "  ", "  ")
	if err != nil {
		return
	}
	log.Printf("\n%s\n", string(prettyJson))
}

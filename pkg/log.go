package pkg

import (
	"log/slog"
	"os"
	"strings"
)

var Log *slog.Logger

func init() {
	lvl := slog.LevelInfo
	if s := os.Getenv("LOG_LEVEL"); s != "" {
		switch strings.ToUpper(s) {
		case "DEBUG":
			lvl = slog.LevelDebug
		case "INFO":
			lvl = slog.LevelInfo
		case "WARN":
			lvl = slog.LevelWarn
		case "ERROR":
			lvl = slog.LevelError
		}
	}
	Log = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}

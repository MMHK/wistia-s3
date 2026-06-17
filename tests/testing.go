package tests

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/joho/godotenv"
)

var log *slog.Logger

//preload config in testing
func init()  {
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
	log = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))

	err := godotenv.Load(GetLocalPath("../.env"))
	if err != nil {
		log.Error("Error loading environment", "error", err)
	}
}

func GetLocalPath(file string) string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), file)
}

func ToJSON(target interface{}) (string) {
	str := new(bytes.Buffer)
	encoder := json.NewEncoder(str)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "    ")
	err := encoder.Encode(target)
	if err != nil {
		return err.Error()
	}

	return str.String()
}
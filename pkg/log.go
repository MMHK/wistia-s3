package pkg

import (
	"github.com/op/go-logging"
	"os"
)

var Log = logging.MustGetLogger("wistia-s3")

func init() {
	format := logging.MustStringFormatter(
		`WISTIA-S3 %{color} %{shortfunc} %{level:.4s} %{shortfile}
%{id:03x}%{color:reset} %{message}`,
	)
	logging.SetFormatter(format)

	levelStr := os.Getenv("LOG_LEVEL")
	if len(levelStr) == 0 {
		levelStr = "INFO"
	}
	level, err := logging.LogLevel(levelStr)
	if err != nil {
		level = logging.INFO
	}
	logging.SetLevel(level, "wistia-s3")
}

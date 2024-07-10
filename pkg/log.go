package pkg

import "github.com/op/go-logging"

var Log = logging.MustGetLogger("wistia-s3")

func init() {
	format := logging.MustStringFormatter(
		`WISTIA-S3 %{color} %{shortfunc} %{level:.4s} %{shortfile}
%{id:03x}%{color:reset} %{message}`,
	)
	logging.SetFormatter(format)
}

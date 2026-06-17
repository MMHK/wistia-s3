package main

import (
	"flag"
	"runtime"
	"wistia-s3/pkg"
)

func main() {
	conf_path := flag.String("c", "conf.json", "config json file")
	flag.Parse()

	runtime.GOMAXPROCS(runtime.NumCPU())

	conf, err := pkg.NewConfigFromLocal(*conf_path)
	if err != nil {
		pkg.Log.Error("failed to load config, using defaults", "error", err, "path", *conf_path)
		conf = &pkg.Config{}
	}

	conf.MarginWithENV()

	pkg.Log.Debug("config loaded", "config", conf)

	service := pkg.NewHTTP(conf)
	service.Start()
}

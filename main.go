package main

import (
	"flag"

	"github.com/prometheus/common/log"
	"github.com/prometheus/prometheus/config"
)

var (
	configFile = flag.String("config.file", "prometheus.yml", "prometheus configuration file name.")
)

func main() {
	flag.Parse()

	log.Infof("Loading configuration file %s", *configFile)

	conf, err := config.LoadFile(*configFile)
	if err != nil {
		log.Errorf("Couldn't load configuration (-config.file=%s): %v", *configFile, err)
	} else {
		for _, scfg := range conf.ScrapeConfigs {
			log.Info(scfg.JobName)
		}
	}

}

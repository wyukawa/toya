package main

import (
	"flag"

	"github.com/prometheus/common/log"
	"github.com/prometheus/prometheus/config"

	"gopkg.in/yaml.v2"
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
		cfg := &config.ScrapeConfig{}
		cfg.JobName = "aaa"
		var newScrapeConfigs []*config.ScrapeConfig
		newScrapeConfigs = append(newScrapeConfigs, cfg)
		for _, scfg := range conf.ScrapeConfigs {
			newScrapeConfigs = append(newScrapeConfigs, scfg)
		}
		for _, nscfg := range newScrapeConfigs {
			log.Info(nscfg.JobName)
		}
		newYamlData, err := yaml.Marshal(&newScrapeConfigs)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		log.Infof(string(newYamlData))
	}

}

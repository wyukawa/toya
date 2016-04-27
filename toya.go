package main

import (
	"flag"
	"html/template"
	"io/ioutil"
	"net/http"

	"github.com/prometheus/common/log"
	"github.com/prometheus/prometheus/config"

	"gopkg.in/yaml.v2"
)

var (
	configFile    = flag.String("config.file", "prometheus.yml", "prometheus configuration file name.")
	listenAddress = flag.String("listen-address", ":12345", "The address to listen on for HTTP requests.")
)

func main() {
	flag.Parse()

	log.Infof("Loading configuration file %s", *configFile)

	conf, err := config.LoadFile(*configFile)
	if err != nil {
		log.Errorf("Couldn't load configuration (-config.file=%s): %v", *configFile, err)
	} else {
		log.Infof("Starting Server: %s", *listenAddress)
		t := template.Must(template.ParseFiles("templates/toya.html"))
		http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()
			if r.Method == "GET" {
				t.Execute(w, conf.ScrapeConfigs)
			} else if r.Method == "POST" {
				//exporter_list = r.FormValue("exporter_list")
				newcfg := &config.ScrapeConfig{}
				newcfg.JobName = r.FormValue("job_name")
				fsdc := &config.FileSDConfig{}
				fsdc.Names = []string{"aaa.yaml"}
				newcfg.FileSDConfigs = append(newcfg.FileSDConfigs, fsdc)
				rc := &config.RelabelConfig{}
				rc.Regex = config.MustNewRegexp("(.*):\\d+")
				newcfg.RelabelConfigs = append(newcfg.RelabelConfigs, rc)

				var newScrapeConfigs []*config.ScrapeConfig
				newScrapeConfigs = append(newScrapeConfigs, newcfg)
				for _, scfg := range conf.ScrapeConfigs {
					newScrapeConfigs = append(newScrapeConfigs, scfg)
				}
				conf.ScrapeConfigs = newScrapeConfigs
				newYamlData, err := yaml.Marshal(&conf)
				if err != nil {
					log.Fatalf("error: %v", err)
				}
				writeErr := ioutil.WriteFile(*configFile, newYamlData, 0644)
				if writeErr != nil {
					log.Fatalf("error: %v", writeErr)
				}
				t.Execute(w, newScrapeConfigs)
			}
		})
		err := http.ListenAndServe(*listenAddress, nil)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
	}

}

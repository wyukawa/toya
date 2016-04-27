package main

import (
	"flag"
	"html/template"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"github.com/prometheus/common/log"
	"github.com/prometheus/prometheus/config"

	"gopkg.in/yaml.v2"
)

var (
	configFile      = flag.String("config.file", "prometheus.yml", "prometheus configuration file name.")
	exporterListDir = flag.String("exporter.list.dir", ".", "file_sd_configs names file dir")
	listenAddress   = flag.String("listen-address", ":12345", "The address to listen on for HTTP requests.")
)

func rewrite() {
	conf, err := config.LoadFile(*configFile)
	if err != nil {
		log.Errorf("Couldn't load configuration (-config.file=%s): %v", *configFile, err)
	}
	yamlData, yamlErr := yaml.Marshal(&conf)
	if yamlErr != nil {
		log.Fatalf("error: %v", yamlErr)
	}
	writeErr := ioutil.WriteFile(*configFile, yamlData, 0644)
	if writeErr != nil {
		log.Fatalf("error: %v", writeErr)
	}
}

func create(job_name string, exporter_list string) {
	newcfg := &config.ScrapeConfig{}
	newcfg.JobName = job_name
	fsdc := &config.FileSDConfig{}
	exporterListFile := filepath.Join(*exporterListDir, job_name+".yml")
	exporterListFileWriteErr := ioutil.WriteFile(exporterListFile, []byte(exporter_list), 0644)
	if exporterListFileWriteErr != nil {
		log.Fatalf("error: %v", exporterListFileWriteErr)
	}
	fsdc.Names = []string{exporterListFile}
	newcfg.FileSDConfigs = append(newcfg.FileSDConfigs, fsdc)
	rc := &config.RelabelConfig{}
	rc.Regex = config.MustNewRegexp("(.*):\\d+")
	newcfg.RelabelConfigs = append(newcfg.RelabelConfigs, rc)

	var newScrapeConfigs []*config.ScrapeConfig
	newScrapeConfigs = append(newScrapeConfigs, newcfg)
	conf, confErr := config.LoadFile(*configFile)
	if confErr != nil {
		log.Errorf("Couldn't load configuration (-config.file=%s): %v", *configFile, confErr)
	}
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
	rewrite()
}

func main() {
	flag.Parse()

	log.Infof("Loading configuration file %s", *configFile)
	log.Infof("Starting Server: %s", *listenAddress)

	t := template.Must(template.ParseFiles("templates/toya.html"))
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if r.Method == "GET" {
			conf, err := config.LoadFile(*configFile)
			if err != nil {
				log.Errorf("Couldn't load configuration (-config.file=%s): %v", *configFile, err)
			}
			t.Execute(w, conf.ScrapeConfigs)
		}
	})
	http.HandleFunc("/create", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if r.Method == "POST" {
			create(r.FormValue("job_name"), r.FormValue("exporter_list"))
			http.Redirect(w, r, "/", http.StatusFound)
		}
	})
	err := http.ListenAndServe(*listenAddress, nil)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

}

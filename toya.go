package main

import (
	"flag"
	"html/template"
	"net/http"

	"github.com/prometheus/common/log"
	"github.com/prometheus/prometheus/config"
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
			if r.Method == "GET" {
				t.Execute(w, conf.ScrapeConfigs)
			}
		})
		err := http.ListenAndServe(*listenAddress, nil)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
	}

}

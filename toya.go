package main

import (
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/prometheus/common/log"
	"github.com/prometheus/prometheus/config"

	"gopkg.in/yaml.v2"
)

type Page struct {
	JobExporterMap      map[string]string
	ErrorMessage        string
	PrometheusStatusUrl string
}

var (
	configFile      = flag.String("config.file", "prometheus.yml", "prometheus configuration file name.")
	exporterListDir = flag.String("exporter.list.dir", "/tmp", "file_sd_configs names file dir")
	listenAddress   = flag.String("listen-address", ":20010", "The address to listen on for HTTP requests.")
	prometheusUrl   = flag.String("prometheus.url", "http://localhost:9090", "prometheus")
)

func rewrite() error {
	conf, err := config.LoadFile(*configFile)
	if err != nil {
		return errors.Wrapf(err, "Couldn't load configuration (-config.file=%s): %v", *configFile)
	}
	yamlData, yamlErr := yaml.Marshal(&conf)
	if yamlErr != nil {
		return errors.Wrapf(yamlErr, "yaml marshal error: %v", yamlErr)
	}
	writeErr := ioutil.WriteFile(*configFile, yamlData, 0644)
	if writeErr != nil {
		return errors.Wrapf(writeErr, "yaml write error: %v", writeErr)
	}
	return nil
}

func create(job_name string, exporter_list string) error {
	newcfg := &config.ScrapeConfig{}
	newcfg.JobName = job_name
	fsdc := &config.FileSDConfig{}
	exporterListFile := filepath.Join(*exporterListDir, job_name+".yml")
	strs := "'" + strings.Replace(exporter_list, "\r\n", "','", -1) + "'"
	exporterListFileWriteErr := ioutil.WriteFile(exporterListFile, []byte(fmt.Sprintf("[{\"targets\":[%s]}]", strs)), 0644)
	if exporterListFileWriteErr != nil {
		return errors.Wrapf(exporterListFileWriteErr, "error: %v", exporterListFileWriteErr)
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
		return errors.Wrapf(confErr, "Couldn't load configuration (-config.file=%s): %v", *configFile, confErr)
	}
	for _, scfg := range conf.ScrapeConfigs {
		newScrapeConfigs = append(newScrapeConfigs, scfg)
	}
	conf.ScrapeConfigs = newScrapeConfigs
	reloadErr := reload(conf)
	if reloadErr != nil {
		return errors.Wrapf(reloadErr, "reload error %v", reloadErr)
	}
	return nil
}

func reload(conf *config.Config) error {
	newYamlData, err := yaml.Marshal(&conf)
	if err != nil {
		return errors.Wrapf(err, "yaml marshal error: %v", err)
	}
	writeErr := ioutil.WriteFile(*configFile, newYamlData, 0644)
	if writeErr != nil {
		return errors.Wrapf(writeErr, "write error: %v", writeErr)
	}
	rewriteErr := rewrite()
	if rewriteErr != nil {
		return errors.Wrapf(rewriteErr, "rewrite error: %v", rewriteErr)
	}
	_, postErr := http.PostForm(*prometheusUrl+"/-/reload", url.Values{})
	if postErr != nil {
		return errors.Wrapf(postErr, "post error: %v", postErr)
	}
	return nil
}

func loadPage(w http.ResponseWriter, t *template.Template, errorMessage string) error {
	conf, err := config.LoadFile(*configFile)
	if err != nil {
		return errors.Wrapf(err, "Couldn't load configuration (-config.file=%s): %v", *configFile, err)
	}
	jobExporterMap := make(map[string]string)
	for _, scfg := range conf.ScrapeConfigs {
		for _, fsdc := range scfg.FileSDConfigs {
			content, e := ioutil.ReadFile(fsdc.Names[0])
			if e != nil {
				return errors.Wrapf(e, "read file error: %v", e)
			}
			jobExporterMap[scfg.JobName] = string(content)
		}
	}
	page := Page{
		JobExporterMap:      jobExporterMap,
		ErrorMessage:        errorMessage,
		PrometheusStatusUrl: *prometheusUrl + "/status",
	}
	t.Execute(w, page)
	return nil
}

func delete(job_name string) error {
	conf, err := config.LoadFile(*configFile)
	if err != nil {
		return errors.Wrapf(err, "Couldn't load configuration (-config.file=%s): %v", *configFile, err)
	}
	var newScrapeConfigs []*config.ScrapeConfig
	for _, scfg := range conf.ScrapeConfigs {
		if scfg.JobName == job_name {
			for _, fsdc := range scfg.FileSDConfigs {
				removeErr := os.Remove(fsdc.Names[0])
				if removeErr != nil {
					return errors.Wrapf(removeErr, "remove error %v", removeErr)
				}
			}
		} else {
			newScrapeConfigs = append(newScrapeConfigs, scfg)
		}
	}
	conf.ScrapeConfigs = newScrapeConfigs
	reloadErr := reload(conf)
	if reloadErr != nil {
		return errors.Wrapf(reloadErr, "reload error %v", reloadErr)
	}
	return nil
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
			loadPageErr := loadPage(w, t, "")
			if loadPageErr != nil {
				log.Errorf("loadPage error: %v", loadPageErr)
				return
			}
		}

	})

	http.HandleFunc("/create", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if r.Method == "POST" {
			errorMessage := ""
			if r.FormValue("job_name") == "" {
				errorMessage += "Job Name is required\n"
			}
			conf, err := config.LoadFile(*configFile)
			if err != nil {
				log.Errorf("Couldn't load configuration (-config.file=%s): %v", *configFile)
				return
			}
			for _, scfg := range conf.ScrapeConfigs {
				if r.FormValue("job_name") == scfg.JobName {
					errorMessage += "duplicate Job Name\n"
					break
				}
			}
			if r.FormValue("exporter_list") == "" {
				errorMessage += "Exporter lis is required\n"
			}
			for _, exporter := range strings.Split(r.FormValue("exporter_list"), "\r\n") {
				if strings.Contains(exporter, "/") {
					errorMessage += exporter + " is not a valid hostname\n"
				}
			}
			if errorMessage == "" {
				err := create(r.FormValue("job_name"), r.FormValue("exporter_list"))
				if err != nil {
					log.Errorf("create error: %v", err)
					return
				}
			}

			http.Redirect(w, r, "/", http.StatusFound)
		}

	})

	http.HandleFunc("/delete", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if r.Method == "POST" {
			deleteErr := delete(r.FormValue("job_name"))
			if deleteErr != nil {
				log.Errorf("delete error: %v", deleteErr)
				return
			}
			http.Redirect(w, r, "/", http.StatusFound)
		}

	})

	err := http.ListenAndServe(*listenAddress, nil)
	if err != nil {
		log.Errorf("error: %v", err)
		return
	}

}

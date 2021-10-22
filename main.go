/*
 * Copyright 2021 Kevin van den Broek (info@kevinvandenbroek.nl)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package main

import (
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
)

const (
	Down = "Down"
	Up   = "Up"
)

var (
	dotcomScrapeSuccessDesc = prometheus.NewDesc(
		prometheus.BuildFQName("dotcom", "", "scrape_success"),
		"Whether scraping dotcom device status was successful.",
		nil, nil)
	dotcomDeviceStatusDesc = prometheus.NewDesc(
		prometheus.BuildFQName("dotcom", "device", "status"),
		"Whether the dotcom alert is active or not",
		[]string{"id", "name", "status"}, nil)
)

var ErrNoResults = errors.New("no device statuses received")

// DotcomMonitorConfig is the top level structure contained in the XML response
type DotcomMonitorConfig struct {
	Devices []Device `xml:"Site"`
}

// Collect data from each individual site Device
func (d *DotcomMonitorConfig) Collect(ch chan<- prometheus.Metric) {
	for _, d := range d.Devices {
		d.Collect(ch)
	}
}

// Device is a target that dotcom monitor monitors
type Device struct {
	ID     string `xml:"ID,attr"`
	Name   string `xml:"Name,attr"`
	State  string `xml:"State,attr"`
	Status string `xml:"Status,attr"`
}

// Collect data from the per-device object and convert it to Prometheus metrics.
func (d *Device) Collect(ch chan<- prometheus.Metric) {
	var state float64
	switch d.State {
	case Down:
		state = 0
	case Up:
		state = 1
	default:
		state = 2
	}

	ch <- prometheus.MustNewConstMetric(dotcomDeviceStatusDesc, prometheus.GaugeValue, state, d.ID, d.Name, d.Status)
}

// DotcomMonitor scrapes the XML api from dotcom monitor for the given sites.
type DotcomMonitor struct {
	// Used to make sure we don't have two concurrent requests to dotcom.
	mu sync.Mutex

	// Client used to talk with the dotcom monitor XML api.
	client http.Client

	// This is your account Global Unique Identifier (Configure > Integrations > the Unique Identifier (UID) column).
	pid string

	// This is a list of site (so called “Devices”) IDs or names in Dotcom-Monitor. You can output multiple sites in one request:
	// Use “*” to select all the sites.
	// Individual Site IDs can be found when logged in to your account on the Device Manager screen by clicking edit in the action menu next to the selected device. When the next screen loads, the Site ID will be in the URL as such:
	// https://user.dotcom-monitor.com/Site-Edit.aspx?id=123456
	// You can also get a Site ID with the config.aspx request using the XML interface.
	sites []string
}

// Describe metrics provided by the dotcom exporter.
func (e *DotcomMonitor) Describe(ch chan<- *prometheus.Desc) {
	ch <- dotcomScrapeSuccessDesc
	ch <- dotcomDeviceStatusDesc
}

// scrape scrapes the dotcom monitor XML api.
// It returns an error if the HTTP status code is not 200
// and when the response is empty.
func (d *DotcomMonitor) scrape(ch chan<- prometheus.Metric) error {
	params := url.Values{}
	params.Add("PID", d.pid)
	for _, s := range d.sites {
		params.Add("site", s)
	}

	res, err := d.client.Get("https://xmlreporter.dotcom-monitor.com/reporting/xml/status.aspx?" + params.Encode())
	if err != nil {
		return err
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("did not receive a HTTP 200 OK, received HTTP %s", res.Status)
	}

	var response DotcomMonitorConfig
	decoder := xml.NewDecoder(res.Body)
	err = decoder.Decode(&response)
	if err != nil {
		return err
	}

	if len(response.Devices) == 0 {
		return ErrNoResults
	}

	// Extract Prometheus metrics from the report.
	response.Collect(ch)
	return nil
}

// Collect metrics from dotcom monitor.
func (d *DotcomMonitor) Collect(ch chan<- prometheus.Metric) {
	ts := time.Now()
	if err := d.scrape(ch); err != nil {
		log.Error("Failed to gather stats: ", err)
		ch <- prometheus.NewMetricWithTimestamp(ts, prometheus.MustNewConstMetric(dotcomScrapeSuccessDesc, prometheus.GaugeValue, 0.0))
		return
	}

	ch <- prometheus.NewMetricWithTimestamp(ts, prometheus.MustNewConstMetric(dotcomScrapeSuccessDesc, prometheus.GaugeValue, 1.0))
}

func NewDotcomMonitor(pid string, sites []string, httpTimeout time.Duration) *DotcomMonitor {
	return &DotcomMonitor{
		client: http.Client{
			Timeout: httpTimeout,
		},
		sites: sites,
		pid:   pid,
	}
}

func main() {
	var (
		listenAddress = flag.String("web.listen-address", ":9423", "Address to listen on for web interface and telemetry.")
		metricsPath   = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")

		sites       = flag.String("dotcom.sites", "*", "comma separated list of sites to monitor. Format: Site1,Site2,Site3. This is a list of site (so called “Devices”) IDs or names in Dotcom-Monitor. You can output multiple sites in one request and use wildcard characters (“*”, “?”) to filter request results by some pattern: 123*")
		pid         = flag.String("dotcom.pid", "2AA43CD13DDS2CHJ20FGHY85DF203E33", "This is your account Global Unique Identifier (Configure > Integrations > the Unique Identifier (UID) column).")
		httpTimeout = flag.Duration("dotcom.http.timeout", 10*time.Second, "HTTP timeout used when scraping from dotcom")
	)
	flag.Parse()

	log.Infof("Starting dotcom monitor exporter; PID: %.10s... ; Sites: %s", *pid, strings.Split(*sites, ","))
	exporter := NewDotcomMonitor(*pid, strings.Split(*sites, ","), *httpTimeout)
	prometheus.MustRegister(exporter)

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
			<html>
			<head><title>dotcom-monitor exporter</title></head>
			<body>
			<h1>dotcom-monitor exporter</h1>
			<p><a href='` + *metricsPath + `'>Metrics</a></p>
			</body>
			</html>`),
		)
	})

	log.Info("Listening on address:port => ", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}

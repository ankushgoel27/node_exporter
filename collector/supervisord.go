// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !nosupervisord
// +build !nosupervisord

package collector

import (
	"fmt"

	"github.com/mattn/go-xmlrpc"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	supervisordURL = kingpin.Flag("collector.supervisord.url", "XML RPC endpoint.").Default("http://localhost:9001/RPC2").String()
)

type supervisordCollector struct {
	upDesc         *prometheus.Desc
	stateDesc      *prometheus.Desc
	exitStatusDesc *prometheus.Desc
	startTimeDesc  *prometheus.Desc
}

func init() {
	registerCollector("supervisord", defaultDisabled, NewSupervisordCollector)
}

// NewSupervisordCollector returns a new Collector exposing supervisord statistics.
func NewSupervisordCollector() (Collector, error) {
	var (
		subsystem  = "supervisord"
		labelNames = []string{"name", "group"}
	)
	return &supervisordCollector{
		upDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "up"),
			"Process Up",
			labelNames,
			nil,
		),
		stateDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "state"),
			"Process State",
			labelNames,
			nil,
		),
		exitStatusDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "exit_status"),
			"Process Exit Status",
			labelNames,
			nil,
		),
		startTimeDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "start_time_seconds"),
			"Process start time",
			labelNames,
			nil,
		),
	}, nil
}

func (c *supervisordCollector) isRunning(state int) bool {
	// http://supervisord.org/subprocess.html#process-states
	const (
		STOPPED  = 0
		STARTING = 10
		RUNNING  = 20
		BACKOFF  = 30
		STOPPING = 40
		EXITED   = 100
		FATAL    = 200
		UNKNOWN  = 1000
	)
	switch state {
	case STARTING, RUNNING, STOPPING:
		return true
	}
	return false
}

func (c *supervisordCollector) Update(ch chan<- prometheus.Metric) error {
	var info struct {
		Name          string `xmlrpc:"name"`
		Group         string `xmlrpc:"group"`
		Start         int    `xmlrpc:"start"`
		Stop          int    `xmlrpc:"stop"`
		Now           int    `xmlrpc:"now"`
		State         int    `xmlrpc:"state"`
		StateName     string `xmlrpc:"statename"`
		SpawnErr      string `xmlrpc:"spanerr"`
		ExitStatus    int    `xmlrpc:"exitstatus"`
		StdoutLogfile string `xmlrcp:"stdout_logfile"`
		StderrLogfile string `xmlrcp:"stderr_logfile"`
		PID           int    `xmlrpc:"pid"`
	}

	res, err := xmlrpc.Call(*supervisordURL, "supervisor.getAllProcessInfo")
	if err != nil {
		return fmt.Errorf("unable to call supervisord: %s", err)
	}

	for _, p := range res.(xmlrpc.Array) {
		for k, v := range p.(xmlrpc.Struct) {
			switch k {
			case "name":
				info.Name = v.(string)
			case "group":
				info.Group = v.(string)
			case "start":
				info.Start = v.(int)
			case "stop":
				info.Stop = v.(int)
			case "now":
				info.Now = v.(int)
			case "state":
				info.State = v.(int)
			case "statename":
				info.StateName = v.(string)
			case "exitstatus":
				info.ExitStatus = v.(int)
			case "pid":
				info.PID = v.(int)
			}
		}
		labels := []string{info.Name, info.Group}

		ch <- prometheus.MustNewConstMetric(c.stateDesc, prometheus.GaugeValue, float64(info.State), labels...)
		ch <- prometheus.MustNewConstMetric(c.exitStatusDesc, prometheus.GaugeValue, float64(info.ExitStatus), labels...)

		if c.isRunning(info.State) {
			ch <- prometheus.MustNewConstMetric(c.upDesc, prometheus.GaugeValue, 1, labels...)
			ch <- prometheus.MustNewConstMetric(c.startTimeDesc, prometheus.CounterValue, float64(info.Start), labels...)
		} else {
			ch <- prometheus.MustNewConstMetric(c.upDesc, prometheus.GaugeValue, 0, labels...)
		}
		log.Debugf("%s:%s is %s on pid %d", info.Group, info.Name, info.StateName, info.PID)
	}

	return nil
}

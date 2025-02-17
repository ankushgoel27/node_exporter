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

package collector

import (
	"os"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

func TestDiskStats(t *testing.T) {
	file, err := os.Open("fixtures/proc/diskstats")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	diskStats, err := parseDiskStats(file)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := "25353629", diskStats["sda4"][0]; want != got {
		t.Errorf("want diskstats sda4 %s, got %s", want, got)
	}

	if want, got := "68", diskStats["mmcblk0p2"][10]; want != got {
		t.Errorf("want diskstats mmcblk0p2 %s, got %s", want, got)
	}
}

func TestDiskStatsUpdate(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{"--path.procfs", "fixtures/proc"}); err != nil {
		t.Fatal(err)
	}

	dsc, err := NewDiskstatsCollector()
	if err != nil {
		t.Fatal(err)
	}

	ch := make(chan prometheus.Metric, 1000)
	err = dsc.Update(ch)
	close(ch)
	if err != nil {
		t.Fatal(err)
	}

	var metrics []prometheus.Metric
	for m := range ch {
		metrics = append(metrics, m)
	}

	// 14 devices reported in fixtures/proc/diskstats
	if len(metrics) != 14*len(dsc.(*diskstatsCollector).descs) {
		t.Errorf("want %d diskstats, got %d", 14*len(dsc.(*diskstatsCollector).descs), len(metrics))
	}
}

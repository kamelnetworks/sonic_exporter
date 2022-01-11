package main

import (
	"fmt"
	"sync"

	promlog "github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/node_exporter/collector"
	"github.com/sirupsen/logrus"
	promflag "gopkg.in/alecthomas/kingpin.v2"
)

type PromLogAdapter struct {
	log *logrus.Logger
}

func (l *PromLogAdapter) Log(vs ...interface{}) error {
	kv := map[string]interface{}{}
	for i := 0; i < len(vs); i += 2 {
		k := fmt.Sprintf("%v", vs[i])
		v := vs[i+1]
		kv[k] = v
	}
	msg, ok := kv["msg"]
	if !ok {
		// Ignore log message without message
		return nil
	}
	delete(kv, "msg")
	ilevel, ok := kv["level"]
	if !ok {
		ilevel = "debug"
	}
	delete(kv, "level")
	level, err := logrus.ParseLevel(fmt.Sprintf("%s", ilevel))
	if err != nil {
		level = logrus.DebugLevel
	}
	l.log.WithFields(logrus.Fields(kv)).Logf(level, "node-library: %v", msg)
	return nil
}

type NodeCollector struct {
	Collectors map[string]Collector
	log        *logrus.Logger
}

// This is how node_exporter does the same, but since it does not allow
// controlling what collectors are enabled programmatically we have to
// duplicate this logic.
func (n NodeCollector) Describe(ch chan<- *prometheus.Desc) {
}

func (n NodeCollector) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	wg.Add(len(n.Collectors))
	for name, c := range n.Collectors {
		go func(name string, c Collector) {
			err := c.Update(ch)
			if err != nil {
				n.log.Errorf("%s failed: %v", name, err)
			}
			wg.Done()
		}(name, c)
	}
	wg.Wait()
}

type Collector interface {
	Update(ch chan<- prometheus.Metric) error
}

type CollectorFactory func(logger promlog.Logger) (collector.Collector, error)

func NewNodeCollector(log *logrus.Logger) (*NodeCollector, error) {
	promlogger := &PromLogAdapter{log}
	nc := &NodeCollector{}
	nc.Collectors = map[string]Collector{}

	for _, c := range []struct {
		cf   CollectorFactory
		name string
	}{
		{collector.NewCPUCollector, "cpu"},
		{collector.NewConntrackCollector, "conntrack"},
		{collector.NewDiskstatsCollector, "diskstats"},
		{collector.NewEdacCollector, "edac"},
		{collector.NewFilesystemCollector, "filesystem"},
		{collector.NewHwMonCollector, "hwmon"},
		{collector.NewLoadavgCollector, "loadavg"},
		{collector.NewMeminfoCollector, "meminfo"},
		{collector.NewNetClassCollector, "netclass"},
		{collector.NewNetDevCollector, "netdev"},
		{collector.NewNetStatCollector, "netstat"},
		{collector.NewStatCollector, "stat"},
		{collector.NewTimeCollector, "time"},
		{collector.NewvmStatCollector, "vmstat"},
		{collector.NewSystemdCollector, "systemd"},
		{collector.NewNtpCollector, "ntp"},
	} {
		cc, err := c.cf(promlogger)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", c.name, err)
		}
		nc.Collectors[c.name] = cc
	}
	return nc, nil
}

func InitNodeFlags() {
	// Again, since node_exporter is not a library as such this is a pretty ugly
	// way to set it up as a library. I expect this to break from time to time
	// when the node library is upgrade.
	promflag.CommandLine.Parse([]string{})
}

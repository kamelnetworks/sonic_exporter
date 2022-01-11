package main

import (
	"context"
	"fmt"
	"log/syslog"
	"net"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	promcollectors "github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	lsyslog "github.com/sirupsen/logrus/hooks/syslog"
)

var (
	Version = "(devel)"
	GitHash = "(no hash)"

	exporterInfoMetric = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sonic_exporter_build_info",
		Help: "This info metric contains build information for about the exporter",
	}, []string{
		"exporter_version", "exporter_revision", "go_version",
		"lib_prometheus_version", "lib_node_version"})
)

const (
	RedisDB_Counters = 2
	RedisDB_Config   = 4
	RedisDB_State    = 6
)

type BuildInfo struct {
	version        string
	gitHash        string
	goVersion      string
	promLibVersion string
	nodeLibVersion string
}

type ExporterConfig struct {
	Port int
	VRF  string
}

func getBuildInfo() BuildInfo {
	info, ok := debug.ReadBuildInfo()
	// don't overwrite the version if it was set by -ldflags=-X
	if ok && Version == "(devel)" {
		mod := &info.Main
		if mod.Replace != nil {
			mod = mod.Replace
		}
		Version = mod.Version
	}
	// remove leading `v`
	massagedVersion := strings.TrimPrefix(Version, "v")
	bi := BuildInfo{
		version:        massagedVersion,
		gitHash:        GitHash,
		goVersion:      runtime.Version(),
		promLibVersion: "(unknown)",
		nodeLibVersion: "(unknown)",
	}
	if ok {
		for _, d := range info.Deps {
			if d.Path == "github.com/prometheus/client_golang" {
				bi.promLibVersion = d.Version
			}
			if d.Path == "github.com/prometheus/node_exporter" {
				bi.nodeLibVersion = d.Version
			}
		}
	}
	exporterInfoMetric.With(prometheus.Labels{
		"exporter_version":       bi.version,
		"exporter_revision":      bi.gitHash,
		"go_version":             bi.goVersion,
		"lib_prometheus_version": bi.promLibVersion,
		"lib_node_version":       bi.nodeLibVersion,
	}).Set(1)
	return bi
}

func attachToVRF(vrf string) func(string, string, syscall.RawConn) error {
	return func(network string, address string, c syscall.RawConn) error {
		if vrf == "" {
			return nil
		}
		var operr error
		fn := func(fd uintptr) {
			operr = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, vrf)
		}
		if err := c.Control(fn); err != nil {
			return err
		}
		if operr != nil {
			return operr
		}
		return nil
	}
}

func metricsHandler(w http.ResponseWriter, r *http.Request, log *logrus.Logger, registry *prometheus.Registry) {
	start := time.Now()

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)

	duration := time.Since(start).Seconds()
	log.Infof("Metrics reporting done, took %.3f seconds", duration)
}

func loadAndWatchConfig(log *logrus.Logger) *ExporterConfig {
	ctx := context.Background()

	configdb := redis.NewClient(&redis.Options{
		Network:  "unix",
		Addr:     "/var/run/redis/redis.sock",
		Password: "",
		DB:       RedisDB_Config,
	})

	cfg, err := configdb.HGetAll(ctx, "SONIC_EXPORTER|default").Result()
	if err != nil {
		log.Fatalf("Failed to read configuration from redis: %v", err)
	}
	cfgpat := fmt.Sprintf("__keyspace@%d__:%s", RedisDB_Config, "SONIC_EXPORTER|default")
	cfgch := configdb.PSubscribe(ctx, cfgpat).Channel()
	go func() {
		<-cfgch
		log.Info("Configuration change detected, restarting exporter...")
		os.Exit(0)
	}()

	var port int64 = 9893 // Registered Prometheus exporter port
	if v, found := cfg["port"]; found {
		port, err = strconv.ParseInt(v, 10, 16)
		if err != nil {
			log.Fatalf("Failed to parse port number %q: %v", v, err)
		}
	}
	vrf := ""
	if v, found := cfg["vrf"]; found {
		vrf = v
	}
	return &ExporterConfig{
		Port: int(port),
		VRF:  vrf,
	}
}

func main() {
	bi := getBuildInfo()
	log := logrus.New()
	log.SetFormatter(&logrus.TextFormatter{
		DisableColors:    true,
		DisableTimestamp: true,
	})
	// Enable this for verbose logging (including node_exporter debug logs)
	// log.SetLevel(logrus.DebugLevel)

	// SONiC rsyslog format is {container_name}#{binary}
	tag := "sonic_exporter#/sonic_exporter"
	// TODO: We should look up this IP or pass it as an argument or something.
	// In SONiC it seems that where the rsyslog receiver is vary when doing
	// multi-ASCI platforms.
	hook, err := lsyslog.NewSyslogHook("udp", "127.0.0.1:514", syslog.LOG_INFO, tag)
	if err != nil {
		panic(err)
	}
	log.Hooks.Add(hook)

	log.WithFields(logrus.Fields{
		"exporter-version":       bi.version,
		"exporter-git-hash":      bi.gitHash,
		"lib-prometheus-version": bi.promLibVersion,
		"lib-node-version":       bi.nodeLibVersion,
	}).Info("Starting up")

	config := loadAndWatchConfig(log)

	log.WithFields(logrus.Fields{
		"port": config.Port,
		"vrf":  config.VRF,
	}).Infof("Configuration loaded")

	listen := fmt.Sprintf(":%d", config.Port)
	lc := net.ListenConfig{Control: attachToVRF(config.VRF)}
	ln, err := lc.Listen(context.Background(), "tcp", listen)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	defer ln.Close()

	InitNodeFlags()

	registry := prometheus.NewRegistry()
	registry.MustRegister(promcollectors.NewProcessCollector(promcollectors.ProcessCollectorOpts{}))
	registry.MustRegister(promcollectors.NewGoCollector())
	registry.MustRegister(exporterInfoMetric)
	nc, err := NewNodeCollector(log)
	if err != nil {
		log.Fatalf("Failed to create node collector: %v", err)
	}
	registry.MustRegister(nc)

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metricsHandler(w, r, log, registry)
	})
	go func() {
		if err := http.Serve(ln, nil); err != nil {
			log.Fatalf("Unable to serve: %v", err)
		}
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
		<head><title>SONiC Exporter</title></head>
		<body>
		<h1>SONiC Exporter</h1>
		<p><a href="/metrics">Metrics</a></p>
		</body>
		</html>`))
	})

	log.Infof("SONiC Prometheus exporter running")
	select {}
}

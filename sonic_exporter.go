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

	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	lsyslog "github.com/sirupsen/logrus/hooks/syslog"
)

var (
	Version = "(devel)"
	GitHash = "(no hash)"
)

const (
	RedisDB_Counters = 2
	RedisDB_Config   = 4
	RedisDB_State    = 6
)

type BuildInfo struct {
	version   string
	gitHash   string
	goVersion string
}

func getBuildInfo() BuildInfo {
	// don't overwrite the version if it was set by -ldflags=-X
	if info, ok := debug.ReadBuildInfo(); ok && Version == "(devel)" {
		mod := &info.Main
		if mod.Replace != nil {
			mod = mod.Replace
		}
		Version = mod.Version
	}
	// remove leading `v`
	massagedVersion := strings.TrimPrefix(Version, "v")
	bi := BuildInfo{
		version:   massagedVersion,
		gitHash:   GitHash,
		goVersion: runtime.Version(),
	}
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

func main() {
	bi := getBuildInfo()
	log := logrus.New()
	log.SetFormatter(&logrus.TextFormatter{
		DisableColors:    true,
		DisableTimestamp: true,
	})
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
		"version":  bi.version,
		"git-hash": bi.gitHash,
	}).Info("Starting up")

	configdb := redis.NewClient(&redis.Options{
		Network:  "unix",
		Addr:     "/var/run/redis/redis.sock",
		Password: "",
		DB:       RedisDB_Config,
	})

	ctx := context.Background()
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

	log.WithFields(logrus.Fields{
		"port": port,
		"vrf":  vrf,
	}).Infof("Configuration loaded")

	listen := fmt.Sprintf(":%d", port)
	lc := net.ListenConfig{Control: attachToVRF(vrf)}
	ln, err := lc.Listen(context.Background(), "tcp", listen)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	defer ln.Close()

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		if err := http.Serve(ln, nil); err != nil {
			log.Fatalf("Unable to serve: %v", err)
		}
	}()

	log.Infof("SONiC Prometheus exporter running")
	select {}
}

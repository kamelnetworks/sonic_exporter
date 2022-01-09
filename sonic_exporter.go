package main

import (
	"log/syslog"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	lsyslog "github.com/sirupsen/logrus/hooks/syslog"
)

var (
	Version = "(devel)"
	GitHash = "(no hash)"
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
	for {
		log.WithFields(logrus.Fields{
			"animal": "walrus",
			"size":   10,
		}).Info("Hello world")
		time.Sleep(1 * time.Second)
		log.WithFields(logrus.Fields{
			"test": "boo",
		}).Error("Hello world error")
		time.Sleep(1 * time.Second)
	}
}

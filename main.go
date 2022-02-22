package main

import (
	"flag"
	"log"

	"github.com/dr-useless/gobkv/store"
)

var cpuProfile = flag.String("cpuprof", "", "write cpu profile to `file`")
var memProfile = flag.String("memprof", "", "write memory profile to `file`")

var configFile = flag.String("c", "", "must be a file path")

func main() {
	flag.Parse()

	// pprof
	startCPUProfile()

	cfg, err := loadConfig()
	if err != nil {
		log.Fatal("failed to load config: ", err)
	}

	st := store.Store{
		Dir: cfg.Dir,
	}
	st.EnsureParts(&cfg.Parts)
	go st.ScanForExpiredKeys(&cfg.Parts, cfg.ExpiryScanPeriod)

	// if ReplServer is defined, start as master
	if cfg.ReplMasterConfig.Address != "" {
		log.Println("configured as master")
		replMaster := store.ReplMaster{}
		go replMaster.Init(&cfg.ReplMasterConfig)
		st.ReplMaster = &replMaster
	} else if cfg.ReplClientConfig.Address != "" {
		log.Println("configured as replica")
		replClient := store.ReplClient{
			Dir: cfg.Dir,
		}
		replClient.Store = &st
		replClient.Init(&cfg.ReplClientConfig)
	}

	watchdog := Watchdog{
		store: &st,
		cfg:   &cfg,
	}
	// blocks until parts are ready
	watchdog.readFromPartFiles()
	go watchdog.watch()

	listener, err := store.GetListener(
		cfg.Network, cfg.Address, cfg.CertFile, cfg.KeyFile)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go serveConn(conn, &st, cfg.AuthSecret)
	}
}

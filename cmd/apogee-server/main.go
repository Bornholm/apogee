// Command apogee-server runs a dedicated sky on the local network (GDD 10.3).
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"apogee/internal/discovery"
	"apogee/internal/proto"
	"apogee/internal/server"
)

// version is set at release time, by the linker.
var version = "dev"

func main() {
	addr := flag.String("addr", fmt.Sprintf(":%d", proto.DefaultPort), "TCP listen address")
	name := flag.String("name", "Apogée", "name of the sky, shown to players")
	mdns := flag.Bool("mdns", true, "announce the server on the local network")
	data := flag.String("data", "sky.json", "file where the sky is saved and restored")
	rivals := flag.Int("rivals", 5, "rival AI gods in a new sky")
	seed := flag.Uint64("seed", 1, "seed of a new sky")
	speed := flag.Int("speed", 1, "simulation steps per tick (1 is real time)")
	showVersion := flag.Bool("version", false, "print the version and quit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Error("listen", "err", err)
		os.Exit(1)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	log.Info("sky open", "version", version, "name", *name, "addr", ln.Addr().String())

	if *mdns {
		ad, err := discovery.Advertise(*name, port)
		if err != nil {
			// Not fatal: players can still join by address (GDD 10.1).
			log.Warn("mdns announcement failed", "err", err)
		} else {
			defer ad.Stop()
		}
	}

	srv, err := server.New(server.Config{
		Name: *name, Logger: log, DataFile: *data, Rivals: *rivals, Seed: *seed, Speed: *speed,
	})
	if err != nil {
		log.Error("opening the sky", "err", err)
		os.Exit(1)
	}
	ran := make(chan struct{})
	go func() {
		defer close(ran)
		srv.Run(ctx)
	}()
	err = srv.Serve(ctx, ln)
	stop()
	<-ran // the last save happens when Run ends
	if err != nil {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
	log.Info("sky closed")
}

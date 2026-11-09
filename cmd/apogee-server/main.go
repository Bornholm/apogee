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

func main() {
	addr := flag.String("addr", fmt.Sprintf(":%d", proto.DefaultPort), "TCP listen address")
	name := flag.String("name", "Apogée", "name of the sky, shown to players")
	mdns := flag.Bool("mdns", true, "announce the server on the local network")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Error("listen", "err", err)
		os.Exit(1)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	log.Info("sky open", "name", *name, "addr", ln.Addr().String())

	if *mdns {
		ad, err := discovery.Advertise(*name, port)
		if err != nil {
			// Not fatal: players can still join by address (GDD 10.1).
			log.Warn("mdns announcement failed", "err", err)
		} else {
			defer ad.Stop()
		}
	}

	srv := server.New(server.Config{Name: *name, Logger: log})
	go srv.Run(ctx)
	if err := srv.Serve(ctx, ln); err != nil {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
	log.Info("sky closed")
}

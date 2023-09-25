package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/buglloc/rogue-metadata/internal/config"
)

func fatalf(msg string, a ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "rmd: "+msg+"\n", a...)
	os.Exit(1)
}

func main() {
	var cfgPath string
	flag.StringVar(&cfgPath, "config", "", "path to config")
	flag.Parse()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	log.Info().Msg("load config")
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		fatalf("load config: %v", err)
	}

	log.Info().Msg("create runtime config")
	runtime, err := cfg.NewRuntime()
	if err != nil {
		fatalf("create config: %v", err)
	}

	log.Info().Msg("create DNS server")
	dnssrv, err := runtime.NewDNSServer()
	if err != nil {
		fatalf("create instance data provider: %v", err)
	}

	log.Info().Msg("create instance-data server")
	isrv, err := runtime.NewInstanceDataServer()
	if err != nil {
		fatalf("create instance data provider: %v", err)
	}

	g, ctx := errgroup.WithContext(context.Background())
	g.Go(func() error {
		log.Info().Msg("starts DNS server")
		if err := dnssrv.ListenAndServe(); err != nil {
			return fmt.Errorf("listen dnssrv: %w", err)
		}

		return nil
	})

	g.Go(func() error {
		log.Info().Msg("starts instance-data server")
		if err := isrv.ListenAndServe(); err != nil {
			return fmt.Errorf("listen instance data srv: %w", err)
		}

		return nil
	})

	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()

		_ = dnssrv.Shutdown(ctx)
		_ = isrv.Shutdown(ctx)
	}

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-stopChan:
		log.Info().Msg("shutting down gracefully by signal")
		shutdown()
	case <-ctx.Done():
		log.Warn().Msg("unexpected exit")
		shutdown()
	}

	if err := g.Wait(); err != nil {
		fatalf("%v", err)
	}
}

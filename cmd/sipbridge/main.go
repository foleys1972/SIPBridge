package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sipbridge/internal/api"
	"sipbridge/internal/config"
	"sipbridge/internal/sip"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	cm, err := config.NewManager(cfg)
	if err != nil {
		log.Fatalf("config manager: %v", err)
	}
	rootCfg, err := cm.LoadFromFile()
	if err != nil {
		log.Fatalf("app config: %v", err)
	}

	cfg.SIP = config.MergeSIPFromSpec(cfg.SIP, rootCfg.Spec.SIPStack)
	if err := config.ValidateSIPConfig(cfg.SIP); err != nil {
		log.Fatalf("sip config (env + config.yaml spec.sipStack): %v", err)
	}

	cluster := config.MergeClusterLimits(cfg.Cluster, rootCfg.Spec.Cluster)
	if err := config.ValidateClusterLimits(cluster); err != nil {
		log.Fatalf("cluster: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cm.StartHTTPPoll(ctx)

	router := sip.NewRouter(cm.Current)
	sipSrv := sip.NewServer(cfg.SIP, router, cluster)
	apiSrv := api.NewServer(cfg.API, sipSrv, cfg.SIP, rootCfg, cm)

	errCh := make(chan error, 2)
	go func() { errCh <- sipSrv.Start(ctx) }()
	go func() { errCh <- apiSrv.Start(ctx) }()

	select {
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		_ = sipSrv.Stop(shutdownCtx)
		_ = apiSrv.Stop(shutdownCtx)
	case err := <-errCh:
		log.Printf("service error: %v", err)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = sipSrv.Stop(shutdownCtx)
		_ = apiSrv.Stop(shutdownCtx)
		os.Exit(1)
	}

	_ = http.ErrServerClosed
}

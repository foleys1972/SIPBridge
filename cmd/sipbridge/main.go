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
	"sipbridge/internal/logger"
	"sipbridge/internal/sip"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// File logging — redirect stdlib log to stdout + rotating daily file.
	logCloser, err := logger.Init(cfg.LogDir)
	if err != nil {
		log.Fatalf("logger: %v", err)
	}
	defer logCloser.Close()

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

	// Log active TLS mode so operators can confirm cert configuration on startup.
	if cfg.SIP.OutboundTransport == "tls" {
		log.Printf("SIP outbound transport: TLS proxy=%s:%d ca=%q cert=%q sni=%q insecure=%v",
			cfg.SIP.OutboundProxyAddr, cfg.SIP.OutboundProxyPort,
			cfg.SIP.TLSRootCAFile, cfg.SIP.TLSClientCertFile,
			cfg.SIP.TLSServerName, cfg.SIP.TLSInsecureSkipVerify)
	}
	if cfg.SIP.SessionTimerEnabled {
		log.Printf("SIP session-timer: enabled (RFC 4028, Min-SE=90 Session-Expires=1800)")
	}

	captureSpec := rootCfg.Spec.Capture
	if captureSpec != nil && captureSpec.Enabled {
		log.Printf("audio capture: enabled dir=%s", captureSpec.CaptureDirectory())
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cm.StartHTTPPoll(ctx)

	router := sip.NewRouter(cm.Current)
	sipSrv := sip.NewServer(cfg.SIP, router, cluster)
	sipSrv.SetCaptureSpec(captureSpec)

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

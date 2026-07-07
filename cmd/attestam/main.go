/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/kentakayama/AttesTAM/internal/config"
	internallogger "github.com/kentakayama/AttesTAM/internal/logger"
	"github.com/kentakayama/AttesTAM/internal/server"
)

const (
	envAddr                           = "ATTESTAM_ADDR"
	envTAMTEEPPrivateKeyPath          = "ATTESTAM_TAM_TEEP_PRIVATE_KEY_PATH"
	envDBPath                         = "ATTESTAM_DB_PATH"
	envInsecureDemoMode               = "ATTESTAM_INSECURE_DEMO_MODE"
	envChallengeServer                = "ATTESTAM_CHALLENGE_SERVER"
	envChallengeContentType           = "ATTESTAM_CHALLENGE_CONTENT_TYPE"
	envChallengeInsecureTLS           = "ATTESTAM_CHALLENGE_INSECURE_TLS"
	envChallengeTimeout               = "ATTESTAM_CHALLENGE_TIMEOUT"
	envIntelCollateralCacheDir        = "ATTESTAM_INTEL_COLLATERAL_CACHE_DIR"
	envIntelCollateralServiceURL      = "ATTESTAM_INTEL_COLLATERAL_SERVICE_URL"
	envIntelCollateralInsecureTLS     = "ATTESTAM_INTEL_COLLATERAL_SERVICE_INSECURE_TLS"
	envIntelCollateralSubscriptionKey = "ATTESTAM_INTEL_COLLATERAL_SUBSCRIPTION_KEY"
)

func main() {
	var (
		addr                           = flag.String("addr", "localhost:8080", "listen address in host:port form. If you want to listen on all interfaces, use :8080.")
		privateKeyPath                 = flag.String("tam-teep-private-key-path", "", "file path to the TAM's private key in COSE_Key format. Required unless insecure demo mode is enabled.")
		dbPath                         = flag.String("db-path", "tam_state.db", "file path to the SQLite database")
		insecureDemoMode               = flag.Bool("insecure-demo-mode", false, "enable insecure demo mode with the public insecure demo TAM key, demo TC Developer key, and demo seed data. (not recommended for production)")
		challengeServer                = flag.String("challenge-server", "https://localhost:8443", "base URL for verifier challenge-response server")
		challengeContentType           = flag.String("challenge-content-type", `application/eat+cwt; eat_profile="urn:ietf:rfc:rfc9711"`, "Content-Type for attestation payload submission")
		challengeInsecureTLS           = flag.Bool("challenge-insecure-tls", true, "skip TLS verification when contacting the verifier")
		challengeTimeout               = flag.Duration("challenge-timeout", time.Minute, "timeout for verifier challenge-response interactions")
		intelCollateralCacheDir        = flag.String("intel-collateral-cache-dir", "./", "directory for Intel quote collateral cache. If empty, AttesTAM's Intel collateral cache is disabled.")
		intelCollateralServiceURL      = flag.String("intel-collateral-service-url", "https://api.trustedservices.intel.com/sgx/certification/v4", "base URL for Intel PCS/PCCS collateral retrieval")
		intelCollateralInsecureTLS     = flag.Bool("intel-collateral-service-insecure-tls", false, "skip TLS verification when retrieving Intel collateral from PCS/PCCS")
		intelCollateralSubscriptionKey = flag.String("intel-collateral-subscription-key", "", "optional Intel PCS subscription key for Intel collateral retrieval")
	)
	flag.Parse()

	logger := internallogger.New().With("service", "attestam")

	addrVal := stringFromEnv(logger, envAddr, *addr)
	privateKeyPathVal := stringFromEnv(logger, envTAMTEEPPrivateKeyPath, *privateKeyPath)
	dbPathVal := stringFromEnv(logger, envDBPath, *dbPath)
	insecureDemoModeVal := boolFromEnv(logger, envInsecureDemoMode, *insecureDemoMode)
	challengeServerVal := stringFromEnv(logger, envChallengeServer, *challengeServer)
	challengeContentTypeVal := stringFromEnv(logger, envChallengeContentType, *challengeContentType)
	challengeInsecureTLSVal := boolFromEnv(logger, envChallengeInsecureTLS, *challengeInsecureTLS)
	challengeTimeoutVal := durationFromEnv(logger, envChallengeTimeout, *challengeTimeout)
	intelCollateralCacheDirVal := stringFromEnv(logger, envIntelCollateralCacheDir, *intelCollateralCacheDir)
	intelCollateralServiceURLVal := stringFromEnv(logger, envIntelCollateralServiceURL, *intelCollateralServiceURL)
	intelCollateralInsecureTLSVal := boolFromEnv(logger, envIntelCollateralInsecureTLS, *intelCollateralInsecureTLS)
	intelCollateralSubscriptionKeyVal := stringFromEnv(logger, envIntelCollateralSubscriptionKey, *intelCollateralSubscriptionKey)

	cfg := config.TAMConfig{
		Addr:                           addrVal,
		TAMTEEPPrivateKeyPath:          privateKeyPathVal,
		DBPath:                         dbPathVal,
		InsecureDemoMode:               insecureDemoModeVal,
		Logger:                         logger,
		ChallengeServerURL:             challengeServerVal,
		ChallengeContentType:           challengeContentTypeVal,
		ChallengeInsecureTLS:           challengeInsecureTLSVal,
		ChallengeTimeout:               challengeTimeoutVal,
		IntelCollateralCacheDir:        intelCollateralCacheDirVal,
		IntelCollateralServiceURL:      intelCollateralServiceURLVal,
		IntelCollateralInsecureTLS:     intelCollateralInsecureTLSVal,
		IntelCollateralSubscriptionKey: intelCollateralSubscriptionKeyVal,
	}

	srv, err := server.New(cfg)
	if err != nil {
		logger.Error("failed to create server", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received, stopping")
	case err := <-errCh:
		if err != nil {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	logger.Info("server stopped cleanly")
}

func stringFromEnv(logger *slog.Logger, envKey, defaultValue string) string {
	if value, ok := os.LookupEnv(envKey); ok {
		return value
	}
	return defaultValue
}

func boolFromEnv(logger *slog.Logger, envKey string, defaultValue bool) bool {
	value, ok := os.LookupEnv(envKey)
	if !ok {
		return defaultValue
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		logger.Error("invalid boolean environment value", "env", envKey, "err", err)
		os.Exit(1)
	}

	return parsed
}

func durationFromEnv(logger *slog.Logger, envKey string, defaultValue time.Duration) time.Duration {
	value, ok := os.LookupEnv(envKey)
	if !ok {
		return defaultValue
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		logger.Error("invalid duration environment value", "env", envKey, "err", err)
		os.Exit(1)
	}

	return parsed
}

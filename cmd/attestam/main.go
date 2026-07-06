/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/kentakayama/AttesTAM/internal/config"
	"github.com/kentakayama/AttesTAM/internal/server"
)

const (
	envAddr                       = "ATTESTAM_ADDR"
	envTAMTEEPPrivateKeyPath      = "ATTESTAM_TAM_TEEP_PRIVATE_KEY_PATH"
	envDBPath                     = "ATTESTAM_DB_PATH"
	envInsecureDemoMode           = "ATTESTAM_INSECURE_DEMO_MODE"
	envChallengeServer            = "ATTESTAM_CHALLENGE_SERVER"
	envChallengeContentType       = "ATTESTAM_CHALLENGE_CONTENT_TYPE"
	envChallengeInsecureTLS       = "ATTESTAM_CHALLENGE_INSECURE_TLS"
	envChallengeTimeout           = "ATTESTAM_CHALLENGE_TIMEOUT"
	envIntelQVLCollateralCacheDir = "ATTESTAM_INTEL_QVL_COLLATERAL_CACHE_DIR"
	envIntelQVLPCSURL             = "ATTESTAM_INTEL_QVL_PCS_URL"
	envIntelQVLSubscriptionKey    = "ATTESTAM_INTEL_QVL_SUBSCRIPTION_KEY"
)

func main() {
	var (
		addr                       = flag.String("addr", "localhost:8080", "listen address in host:port form. If you want to listen on all interfaces, use :8080.")
		privateKeyPath             = flag.String("tam-teep-private-key-path", "", "file path to the TAM's private key in COSE_Key format. Required unless insecure demo mode is enabled.")
		dbPath                     = flag.String("db-path", "tam_state.db", "file path to the SQLite database")
		insecureDemoMode           = flag.Bool("insecure-demo-mode", false, "enable insecure demo mode with the public insecure demo TAM key, demo TC Developer key, and demo seed data. (not recommended for production)")
		challengeServer            = flag.String("challenge-server", "https://localhost:8443", "base URL for verifier challenge-response server")
		challengeContentType       = flag.String("challenge-content-type", `application/eat+cwt; eat_profile="urn:ietf:rfc:rfc9711"`, "Content-Type for attestation payload submission")
		challengeInsecureTLS       = flag.Bool("challenge-insecure-tls", true, "skip TLS verification when contacting the verifier")
		challengeTimeout           = flag.Duration("challenge-timeout", time.Minute, "timeout for verifier challenge-response interactions")
		intelQVLCollateralCacheDir = flag.String("intel-qvl-collateral-cache-dir", "./", "directory for Intel QVL quote collateral cache. If empty, AttesTAM's Intel QVL collateral cache is disabled.")
		intelQVLPCSURL             = flag.String("intel-qvl-pcs-url", "https://api.trustedservices.intel.com/sgx/certification/v4", "base URL for Intel PCS collateral retrieval used by Intel QVL quote verification")
		intelQVLSubscriptionKey    = flag.String("intel-qvl-subscription-key", "", "optional Intel PCS subscription key for Intel QVL collateral retrieval")
	)
	flag.Parse()

	logger := log.New(os.Stdout, "[attestam] ", log.LstdFlags|log.LUTC)

	addrVal := stringFromEnv(logger, envAddr, *addr)
	privateKeyPathVal := stringFromEnv(logger, envTAMTEEPPrivateKeyPath, *privateKeyPath)
	dbPathVal := stringFromEnv(logger, envDBPath, *dbPath)
	insecureDemoModeVal := boolFromEnv(logger, envInsecureDemoMode, *insecureDemoMode)
	challengeServerVal := stringFromEnv(logger, envChallengeServer, *challengeServer)
	challengeContentTypeVal := stringFromEnv(logger, envChallengeContentType, *challengeContentType)
	challengeInsecureTLSVal := boolFromEnv(logger, envChallengeInsecureTLS, *challengeInsecureTLS)
	challengeTimeoutVal := durationFromEnv(logger, envChallengeTimeout, *challengeTimeout)
	intelQVLCollateralCacheDirVal := stringFromEnv(logger, envIntelQVLCollateralCacheDir, *intelQVLCollateralCacheDir)
	intelQVLPCSURLVal := stringFromEnv(logger, envIntelQVLPCSURL, *intelQVLPCSURL)
	intelQVLSubscriptionKeyVal := stringFromEnv(logger, envIntelQVLSubscriptionKey, *intelQVLSubscriptionKey)

	cfg := config.TAMConfig{
		Addr:                       addrVal,
		TAMTEEPPrivateKeyPath:      privateKeyPathVal,
		DBPath:                     dbPathVal,
		InsecureDemoMode:           insecureDemoModeVal,
		Logger:                     logger,
		ChallengeServerURL:         challengeServerVal,
		ChallengeContentType:       challengeContentTypeVal,
		ChallengeInsecureTLS:       challengeInsecureTLSVal,
		ChallengeTimeout:           challengeTimeoutVal,
		IntelQVLCollateralCacheDir: intelQVLCollateralCacheDirVal,
		IntelQVLPCSURL:             intelQVLPCSURLVal,
		IntelQVLSubscriptionKey:    intelQVLSubscriptionKeyVal,
	}

	srv, err := server.New(cfg)
	if err != nil {
		logger.Fatalf("failed to create server: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Println("Shutdown signal received, stopping...")
	case err := <-errCh:
		if err != nil {
			logger.Fatalf("server error: %v", err)
		}
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Fatalf("graceful shutdown failed: %v", err)
	}
	logger.Println("Server stopped cleanly.")
}

func stringFromEnv(logger *log.Logger, envKey, defaultValue string) string {
	if value, ok := os.LookupEnv(envKey); ok {
		return value
	}
	return defaultValue
}

func boolFromEnv(logger *log.Logger, envKey string, defaultValue bool) bool {
	value, ok := os.LookupEnv(envKey)
	if !ok {
		return defaultValue
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		logger.Fatalf("invalid boolean for %s: %v", envKey, err)
	}

	return parsed
}

func durationFromEnv(logger *log.Logger, envKey string, defaultValue time.Duration) time.Duration {
	value, ok := os.LookupEnv(envKey)
	if !ok {
		return defaultValue
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		logger.Fatalf("invalid duration for %s: %v", envKey, err)
	}

	return parsed
}

/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
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

	"github.com/kentakayama/tam-over-http/internal/config"
	"github.com/kentakayama/tam-over-http/internal/server"
)

const (
	envAddr                 = "TAM4WASM_ADDR"
	envChallengeServer      = "TAM4WASM_CHALLENGE_SERVER"
	envChallengeContentType = "TAM4WASM_CHALLENGE_CONTENT_TYPE"
	envChallengeInsecureTLS = "TAM4WASM_CHALLENGE_INSECURE_TLS"
	envChallengeTimeout     = "TAM4WASM_CHALLENGE_TIMEOUT"
)

func main() {
	var (
		addr                 = flag.String("addr", ":8080", "listen address in host:port form")
		challengeServer      = flag.String("challenge-server", "https://localhost:8443", "base URL for verifier challenge-response server")
		challengeContentType = flag.String("challenge-content-type", `application/eat+cwt; eat_profile="urn:ietf:rfc:rfc9711"`, "Content-Type for attestation payload submission")
		challengeInsecureTLS = flag.Bool("challenge-insecure-tls", true, "skip TLS verification when contacting the verifier")
		challengeTimeout     = flag.Duration("challenge-timeout", time.Minute, "timeout for verifier challenge-response interactions")
	)
	flag.Parse()

	logger := log.New(os.Stdout, "[tam4wasm] ", log.LstdFlags|log.LUTC)

	addrVal := stringFromEnv(logger, envAddr, *addr)
	challengeServerVal := stringFromEnv(logger, envChallengeServer, *challengeServer)
	challengeContentTypeVal := stringFromEnv(logger, envChallengeContentType, *challengeContentType)
	challengeInsecureTLSVal := boolFromEnv(logger, envChallengeInsecureTLS, *challengeInsecureTLS)
	challengeTimeoutVal := durationFromEnv(logger, envChallengeTimeout, *challengeTimeout)

	cfg := config.TAMConfig{
		Addr:                 addrVal,
		Logger:               logger,
		ChallengeServerURL:   challengeServerVal,
		ChallengeContentType: challengeContentTypeVal,
		ChallengeInsecureTLS: challengeInsecureTLSVal,
		ChallengeTimeout:     challengeTimeoutVal,
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

/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/kentakayama/AttesTAM/internal/config"
	"github.com/kentakayama/AttesTAM/internal/infra/rats"
	"github.com/kentakayama/AttesTAM/internal/tam"
)

// Server wires the HTTP listener and request handling stack.
type Server struct {
	cfg     config.TAMConfig
	handler *handler
	http    *http.Server
	logger  *slog.Logger
}

// New constructs a Server using the provided configuration.
func New(cfg config.TAMConfig) (*Server, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	if !cfg.InsecureDemoMode && cfg.TAMTEEPPrivateKeyPath == "" {
		logger.Error("using the public insecure demo TAM private key is not allowed outside insecure demo mode; set -tam-teep-private-key-path")
		return nil, errors.New("tam private key path is required outside insecure demo mode")
	}

	raCfg := config.RAConfig{
		BaseURL:                        cfg.ChallengeServerURL,
		ContentType:                    cfg.ChallengeContentType,
		InsecureTLS:                    cfg.ChallengeInsecureTLS,
		Timeout:                        cfg.ChallengeTimeout,
		Logger:                         logger,
		IntelCollateralCacheDir:        cfg.IntelCollateralCacheDir,
		IntelCollateralServiceURL:      cfg.IntelCollateralServiceURL,
		IntelCollateralInsecureTLS:     cfg.IntelCollateralInsecureTLS,
		IntelCollateralSubscriptionKey: cfg.IntelCollateralSubscriptionKey,
	}

	logger.Debug("attestation verifier backend selection follows QueryResponse attestation-payload-format", "intel_qvl_format", rats.AttestationPayloadFormatSGXQuote3TEEP)

	var t *tam.TAM
	var err error
	if cfg.InsecureDemoMode {
		t, err = tam.NewDemoTAMWithRAConfig(raCfg, logger)
	} else {
		t, err = tam.NewTAMWithRAConfig(cfg.TAMTEEPPrivateKeyPath, raCfg, logger)
	}
	if err != nil {
		return nil, err
	}
	if err := t.InitDB(cfg.DBPath); err != nil {
		return nil, err
	}
	if cfg.InsecureDemoMode {
		logger.Warn("insecure demo mode is enabled; this should not be used in production environments")
		if err := t.SeedDemoData(); err != nil {
			return nil, err
		}
	}

	h, err := newHandler(t, logger)
	if err != nil {
		return nil, err
	}

	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &Server{
		cfg:     cfg,
		handler: h,
		http:    httpSrv,
		logger:  logger,
	}, nil
}

// ListenAndServe starts the HTTP server and blocks until it stops.
func (s *Server) ListenAndServe() error {
	s.logger.Debug("run TAM server", "addr", s.http.Addr)

	err := s.http.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown gracefully takes down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/kentakayama/tam-over-http/internal/config"
	"github.com/kentakayama/tam-over-http/internal/infra/rats"
	"github.com/kentakayama/tam-over-http/internal/tam"
)

// Server wires the HTTP listener and request handling stack.
type Server struct {
	cfg     config.TAMConfig
	handler *handler
	http    *http.Server
	logger  *log.Logger
}

// New constructs a Server using the provided configuration.
func New(cfg config.TAMConfig) (*Server, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}

	verifierClient, err := rats.NewVerifierClient(config.RAConfig{
		BaseURL:     cfg.ChallengeServerURL,
		ContentType: cfg.ChallengeContentType,
		InsecureTLS: cfg.ChallengeInsecureTLS,
		Timeout:     cfg.ChallengeTimeout,
		Logger:      logger,
	})
	if err != nil {
		return nil, err
	}

	tam, err := tam.NewTAM(false, verifierClient, logger)
	if err != nil {
		return nil, err
	}
	if err := tam.Init(); err != nil {
		return nil, err
	}
	if err := tam.EnsureDefaultEntity(false); err != nil {
		return nil, err
	}
	if err := tam.EnsureDefaultTEEPAgent(true); err != nil {
		return nil, err
	}

	h, err := newHandler(tam, logger)
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
	s.logger.Printf("Run TAM Server on %s.", s.http.Addr)

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

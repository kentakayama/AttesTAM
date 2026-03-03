/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package config

import (
	"log"
	"time"
)

// Config captures the tunables required to start the TAM mock server.
type TAMConfig struct {
	Addr                  string
	InsecureDemoMode      bool
	TAMTEEPPrivateKeyPath string
	Logger                *log.Logger
	ChallengeServerURL    string
	ChallengeContentType  string
	ChallengeInsecureTLS  bool
	ChallengeTimeout      time.Duration
}

type RAConfig struct {
	BaseURL     string
	ContentType string
	InsecureTLS bool
	Timeout     time.Duration
	Logger      *log.Logger
}

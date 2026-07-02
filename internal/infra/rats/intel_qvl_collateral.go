//go:build intel_qvl && cgo

/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package rats

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

const (
	intelQVLFMSPCSize = 6
)

type intelQVLCollateralCache struct {
	dir    string
	logger *log.Logger
}

func newIntelQVLCollateralCache(dir string, logger *log.Logger) (*intelQVLCollateralCache, error) {
	if dir == "" {
		return nil, nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create Intel QVL collateral cache directory: %w", err)
	}
	return &intelQVLCollateralCache{dir: dir, logger: logger}, nil
}

func (c *intelQVLCollateralCache) Get(quote []byte) (*intelQVLQuoteCollateral, bool, string, error) {
	if c == nil {
		return nil, false, "", nil
	}
	path, key, err := c.pathForQuote(quote)
	if err != nil {
		return nil, false, "", err
	}
	collateral, err := os.ReadFile(path)
	if err == nil {
		if len(collateral) == 0 {
			return nil, false, key, fmt.Errorf("Intel QVL collateral cache file is empty: %s", path)
		}
		decoded, err := unmarshalIntelQVLQuoteCollateral(collateral)
		if err != nil {
			return nil, false, key, fmt.Errorf("decode Intel QVL collateral cache file: %w", err)
		}
		return decoded, true, key, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, key, nil
	}
	return nil, false, key, fmt.Errorf("read Intel QVL collateral cache file: %w", err)
}

func (c *intelQVLCollateralCache) Put(quote []byte, collateral *intelQVLQuoteCollateral) (string, error) {
	if c == nil {
		return "", nil
	}
	encoded, err := marshalIntelQVLQuoteCollateral(collateral)
	if err != nil {
		return "", err
	}
	path, key, err := c.pathForQuote(quote)
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(c.dir, ".collateral-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create Intel QVL collateral cache temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(encoded); err != nil {
		tmp.Close()
		return "", fmt.Errorf("write Intel QVL collateral cache temp file: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return "", fmt.Errorf("chmod Intel QVL collateral cache temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close Intel QVL collateral cache temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return "", fmt.Errorf("commit Intel QVL collateral cache file: %w", err)
	}
	return key, nil
}

func (c *intelQVLCollateralCache) pathForQuote(quote []byte) (string, string, error) {
	key, err := intelQVLCollateralCacheKey(quote)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(c.dir, key+".bin"), key, nil
}

func intelQVLCollateralCacheKey(quote []byte) (string, error) {
	if len(quote) == 0 {
		return "", errors.New("Intel QVL quote is empty")
	}
	fmspc, err := extractIntelQVLFMSPC(quote)
	if err == nil && len(fmspc) == intelQVLFMSPCSize {
		return "fmspc-" + hex.EncodeToString(fmspc), nil
	}
	sum := sha256.Sum256(quote)
	return "quote-" + hex.EncodeToString(sum[:16]), nil
}

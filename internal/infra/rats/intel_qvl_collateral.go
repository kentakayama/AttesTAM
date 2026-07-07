//go:build intel_qvl && cgo

/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package rats

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

const (
	intelQVLCollateralCacheTTL = 7 * 24 * time.Hour
)

type intelQVLCollateralCache struct {
	dir    string
	logger *log.Logger
}

type intelQVLCollateralCacheEntry struct {
	ExpiresAt  time.Time                `json:"expires_at"`
	Collateral *intelQVLQuoteCollateral `json:"collateral"`
}

var intelQVLNow = time.Now

// newIntelQVLCollateralCache creates the Verifier-side Endorsement store directory.
// The cache policy is intentionally owned by AttesTAM rather than QCNL/PCCS.
func newIntelQVLCollateralCache(dir string, logger *log.Logger) (*intelQVLCollateralCache, error) {
	if dir == "" {
		return nil, nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create Intel QVL collateral cache directory: %w", err)
	}
	return &intelQVLCollateralCache{dir: dir, logger: logger}, nil
}

// Get returns cached collateral only when the cache entry exists and is still valid.
// Missing or expired entries are treated as cache misses and are not surfaced as errors.
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
		decoded, err := unmarshalIntelQVLCollateralCacheEntry(collateral)
		if err != nil {
			return nil, false, key, fmt.Errorf("decode Intel QVL collateral cache file: %w", err)
		}
		if !decoded.ValidAt(intelQVLNow().UTC()) {
			_ = os.Remove(path)
			return nil, false, key, nil
		}
		return decoded.Collateral, true, key, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, key, nil
	}
	return nil, false, key, fmt.Errorf("read Intel QVL collateral cache file: %w", err)
}

// Put stores collateral together with the AttesTAM-managed expiration time.
// The effective expiration is the earlier of "now + 7 days" and the nearest collateral nextUpdate.
func (c *intelQVLCollateralCache) Put(quote []byte, collateral *intelQVLQuoteCollateral) (string, error) {
	if c == nil {
		return "", nil
	}
	entry, err := newIntelQVLCollateralCacheEntry(collateral, intelQVLNow().UTC())
	if err != nil {
		return "", err
	}
	encoded, err := marshalIntelQVLCollateralCacheEntry(entry)
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

// pathForQuote derives the cache filename for the supplied Quote.
func (c *intelQVLCollateralCache) pathForQuote(quote []byte) (string, string, error) {
	key, err := intelQVLCollateralCacheKey(quote)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(c.dir, key+".bin"), key, nil
}

// intelQVLCollateralCacheKey uses an FMSPC-derived key so collateral can be reused
// across Quotes from the same platform family.
func intelQVLCollateralCacheKey(quote []byte) (string, error) {
	if len(quote) == 0 {
		return "", errors.New("Intel QVL quote is empty")
	}
	fmspc, err := extractIntelQVLFMSPC(quote)
	if err != nil {
		return "", fmt.Errorf("extract FMSPC for Intel QVL collateral cache key: %w", err)
	}
	fmspcKeyValue, err := intelQVLFMSPCCacheKeyValue(fmspc)
	if err != nil {
		return "", fmt.Errorf("format FMSPC for Intel QVL collateral cache key: %w", err)
	}
	pckCA, err := extractIntelQVLQuotePCKCA(quote)
	if err != nil {
		return "", fmt.Errorf("extract PCK CA for Intel QVL collateral cache key: %w", err)
	}
	return "fmspc-" + fmspcKeyValue + "-pckca-" + pckCA, nil
}

// ValidAt reports whether the cached collateral entry is still valid at the supplied time.
func (e *intelQVLCollateralCacheEntry) ValidAt(now time.Time) bool {
	return e != nil && e.Collateral != nil && !e.ExpiresAt.IsZero() && now.Before(e.ExpiresAt)
}

// newIntelQVLCollateralCacheEntry computes the cache expiration managed by AttesTAM.
func newIntelQVLCollateralCacheEntry(collateral *intelQVLQuoteCollateral, now time.Time) (*intelQVLCollateralCacheEntry, error) {
	if collateral == nil {
		return nil, errors.New("Intel QVL collateral is nil")
	}
	expiresAt, err := intelQVLQuoteCollateralExpiry(collateral, now)
	if err != nil {
		return nil, err
	}
	return &intelQVLCollateralCacheEntry{
		ExpiresAt:  expiresAt,
		Collateral: collateral,
	}, nil
}

// intelQVLQuoteCollateralExpiry chooses the earlier of AttesTAM's fixed TTL and
// the nearest nextUpdate carried by the collateral payloads.
func intelQVLQuoteCollateralExpiry(collateral *intelQVLQuoteCollateral, now time.Time) (time.Time, error) {
	candidate := now.Add(intelQVLCollateralCacheTTL)
	nextUpdate, ok, err := intelQVLQuoteCollateralNextUpdate(collateral)
	if err != nil {
		return time.Time{}, err
	}
	if ok && nextUpdate.Before(candidate) {
		candidate = nextUpdate
	}
	return candidate, nil
}

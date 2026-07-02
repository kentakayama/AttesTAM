//go:build intel_qvl && cgo

/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package rats

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kentakayama/AttesTAM/internal/config"
)

func TestExtractIntelQVLQuotePCKCAFromFixture(t *testing.T) {
	quote := readIntelQVLQuoteFixture(t)

	pckCA, err := extractIntelQVLQuotePCKCA(quote)
	require.NoError(t, err)
	require.Contains(t, []string{"processor", "platform"}, pckCA)
}

func TestIntelQVLPCSClientFetch(t *testing.T) {
	quote := readIntelQVLQuoteFixture(t)
	fmspc, err := extractIntelQVLFMSPC(quote)
	require.NoError(t, err)
	pckCA, err := extractIntelQVLQuotePCKCA(quote)
	require.NoError(t, err)

	rootCRLBody := []byte{0x30, 0x03, 0x02, 0x01, 0x00}
	var rootCRLURL string
	issuerChain := makeTestIssuerChainPEM(t, func(cert *x509.Certificate) {
		cert.CRLDistributionPoints = []string{rootCRLURL}
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/root.crl":
			_, _ = w.Write(rootCRLBody)
		case "/sgx/certification/v4/pckcrl":
			require.Equal(t, pckCA, r.URL.Query().Get("ca"))
			require.Equal(t, "der", r.URL.Query().Get("encoding"))
			w.Header().Set(intelQVLPCSHeaderPCKCRL, url.QueryEscape(string(issuerChain)))
			_, _ = w.Write([]byte{0x01, 0x02, 0x03})
		case "/sgx/certification/v4/tcb":
			require.Equal(t, strings.ToUpper(hexEncode(fmspc)), r.URL.Query().Get("fmspc"))
			w.Header().Set(intelQVLPCSHeaderTCBInfo, url.QueryEscape(string(issuerChain)))
			_, _ = w.Write([]byte(`{"tcbInfo":{"id":"SGX"}}`))
		case "/sgx/certification/v4/qe/identity":
			w.Header().Set(intelQVLPCSHeaderQEIDChain, url.QueryEscape(string(issuerChain)))
			_, _ = w.Write([]byte(`{"enclaveIdentity":{"id":"QE"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	rootCRLURL = server.URL + "/root.crl"
	issuerChain = makeTestIssuerChainPEM(t, func(cert *x509.Certificate) {
		cert.CRLDistributionPoints = []string{rootCRLURL}
	})

	client, err := newIntelQVLPCSClient(config.RAConfig{
		IntelQVLPCSURL: server.URL + "/sgx/certification/v4",
		Timeout:        5 * time.Second,
	})
	require.NoError(t, err)

	collateral, err := client.Fetch(quote)
	require.NoError(t, err)
	require.Equal(t, uint16(intelQVLPCSAPIVersionMajor), collateral.MajorVersion)
	require.Equal(t, uint16(intelQVLPCSCRLVersionMinor), collateral.MinorVersion)
	require.Equal(t, []byte{0x01, 0x02, 0x03}, collateral.PCKCRL)
	require.Equal(t, rootCRLBody, collateral.RootCACRL)
	require.Equal(t, []byte(`{"tcbInfo":{"id":"SGX"}}`), collateral.TCBInfo)
	require.Equal(t, []byte(`{"enclaveIdentity":{"id":"QE"}}`), collateral.QEIdentity)
}

func makeTestIssuerChainPEM(t *testing.T, mutateRoot func(*x509.Certificate)) []byte {
	t.Helper()

	now := time.Now().UTC()
	rootKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	rootTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Intel SGX Root CA"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	if mutateRoot != nil {
		mutateRoot(rootTemplate)
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, &rootKey.PublicKey, rootKey)
	require.NoError(t, err)

	rootCert, err := x509.ParseCertificate(rootDER)
	require.NoError(t, err)
	intermediateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	intermediateTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "Intel SGX PCK Processor CA"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	intermediateDER, err := x509.CreateCertificate(rand.Reader, intermediateTemplate, rootCert, &intermediateKey.PublicKey, rootKey)
	require.NoError(t, err)

	var out []byte
	out = append(out, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: intermediateDER})...)
	out = append(out, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER})...)
	return out
}

func hexEncode(data []byte) string {
	const hexdigits = "0123456789abcdef"
	out := make([]byte, len(data)*2)
	for i, b := range data {
		out[i*2] = hexdigits[b>>4]
		out[i*2+1] = hexdigits[b&0x0f]
	}
	return string(out)
}

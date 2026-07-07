//go:build intel_qvl && cgo

/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package rats

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	fmspcQueryValue, err := intelQVLFMSPCQueryValue(fmspc)
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
		case "/sgx/certification/v4/crl":
			require.Equal(t, rootCRLURL, r.URL.Query().Get("uri"))
			_, _ = w.Write(rootCRLBody)
		case "/sgx/certification/v4/pckcrl":
			require.Equal(t, pckCA, r.URL.Query().Get("ca"))
			require.Equal(t, "der", r.URL.Query().Get("encoding"))
			w.Header().Set(intelQVLPCSHeaderPCKCRL, url.QueryEscape(string(issuerChain)))
			_, _ = w.Write([]byte{0x01, 0x02, 0x03})
		case "/sgx/certification/v4/tcb":
			require.Equal(t, fmspcQueryValue, r.URL.Query().Get("fmspc"))
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

	rootCRLURL = "https://certificates.trustedservices.intel.com/IntelSGXRootCA.der"
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

func TestIntelQVLPCSClientLogsFetchResultAtDebugLevel(t *testing.T) {
	var logBuf bytes.Buffer
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":"ok"}`))
	}))
	defer server.Close()

	client, err := newIntelQVLPCSClient(config.RAConfig{
		IntelQVLPCSURL: server.URL,
		Logger:         log.New(&logBuf, "", 0),
	})
	require.NoError(t, err)

	body, header, err := client.get("tcb", url.Values{"fmspc": []string{"001122334455"}})
	require.NoError(t, err)
	require.Equal(t, []byte(`{"result":"ok"}`), body)
	require.Equal(t, "application/json", header.Get("Content-Type"))

	logged := logBuf.String()
	require.Contains(t, logged, "DEBUG Intel QVL PCS fetch result")
	require.Contains(t, logged, server.URL+"/tcb?fmspc=001122334455")
	require.Contains(t, logged, "status=200 OK")
	require.Contains(t, logged, `body="{\"result\":\"ok\"}"`)
}

func TestIntelQVLPCSClientRootCACRLURL(t *testing.T) {
	const cdpURL = "https://certificates.trustedservices.intel.com/IntelSGXRootCA.der"

	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{
			name:    "Intel PCS uses CRL distribution point directly",
			baseURL: "https://api.trustedservices.intel.com/sgx/certification/v4",
			want:    cdpURL,
		},
		{
			name:    "Intel PCS subdomain uses CRL distribution point directly",
			baseURL: "https://validation.api.trustedservices.intel.com/sgx/certification/v4",
			want:    cdpURL,
		},
		{
			name:    "PCCS uses crl endpoint with CDP URI query",
			baseURL: "https://localhost:8081/sgx/certification/v4",
			want:    "https://localhost:8081/sgx/certification/v4/crl?uri=https%3A%2F%2Fcertificates.trustedservices.intel.com%2FIntelSGXRootCA.der",
		},
		{
			name:    "PCCS base URL with trailing slash uses crl endpoint",
			baseURL: "https://pccs.example.test/sgx/certification/v4/",
			want:    "https://pccs.example.test/sgx/certification/v4/crl?uri=https%3A%2F%2Fcertificates.trustedservices.intel.com%2FIntelSGXRootCA.der",
		},
		{
			name:    "Host suffix must be a real Intel trustedservices subdomain",
			baseURL: "https://fake-trustedservices.intel.com.example.test/sgx/certification/v4",
			want:    "https://fake-trustedservices.intel.com.example.test/sgx/certification/v4/crl?uri=https%3A%2F%2Fcertificates.trustedservices.intel.com%2FIntelSGXRootCA.der",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := newIntelQVLPCSClient(config.RAConfig{
				IntelQVLPCSURL: tt.baseURL,
			})
			require.NoError(t, err)

			require.Equal(t, tt.want, client.rootCACRLURL(cdpURL))
		})
	}
}

func TestIntelQVLPCSClientInsecureTLSAllowsSelfSignedHTTPS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"ok"}`))
	}))
	defer server.Close()

	secureClient, err := newIntelQVLPCSClient(config.RAConfig{
		IntelQVLPCSURL: server.URL,
	})
	require.NoError(t, err)

	_, _, err = secureClient.get("tcb", nil)
	require.Error(t, err)

	insecureClient, err := newIntelQVLPCSClient(config.RAConfig{
		IntelQVLPCSURL:         server.URL,
		IntelQVLPCSInsecureTLS: true,
	})
	require.NoError(t, err)

	body, _, err := insecureClient.get("tcb", nil)
	require.NoError(t, err)
	require.Equal(t, []byte(`{"result":"ok"}`), body)
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

//go:build intel_qvl && cgo

/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package rats

import (
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/kentakayama/AttesTAM/internal/config"
)

const (
	intelQVLDefaultPCSURL      = "https://api.trustedservices.intel.com/sgx/certification/v4"
	intelQVLPCSHeaderAPIKey    = "Ocp-Apim-Subscription-Key"
	intelQVLPCSHeaderPCKCRL    = "SGX-PCK-CRL-Issuer-Chain"
	intelQVLPCSHeaderTCBInfo   = "TCB-Info-Issuer-Chain"
	intelQVLPCSHeaderQEIDChain = "SGX-Enclave-Identity-Issuer-Chain"
	intelQVLPCSAPIVersionMajor = 3
	intelQVLPCSCRLVersionMinor = 1
	intelQVLPCSDefaultTimeout  = time.Minute
)

// See definition of sgx_ql_qve_collateral_t in
// https://download.01.org/intel-sgx/latest/dcap-latest/linux/docs/Intel_SGX_ECDSA_QuoteLibReference_DCAP_API.pdf
type intelQVLQuoteCollateral struct {
	MajorVersion          uint16 `json:"major_version"`
	MinorVersion          uint16 `json:"minor_version"`
	TEEType               uint32 `json:"tee_type"`
	PCKCRLIssuerChain     []byte `json:"pck_crl_issuer_chain"`
	RootCACRL             []byte `json:"root_ca_crl"`
	PCKCRL                []byte `json:"pck_crl"`
	TCBInfoIssuerChain    []byte `json:"tcb_info_issuer_chain"`
	TCBInfo               []byte `json:"tcb_info"`
	QEIdentityIssuerChain []byte `json:"qe_identity_issuer_chain"`
	QEIdentity            []byte `json:"qe_identity"`
}

type intelQVLNextUpdateCarrier struct {
	NextUpdate string `json:"nextUpdate"`
}

type intelQVLPCSClient struct {
	baseURL         *url.URL
	httpClient      *http.Client
	subscriptionKey string
	logger          *log.Logger
}

func newIntelQVLPCSClient(cfg config.RAConfig) (*intelQVLPCSClient, error) {
	baseURL := strings.TrimSpace(cfg.IntelQVLPCSURL)
	if baseURL == "" {
		baseURL = intelQVLDefaultPCSURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse Intel PCS URL: %w", err)
	}
	client := &http.Client{Timeout: cfg.Timeout}
	if client.Timeout == 0 {
		client.Timeout = intelQVLPCSDefaultTimeout
	}
	return &intelQVLPCSClient{
		baseURL:         parsed,
		httpClient:      client,
		subscriptionKey: strings.TrimSpace(cfg.IntelQVLSubscriptionKey),
		logger:          cfg.Logger,
	}, nil
}

func (c *intelQVLPCSClient) Fetch(quote []byte) (*intelQVLQuoteCollateral, error) {
	fmspc, err := extractIntelQVLFMSPC(quote)
	if err != nil {
		return nil, err
	}
	pckCA, err := extractIntelQVLQuotePCKCA(quote)
	if err != nil {
		return nil, err
	}

	pckCRL, pckCRLIssuerChain, err := c.fetchPCKCRL(pckCA)
	if err != nil {
		return nil, err
	}
	rootCACRL, err := c.fetchRootCACRL(pckCRLIssuerChain)
	if err != nil {
		return nil, err
	}
	tcbInfo, tcbInfoIssuerChain, err := c.fetchTCBInfo(fmspc)
	if err != nil {
		return nil, err
	}
	qeIdentity, qeIdentityIssuerChain, err := c.fetchQEIdentity()
	if err != nil {
		return nil, err
	}

	return &intelQVLQuoteCollateral{
		MajorVersion:          intelQVLPCSAPIVersionMajor,
		MinorVersion:          intelQVLPCSCRLVersionMinor,
		TEEType:               0, // 0x00000000: SGX or 0x00000081: TDX
		PCKCRLIssuerChain:     pckCRLIssuerChain,
		RootCACRL:             rootCACRL,
		PCKCRL:                pckCRL,
		TCBInfoIssuerChain:    tcbInfoIssuerChain,
		TCBInfo:               tcbInfo,
		QEIdentityIssuerChain: qeIdentityIssuerChain,
		QEIdentity:            qeIdentity,
	}, nil
}

func (c *intelQVLPCSClient) fetchPCKCRL(ca string) ([]byte, []byte, error) {
	body, header, err := c.get("pckcrl", url.Values{
		"ca":       []string{ca},
		"encoding": []string{"der"},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("fetch PCK CRL: %w", err)
	}
	issuerChain, err := decodeIntelPCSHeader(header.Get(intelQVLPCSHeaderPCKCRL))
	if err != nil {
		return nil, nil, fmt.Errorf("decode PCK CRL issuer chain: %w", err)
	}
	return body, issuerChain, nil
}

func (c *intelQVLPCSClient) fetchTCBInfo(fmspc []byte) ([]byte, []byte, error) {
	body, header, err := c.get("tcb", url.Values{
		"fmspc": []string{strings.ToUpper(hex.EncodeToString(fmspc))},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("fetch TCB Info: %w", err)
	}
	issuerChain, err := decodeIntelPCSHeader(header.Get(intelQVLPCSHeaderTCBInfo))
	if err != nil {
		return nil, nil, fmt.Errorf("decode TCB Info issuer chain: %w", err)
	}
	return body, issuerChain, nil
}

func (c *intelQVLPCSClient) fetchQEIdentity() ([]byte, []byte, error) {
	body, header, err := c.get("qe/identity", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch QE Identity: %w", err)
	}
	issuerChain, err := decodeIntelPCSHeader(header.Get(intelQVLPCSHeaderQEIDChain))
	if err != nil {
		return nil, nil, fmt.Errorf("decode QE Identity issuer chain: %w", err)
	}
	return body, issuerChain, nil
}

func (c *intelQVLPCSClient) fetchRootCACRL(issuerChain []byte) ([]byte, error) {
	rootCert, err := lastCertificateFromPEM(issuerChain)
	if err != nil {
		return nil, fmt.Errorf("parse root CA certificate: %w", err)
	}
	if len(rootCert.CRLDistributionPoints) == 0 {
		return nil, fmt.Errorf("root CA certificate CRL distribution point missing")
	}
	body, _, err := c.getAbsolute(rootCert.CRLDistributionPoints[0])
	if err != nil {
		return nil, fmt.Errorf("fetch root CA CRL: %w", err)
	}
	if block, _ := pem.Decode(body); block != nil {
		return block.Bytes, nil
	}
	return body, nil
}

func (c *intelQVLPCSClient) get(endpoint string, query url.Values) ([]byte, http.Header, error) {
	u := *c.baseURL
	u.Path = path.Join(c.baseURL.Path, endpoint)
	u.RawQuery = query.Encode()
	return c.getAbsolute(u.String())
}

func (c *intelQVLPCSClient) getAbsolute(rawURL string) ([]byte, http.Header, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, err
	}
	if c.subscriptionKey != "" {
		req.Header.Set(intelQVLPCSHeaderAPIKey, c.subscriptionKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	c.logFetchResult(rawURL, resp, body)
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("GET %s returned %s: %s", rawURL, resp.Status, strings.TrimSpace(string(body)))
	}
	return body, resp.Header.Clone(), nil
}

func (c *intelQVLPCSClient) logFetchResult(rawURL string, resp *http.Response, body []byte) {
	if c.logger == nil {
		return
	}
	c.logger.Printf("DEBUG Intel QVL PCS fetch result: url=%s status=%s headers=%v body_len=%d body=%q",
		rawURL,
		resp.Status,
		resp.Header,
		len(body),
		body,
	)
}

func decodeIntelPCSHeader(value string) ([]byte, error) {
	if strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("required Intel PCS header missing")
	}
	decoded, err := url.QueryUnescape(value)
	if err != nil {
		return nil, err
	}
	return []byte(decoded), nil
}

func lastCertificateFromPEM(chain []byte) (*x509.Certificate, error) {
	rest := chain
	var last *x509.Certificate
	for len(rest) > 0 {
		block, remaining := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = remaining
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		last = cert
	}
	if last == nil {
		return nil, fmt.Errorf("no certificate found in PEM chain")
	}
	return last, nil
}

func marshalIntelQVLQuoteCollateral(collateral *intelQVLQuoteCollateral) ([]byte, error) {
	if collateral == nil {
		return nil, fmt.Errorf("Intel QVL collateral is nil")
	}
	return json.Marshal(collateral)
}

func unmarshalIntelQVLQuoteCollateral(data []byte) (*intelQVLQuoteCollateral, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("Intel QVL collateral is empty")
	}
	var collateral intelQVLQuoteCollateral
	if err := json.Unmarshal(data, &collateral); err != nil {
		return nil, fmt.Errorf("decode Intel QVL collateral: %w", err)
	}
	return &collateral, nil
}

func marshalIntelQVLCollateralCacheEntry(entry *intelQVLCollateralCacheEntry) ([]byte, error) {
	if entry == nil {
		return nil, fmt.Errorf("Intel QVL collateral cache entry is nil")
	}
	return json.Marshal(entry)
}

func unmarshalIntelQVLCollateralCacheEntry(data []byte) (*intelQVLCollateralCacheEntry, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("Intel QVL collateral cache entry is empty")
	}
	var entry intelQVLCollateralCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("decode Intel QVL collateral cache entry: %w", err)
	}
	return &entry, nil
}

func intelQVLQuoteCollateralNextUpdate(collateral *intelQVLQuoteCollateral) (time.Time, bool, error) {
	if collateral == nil {
		return time.Time{}, false, fmt.Errorf("Intel QVL collateral is nil")
	}

	var nearest time.Time
	for _, raw := range [][]byte{collateral.RootCACRL, collateral.PCKCRL} {
		nextUpdate, ok, err := intelQVLParseCRLNextUpdate(raw)
		if err != nil {
			return time.Time{}, false, err
		}
		if ok && (nearest.IsZero() || nextUpdate.Before(nearest)) {
			nearest = nextUpdate
		}
	}
	for _, raw := range [][]byte{collateral.TCBInfo, collateral.QEIdentity} {
		nextUpdate, ok, err := intelQVLParseJSONNextUpdate(raw)
		if err != nil {
			return time.Time{}, false, err
		}
		if ok && (nearest.IsZero() || nextUpdate.Before(nearest)) {
			nearest = nextUpdate
		}
	}
	if nearest.IsZero() {
		return time.Time{}, false, nil
	}
	return nearest, true, nil
}

func intelQVLParseCRLNextUpdate(raw []byte) (time.Time, bool, error) {
	if len(raw) == 0 {
		return time.Time{}, false, nil
	}
	crl, err := x509.ParseRevocationList(raw)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("parse CRL nextUpdate: %w", err)
	}
	if crl.NextUpdate.IsZero() {
		return time.Time{}, false, nil
	}
	return crl.NextUpdate.UTC(), true, nil
}

func intelQVLParseJSONNextUpdate(raw []byte) (time.Time, bool, error) {
	if len(raw) == 0 {
		return time.Time{}, false, nil
	}
	var carrier intelQVLNextUpdateCarrier
	if err := json.Unmarshal(raw, &carrier); err != nil {
		return time.Time{}, false, fmt.Errorf("parse JSON nextUpdate: %w", err)
	}
	if strings.TrimSpace(carrier.NextUpdate) == "" {
		return time.Time{}, false, nil
	}
	nextUpdate, err := time.Parse(time.RFC3339, carrier.NextUpdate)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("parse nextUpdate timestamp %q: %w", carrier.NextUpdate, err)
	}
	return nextUpdate.UTC(), true, nil
}

func extractIntelQVLQuotePCKCA(quote []byte) (string, error) {
	certChain, err := extractIntelQVLQuotePCKCertChain(quote)
	if err != nil {
		return "", err
	}
	certs, err := parseIntelQVLQuoteCertificates(certChain)
	if err != nil {
		return "", fmt.Errorf("parse PCK certificate chain: %w", err)
	}
	if len(certs) < 2 {
		return "", fmt.Errorf("PCK certificate chain is incomplete")
	}
	issuerCN := strings.ToLower(certs[1].Subject.CommonName)
	switch {
	case strings.Contains(issuerCN, "processor ca"):
		return "processor", nil
	case strings.Contains(issuerCN, "platform ca"):
		return "platform", nil
	default:
		return "", fmt.Errorf("unsupported PCK CA %q", certs[1].Subject.CommonName)
	}
}

func parseIntelQVLQuoteCertificates(certChain []byte) ([]*x509.Certificate, error) {
	if certs, err := x509.ParseCertificates(certChain); err == nil && len(certs) > 0 {
		return certs, nil
	}

	var certs []*x509.Certificate
	rest := certChain
	for len(rest) > 0 {
		block, remaining := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = remaining
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
	}
	if len(certs) == 0 {
		return nil, fmt.Errorf("no certificates found")
	}
	return certs, nil
}

func extractIntelQVLQuotePCKCertChain(quote []byte) ([]byte, error) {
	if len(quote) < sgxQuote3Size {
		return nil, fmt.Errorf("quote is too short")
	}
	version := binary.LittleEndian.Uint16(quote[:quoteHeaderVersionSize])
	if version != quoteVersion3 && version != quoteVersion4 {
		return nil, fmt.Errorf("unsupported quote version %d for PCK chain extraction", version)
	}

	signatureDataLen := binary.LittleEndian.Uint32(quote[432:436])
	if len(quote) < sgxQuote3Size+int(signatureDataLen) {
		return nil, fmt.Errorf("quote signature data is truncated")
	}

	offset := sgxQuote3Size
	const fixedECDSASigDataSize = 64 + 64 + sgxReportBodySize + 64
	if int(signatureDataLen) < fixedECDSASigDataSize+2+6 {
		return nil, fmt.Errorf("quote signature data is too short")
	}
	offset += fixedECDSASigDataSize

	authDataSize := int(binary.LittleEndian.Uint16(quote[offset : offset+2]))
	offset += 2 + authDataSize
	if len(quote) < offset+6 {
		return nil, fmt.Errorf("quote certification data is truncated")
	}

	certKeyType := binary.LittleEndian.Uint16(quote[offset : offset+2])
	certDataSize := int(binary.LittleEndian.Uint32(quote[offset+2 : offset+6]))
	offset += 6
	if len(quote) < offset+certDataSize {
		return nil, fmt.Errorf("quote certification data payload is truncated")
	}
	if certKeyType != 5 {
		return nil, fmt.Errorf("unsupported quote certification data type %d", certKeyType)
	}

	certData := quote[offset : offset+certDataSize]
	return append([]byte(nil), certData...), nil
}

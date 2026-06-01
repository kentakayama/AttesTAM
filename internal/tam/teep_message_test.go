/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package tam

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/AttesTAM/internal/suit"
	"github.com/kentakayama/AttesTAM/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/veraison/go-cose"
)

func loadHexTestData(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return data
}

func TestTEEPMessage_RoundTrip_QueryRequest_OK(t *testing.T) {
	// [
	//   / type: / 1 / TEEP-TYPE-query-request /,
	//   / options: /
	//   {
	//     / versions / 3 : [ 0 ]  / 0 is current TEEP Protocol /,
	//     / token / 19 : h'A0A1A2A3A4A5A6A7A8A9AAABACADAEAF'
	//   },
	//   / supported-teep-cipher-suites: / [
	//     [ [ 18, -9 ] ] / Sign1 using ES256 with P-256 curve /,
	//     [ [ 18, -19 ] ] / Sign1 using EdDSA with Ed25519 curve /
	//   ],
	//   / supported-suit-cose-profiles: / [
	//     [-16, -9, -29, -65534] / suit-sha256-esp256-ecdh-a128ctr /,
	//     [-16, -19, -29, -65534] / suit-sha256-ed25519-ecdh-a128ctr /,
	//     [-16, -9, -29, 1]      / suit-sha256-esp256-ecdh-a128gcm /,
	//     [-16, -19, -29, 24]     / suit-sha256-ed25519-ecdh-chacha-poly /
	//   ],
	//   / data-item-requested: / 3 / attestation | trusted-components /
	// ]
	encodedQueryRequest := loadHexTestData(t, "query_request.cbor")

	var queryRequest TEEPMessage
	err := cbor.Unmarshal(encodedQueryRequest, &queryRequest)
	require.Nil(t, err)
	assert.Equal(t, TEEPTypeQueryRequest, queryRequest.Type)
	assert.Equal(t, util.DiagList[TEEPVersion]{0}, queryRequest.Options.Versions)
	assert.Equal(t, util.BytesHexMax32([]byte{
		0xa0, 0xa1, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xab, 0xac, 0xad, 0xae, 0xaf,
	}), queryRequest.Options.Token)
	assert.Equal(t, 2, len(queryRequest.SupportedTEEPCipherSuites))
	assert.Equal(t, TEEPCipherSuite{Type: 18, Algorithm: -9}, queryRequest.SupportedTEEPCipherSuites[0][0])
	assert.Equal(t, TEEPCipherSuite{Type: 18, Algorithm: -19}, queryRequest.SupportedTEEPCipherSuites[1][0])
	assert.Equal(t, suit.COSEProfile{DigestAlg: -16, AuthAlg: -9, KeyExchangeAlg: -29, EncryptionAlg: -65534}, queryRequest.SupportedSUITCOSEProfiles[0])
	assert.Equal(t, RequestDataItem(true, true, false, false), queryRequest.DataItemRequested)

	encoded, err := cbor.Marshal(queryRequest)
	assert.Nil(t, err)
	assert.Equal(t, encodedQueryRequest, encoded)
}

func TestTEEPMessage_RoundTrip_QueryResponse_OK(t *testing.T) {
	// / query-response = /
	// [
	//   / type: / 2 / TEEP-TYPE-query-response /,
	//   / options: /
	//   {
	//     / selected-version / 5 : 0,
	//     / tc-list / 7 : [
	//       {
	//         / system-component-id / 0 : [ h'0102030405060708090A0B0C0D0E0F' ],
	//         / suit-parameter-image-digest / 3: << [
	//           / suit-digest-algorithm-id / -16 / SHA256 /,
	//           / suit-digest-bytes / h'A7FD6593EAC32EB4BE578278E6540C5C09CFD7D4D234973054833B2B93030609'
	//             / SHA256 digest of tc binary /
	//         ] >>
	//       }
	//     ],
	//     / token / 19 : h'A0A1A2A3A4A5A6A7A8A9AAABACADAEAF'
	//   }
	// ]
	encodedQueryResponse := loadHexTestData(t, "query_response.cbor")

	var queryResponse TEEPMessage
	err := cbor.Unmarshal(encodedQueryResponse, &queryResponse)
	assert.Nil(t, err)
	assert.Equal(t, TEEPTypeQueryResponse, queryResponse.Type)
	assert.Equal(t, TEEPVersion(0), *queryResponse.Options.SelectedVersion)
	require.NotNil(t, queryResponse.Options.TCList)
	assert.Equal(t, 1, len(queryResponse.Options.TCList))
	assert.Equal(t, suit.ComponentID{[]byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	}}, (queryResponse.Options.TCList)[0].SystemComponentID)
	var digest suit.Digest
	err = cbor.Unmarshal((queryResponse.Options.TCList)[0].ImageDigest, &digest)
	assert.Nil(t, err)
	assert.Equal(t, cose.AlgorithmSHA256, digest.DigestAlg)
	assert.Equal(t, []byte{
		0xa7, 0xfd, 0x65, 0x93, 0xea, 0xc3, 0x2e, 0xb4, 0xbe, 0x57, 0x82, 0x78, 0xe6, 0x54, 0x0c, 0x5c,
		0x09, 0xcf, 0xd7, 0xd4, 0xd2, 0x34, 0x97, 0x30, 0x54, 0x83, 0x3b, 0x2b, 0x93, 0x03, 0x06, 0x09,
	}, digest.DigestBytes)
	assert.Equal(t, util.BytesHexMax32([]byte{
		0xa0, 0xa1, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xab, 0xac, 0xad, 0xae, 0xaf,
	}), queryResponse.Options.Token)

	encoded, err := cbor.Marshal(queryResponse)
	assert.Nil(t, err)
	assert.Equal(t, encodedQueryResponse, encoded)
}

func TestTEEPMessage_RoundTrip_Update_OK(t *testing.T) {
	// / update = /
	// [
	//   / type: / 3 / TEEP-TYPE-update /,
	//   / options: /
	//   {
	//     / manifest-list / 9 : [
	//       <<
	//         / SUIT_Envelope / {
	//           / suit-authentication-wrapper / 2: << [
	//             << [
	//               / suit-digest-algorithm-id: / -16 / suit-cose-alg-sha256 /,
	//               / suit-digest-bytes: / h'DB601ADE73092B58532CA03FBB663DE49532435336F1558B49BB622726A2FEDD'
	//             ] >>,
	//             << / COSE_Sign1_Tagged / 18( [
	//               / protected: / << {
	//                 / algorithm-id / 1: -7 / ES256 /
	//               } >>,
	//               / unprotected: / {},
	//               / payload: / null,
	//               / signature: / h'5B2D535A2B6D5E3C585C1074F414DA9E10BD285C99A33916DADE3ED38812504817AC48B62B8E984EC622785BD1C411888BE531B1B594507816B201F6F28579A4'
	//             ] ) >>
	//           ] >>,
	//           / suit-manifest / 3: << {
	//             / suit-manifest-version / 1: 1,
	//             / suit-manifest-sequence-number / 2: 3,
	//             / suit-common / 3: << {
	//               / suit-components / 2: [
	//                 [
	//                   h'544545502D446576696365',           / "TEEP-Device" /
	//                   h'5365637572654653',                 / "SecureFS" /
	//                   h'8D82573A926D4754935332DC29997F74', / tc-uuid /
	//                   h'7461'                              / "ta" /
	//                 ]
	//               ],
	//               / suit-common-sequence / 4: << [
	//                 / suit-directive-override-parameters / 20, {
	//                   / suit-parameter-vendor-identifier / 1: h'C0DDD5F15243566087DB4F5B0AA26C2F',
	//                   / suit-parameter-class-identifier / 2: h'DB42F7093D8C55BAA8C5265FC5820F4E',
	//                   / suit-parameter-image-digest / 3: << [
	//                     / suit-digest-algorithm-id: / -16 / suit-cose-alg-sha256 /,
	//                     / suit-digest-bytes: / h'8CF71AC86AF31BE184EC7A05A411A8C3A14FD9B77A30D046397481469468ECE8'
	//                   ] >>,
	//                   / suit-parameter-image-size / 14: 20
	//                 },
	//                 / suit-condition-vendor-identifier / 1, 15,
	//                 / suit-condition-class-identifier / 2, 15
	//               ] >>
	//             } >>,
	//             / suit-install / 9: << [
	//               / suit-directive-override-parameters / 20, {
	//                 / suit-parameter-uri / 21: "https://example.org/8d82573a-926d-4754-9353-32dc29997f74.ta"
	//               },
	//               / suit-directive-fetch / 21, 15,
	//               / suit-condition-image-match / 3, 15
	//             ] >>
	//           } >>
	//         }
	//       >>
	//     ] / array of bstr wrapped SUIT_Envelope /,
	//     / token / 19 : h'A0A1A2A3A4A5A6A7A8A9AAABACADAEAF'
	//   }
	// ]
	encodedUpdate := loadHexTestData(t, "update.cbor")

	var update TEEPMessage
	err := cbor.Unmarshal(encodedUpdate, &update)
	assert.Nil(t, err)
	assert.Equal(t, TEEPTypeUpdate, update.Type)
	assert.Equal(t, 1, len(update.Options.ManifestList))
	assert.Equal(t, util.BytesHexMax32([]byte{
		0xa2, 0x02, 0x58, 0x73, 0x82, 0x58, 0x24, 0x82, 0x2f, 0x58, 0x20, 0xdb, 0x60, 0x1a, 0xde, 0x73,
		0x09, 0x2b, 0x58, 0x53, 0x2c, 0xa0, 0x3f, 0xbb, 0x66, 0x3d, 0xe4, 0x95, 0x32, 0x43, 0x53, 0x36,
		0xf1, 0x55, 0x8b, 0x49, 0xbb, 0x62, 0x27, 0x26, 0xa2, 0xfe, 0xdd, 0x58, 0x4a, 0xd2, 0x84, 0x43,
		0xa1, 0x01, 0x26, 0xa0, 0xf6, 0x58, 0x40, 0x5b, 0x2d, 0x53, 0x5a, 0x2b, 0x6d, 0x5e, 0x3c, 0x58,
		0x5c, 0x10, 0x74, 0xf4, 0x14, 0xda, 0x9e, 0x10, 0xbd, 0x28, 0x5c, 0x99, 0xa3, 0x39, 0x16, 0xda,
		0xde, 0x3e, 0xd3, 0x88, 0x12, 0x50, 0x48, 0x17, 0xac, 0x48, 0xb6, 0x2b, 0x8e, 0x98, 0x4e, 0xc6,
		0x22, 0x78, 0x5b, 0xd1, 0xc4, 0x11, 0x88, 0x8b, 0xe5, 0x31, 0xb1, 0xb5, 0x94, 0x50, 0x78, 0x16,
		0xb2, 0x01, 0xf6, 0xf2, 0x85, 0x79, 0xa4, 0x03, 0x58, 0xd4, 0xa4, 0x01, 0x01, 0x02, 0x03, 0x03,
		0x58, 0x84, 0xa2, 0x02, 0x81, 0x84, 0x4b, 0x54, 0x45, 0x45, 0x50, 0x2d, 0x44, 0x65, 0x76, 0x69,
		0x63, 0x65, 0x48, 0x53, 0x65, 0x63, 0x75, 0x72, 0x65, 0x46, 0x53, 0x50, 0x8d, 0x82, 0x57, 0x3a,
		0x92, 0x6d, 0x47, 0x54, 0x93, 0x53, 0x32, 0xdc, 0x29, 0x99, 0x7f, 0x74, 0x42, 0x74, 0x61, 0x04,
		0x58, 0x54, 0x86, 0x14, 0xa4, 0x01, 0x50, 0xc0, 0xdd, 0xd5, 0xf1, 0x52, 0x43, 0x56, 0x60, 0x87,
		0xdb, 0x4f, 0x5b, 0x0a, 0xa2, 0x6c, 0x2f, 0x02, 0x50, 0xdb, 0x42, 0xf7, 0x09, 0x3d, 0x8c, 0x55,
		0xba, 0xa8, 0xc5, 0x26, 0x5f, 0xc5, 0x82, 0x0f, 0x4e, 0x03, 0x58, 0x24, 0x82, 0x2f, 0x58, 0x20,
		0x8c, 0xf7, 0x1a, 0xc8, 0x6a, 0xf3, 0x1b, 0xe1, 0x84, 0xec, 0x7a, 0x05, 0xa4, 0x11, 0xa8, 0xc3,
		0xa1, 0x4f, 0xd9, 0xb7, 0x7a, 0x30, 0xd0, 0x46, 0x39, 0x74, 0x81, 0x46, 0x94, 0x68, 0xec, 0xe8,
		0x0e, 0x14, 0x01, 0x0f, 0x02, 0x0f, 0x09, 0x58, 0x45, 0x86, 0x14, 0xa1, 0x15, 0x78, 0x3b, 0x68,
		0x74, 0x74, 0x70, 0x73, 0x3a, 0x2f, 0x2f, 0x65, 0x78, 0x61, 0x6d, 0x70, 0x6c, 0x65, 0x2e, 0x6f,
		0x72, 0x67, 0x2f, 0x38, 0x64, 0x38, 0x32, 0x35, 0x37, 0x33, 0x61, 0x2d, 0x39, 0x32, 0x36, 0x64,
		0x2d, 0x34, 0x37, 0x35, 0x34, 0x2d, 0x39, 0x33, 0x35, 0x33, 0x2d, 0x33, 0x32, 0x64, 0x63, 0x32,
		0x39, 0x39, 0x39, 0x37, 0x66, 0x37, 0x34, 0x2e, 0x74, 0x61, 0x15, 0x0f, 0x03, 0x0f,
	}), update.Options.ManifestList[0])
	assert.Equal(t, util.BytesHexMax32([]byte{
		0xa0, 0xa1, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xab, 0xac, 0xad, 0xae, 0xaf,
	}), update.Options.Token)

	encoded, err := cbor.Marshal(update)
	assert.Nil(t, err)
	assert.Equal(t, encodedUpdate, encoded)
}

func TestTEEPMessage_RoundTrip_Success_OK(t *testing.T) {
	/*
		[
			/ type: / e'TEEP-TYPE-success',
			/ options: / {
				e'token': h'0011223344556677',
				e'suit-reports': [
					<<
						{
							e'suit-reference': [
								/ suit-report-manifest-uri: / "",
								/ suit-report-manifest-digest: / [
									e'cose-alg-sha-256',
									h'6658EA560262696DD1F13B782239A064DA7C6C5CBAF52FDED428A6FC83C7E5AF'
								]
							],
							e'suit-report-records': [
								/ system-property-claims = / {
									e'system-component-id': ['app.wasm'],
									e'suit-parameter-image-size': 65132,
									e'suit-parameter-image-digest': << [
										e'cose-alg-sha-256',
										h'21D2E4B19E44C1906E58050577922D604C204CCC2FD2CB977FEE8ED56B62FD36'
									] >>
								}
							],
							e'suit-report-result': true
						}
					>>
				]
			}
		]
	*/
	encodedSuccess := loadHexTestData(t, "success.cbor")

	var success TEEPMessage
	err := cbor.Unmarshal(encodedSuccess, &success)
	require.Nil(t, err)
	assert.Equal(t, TEEPTypeSuccess, success.Type)
	assert.Equal(t, util.BytesHexMax32([]byte{
		0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
	}), success.Options.Token)

	assert.Equal(t, 1, len(success.Options.SUITReports))
	var report suit.Report
	err = cbor.Unmarshal(success.Options.SUITReports[0], &report)
	require.Nil(t, err)
	assert.Equal(t, "", report.Reference.ManifestUri)
	assert.Equal(t, suit.Digest{
		DigestAlg: cose.AlgorithmSHA256,
		DigestBytes: []byte{
			0x66, 0x58, 0xEA, 0x56, 0x02, 0x62, 0x69, 0x6D, 0xD1, 0xF1, 0x3B, 0x78, 0x22, 0x39, 0xA0, 0x64,
			0xDA, 0x7C, 0x6C, 0x5C, 0xBA, 0xF5, 0x2F, 0xDE, 0xD4, 0x28, 0xA6, 0xFC, 0x83, 0xC7, 0xE5, 0xAF,
		},
	}, report.Reference.Digest)
	require.Equal(t, 1, len(report.ReportRecords))
	require.NotNil(t, report.ReportRecords[0].SystemClaims)
	assert.Equal(t, suit.ComponentID{{0x61, 0x70, 0x70, 0x2E, 0x77, 0x61, 0x73, 0x6D}}, report.ReportRecords[0].SystemClaims.SystemComponentID)
	assert.Equal(t, uint(65132), report.ReportRecords[0].SystemClaims.ImageSize)
	expectedDigest := []byte{
		0x82, 0x2F, // [-16,
		0x58, 0x20, // h'
		0x21, 0xD2, 0xE4, 0xB1, 0x9E, 0x44, 0xC1, 0x90, 0x6E, 0x58, 0x05, 0x05, 0x77, 0x92, 0x2D, 0x60,
		0x4C, 0x20, 0x4C, 0xCC, 0x2F, 0xD2, 0xCB, 0x97, 0x7F, 0xEE, 0x8E, 0xD5, 0x6B, 0x62, 0xFD, 0x36,
	}
	assert.Equal(t, expectedDigest, report.ReportRecords[0].SystemClaims.ImageDigest)
	assert.Nil(t, report.ReportRecords[0].Record)
	require.NotNil(t, report.Result.True)
	assert.Equal(t, true, *report.Result.True)
	assert.Nil(t, report.Result.DetailedResult)

	encoded, err := cbor.Marshal(success)
	assert.Nil(t, err)
	assert.Equal(t, encodedSuccess, encoded)
}

func TestTEEPMessage_RoundTrip_Error_OK(t *testing.T) {
	/*
		[
			/ type: / e'TEEP-TYPE-error',
			/ options: / {
				e'token': h'0011223344556677',
				e'suit-reports': [
					<<
						{
							e'suit-reference': [
								/ suit-report-manifest-uri: / "",
								/ suit-report-manifest-digest: / [
									e'cose-alg-sha-256',
									h'6658EA560262696DD1F13B782239A064DA7C6C5CBAF52FDED428A6FC83C7E5AF'
								]
							],
							e'suit-report-records': [
								/ system-property-claims = / {
									e'system-component-id': ['app.wasm'],
									e'suit-parameter-image-size': 65132
								}
							],
							e'suit-report-result': {
								e'suit-report-result-code': -1,
								e'suit-report-result-record': [
									/ suit-record-manifest-id: / [],
									/ suit-record-manifest-section: / e'suit-payload-fetch',
									/ suit-record-section-offset: / 16 / at suit-condition-image-match' /,
									/ suit-record-component-index: / 0,
									/ suit-record-properties: / {
										e'suit-parameter-image-digest': << [
											e'cose-alg-sha-256',
											h'21D2E4B19E44C1906E58050577922D604C204CCC2FD2CB977FEE8ED56B62FD30' / a bit different from app.wasm /
										] >>
									}
								],
								e'suit-report-result-reason': e'suit-report-reason-condition-failed'
							}
						}
					>>
				]
			},
			e'ERR_MANIFEST_PROCESSING_FAILED'
		]
	*/
	encodedError := loadHexTestData(t, "error.cbor")

	var teepError TEEPMessage
	err := cbor.Unmarshal(encodedError, &teepError)
	assert.Nil(t, err)
	assert.Equal(t, TEEPTypeError, teepError.Type)
	assert.Equal(t, util.BytesHexMax32([]byte{
		0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
	}), teepError.Options.Token)
	assert.Equal(t, TEEPErrCode(11), teepError.ErrCode)

	assert.Equal(t, 1, len(teepError.Options.SUITReports))
	var report suit.Report
	err = cbor.Unmarshal(teepError.Options.SUITReports[0], &report)
	require.Nil(t, err)
	assert.Equal(t, "", report.Reference.ManifestUri)
	assert.Equal(t, suit.Digest{
		DigestAlg: cose.AlgorithmSHA256,
		DigestBytes: []byte{
			0x66, 0x58, 0xEA, 0x56, 0x02, 0x62, 0x69, 0x6D, 0xD1, 0xF1, 0x3B, 0x78, 0x22, 0x39, 0xA0, 0x64,
			0xDA, 0x7C, 0x6C, 0x5C, 0xBA, 0xF5, 0x2F, 0xDE, 0xD4, 0x28, 0xA6, 0xFC, 0x83, 0xC7, 0xE5, 0xAF,
		},
	}, report.Reference.Digest)
	require.Equal(t, 1, len(report.ReportRecords))
	require.NotNil(t, report.ReportRecords[0].SystemClaims)
	assert.Equal(t, suit.ComponentID{{0x61, 0x70, 0x70, 0x2E, 0x77, 0x61, 0x73, 0x6D}}, report.ReportRecords[0].SystemClaims.SystemComponentID)
	assert.Equal(t, uint(65132), report.ReportRecords[0].SystemClaims.ImageSize)
	assert.Nil(t, report.ReportRecords[0].SystemClaims.ImageDigest)
	assert.Nil(t, report.ReportRecords[0].Record)
	assert.Nil(t, report.Result.True)
	require.NotNil(t, report.Result.DetailedResult)
	assert.Equal(t, -1, report.Result.DetailedResult.ResultCode)
	assert.Equal(t, []uint{}, report.Result.DetailedResult.ResultRecord.ManifestID)
	assert.Equal(t, 16, report.Result.DetailedResult.ResultRecord.ManifestSection)
	assert.Equal(t, uint(16), report.Result.DetailedResult.ResultRecord.SectionOffset)
	assert.Equal(t, uint(0), report.Result.DetailedResult.ResultRecord.ComponentIndex)
	expectedIncorrectDigest := []byte{
		0x82, 0x2F, // [-16,
		0x58, 0x20, // h'
		0x21, 0xD2, 0xE4, 0xB1, 0x9E, 0x44, 0xC1, 0x90, 0x6E, 0x58, 0x05, 0x05, 0x77, 0x92, 0x2D, 0x60,
		0x4C, 0x20, 0x4C, 0xCC, 0x2F, 0xD2, 0xCB, 0x97, 0x7F, 0xEE, 0x8E, 0xD5, 0x6B, 0x62, 0xFD, 0x30,
	}
	properties := make(map[int]any)
	properties[3] = expectedIncorrectDigest
	assert.Equal(t, properties, report.Result.DetailedResult.ResultRecord.RecordProperties)

	encoded, err := cbor.Marshal(teepError)
	assert.Nil(t, err)
	assert.Equal(t, encodedError, encoded)
}

func TestTEEPMessage_AgentQueryResponse(t *testing.T) {
	encodedQueryResponse := loadHexTestData(t, "query_response_psa.cbor")
	var queryResponse TEEPMessage
	err := cbor.Unmarshal(encodedQueryResponse, &queryResponse)
	assert.Nil(t, err)
	assert.Equal(t, TEEPTypeQueryResponse, queryResponse.Type)
}

func TestTEEPMessage_SignVerify_OK(t *testing.T) {
	var key cose.Key
	err := cbor.Unmarshal(loadHexTestData(t, "tam_key.cbor"), &key)
	require.Nil(t, err)

	message := TEEPMessage{
		Type: TEEPTypeQueryRequest,
		Options: TEEPOptions{
			Versions: util.DiagList[TEEPVersion]{0},
			Token:    []byte{0xa0, 0xa1, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xab, 0xac, 0xad, 0xae, 0xaf},
			SupportedTEEPCipherSuites: util.DiagList[util.DiagList[TEEPCipherSuite]]{
				{
					{
						Type:      cose.CBORTagSign1Message,
						Algorithm: int(cose.AlgorithmESP256),
					},
				},
				{
					{
						Type:      cose.CBORTagSign1Message,
						Algorithm: int(cose.AlgorithmEd25519EdDSA),
					},
				},
			},
			SupportedSUITCOSEProfiles: util.DiagList[suit.COSEProfile]{
				{
					DigestAlg:      cose.AlgorithmSHA256,
					AuthAlg:        cose.AlgorithmESP256,
					KeyExchangeAlg: cose.Algorithm(-29),
					EncryptionAlg:  cose.Algorithm(-65534),
				},
			},
		},
		DataItemRequested: RequestDataItem(true, true, false, false),
	}

	signed, err := message.COSESign1Sign(&key)
	require.Nil(t, err)

	var payload TEEPMessage
	err = payload.COSESign1Verify(&key, signed)
	require.Nil(t, err)
	assert.Equal(t, message, payload)
}

func TestTEEPMessage_String(t *testing.T) {
	m1 := TEEPMessage{
		Type: TEEPTypeQueryRequest,
		Options: TEEPOptions{
			Challenge: util.BytesHexMax32{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F},
			Versions:  util.DiagList[TEEPVersion]{0},
		},
		SupportedTEEPCipherSuites: util.DiagList[util.DiagList[TEEPCipherSuite]]{
			{
				{
					Type:      cose.CBORTagSign1Message,
					Algorithm: int(cose.AlgorithmESP256),
				},
			},
			{
				{
					Type:      cose.CBORTagSign1Message,
					Algorithm: int(cose.AlgorithmEd25519EdDSA),
				},
			},
		},
		SupportedSUITCOSEProfiles: []suit.COSEProfile{
			{
				DigestAlg:      cose.AlgorithmSHA256,
				AuthAlg:        cose.AlgorithmESP256,
				KeyExchangeAlg: cose.Algorithm(-29),
				EncryptionAlg:  cose.Algorithm(-65534),
			},
			{
				DigestAlg:      cose.AlgorithmSHA256,
				AuthAlg:        cose.AlgorithmEd25519EdDSA,
				KeyExchangeAlg: cose.Algorithm(-29),
				EncryptionAlg:  cose.Algorithm(-65534),
			},
		},
	}
	expected := `/ TEEPMessage: / [
  / type: / 1 / query-request /,
  / options: / {
    / challenge / 2: h'000102030405060708090A0B0C0D0E0F',
    / versions / 3: [0]
  },
  / supported-teep-cipher-suites: / [[[18, -9]], [[18, -19]]],
  / supported-suit-cose-profiles: / [[-16, -9, -29, -65534], [-16, -19, -29, -65534]],
  / data-item-requested: / 0 / none /
]`
	assert.Equal(t, expected, m1.String())

	d1 := RequestDataItem(true, true, true, true)
	assert.Equal(t, "15 / attestation | tc-list | extensions | suit-reports /", d1.CBORDiagString(0))

	errMsg := util.DiagString("manifest processing failed")
	m2 := TEEPMessage{
		Type: TEEPTypeError,
		Options: TEEPOptions{
			ErrMsg: &errMsg,
			SUITReports: util.DiagList[util.BytesHexMax32]{[]byte{
				0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
				0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F,
				0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2A, 0x2B, 0x2C, 0x2D, 0x2E, 0x2F,
			}},
		},
		ErrCode: TEEPErrManifestProcessingFailed,
	}
	expected = `/ TEEPMessage: / [
  / type: / 5 / error /,
  / options: / {
    / err-msg / 11: "manifest processing failed",
    / suit-reports / 18: [h'000102030405060708090A0B0C0D0E0F101112131415161718191A1B1C1D1E1F'/.../]
  },
  / err-code: / 11 / ERR_MANIFEST_PROCESSING_FAILED /
]`
	assert.Equal(t, expected, m2.String())

	sec := uint64(2)
	tr := true
	m3 := TEEPMessage{
		Type: TEEPTypeQueryResponse,
		Options: TEEPOptions{
			TCList: util.DiagList[suit.SystemPropertyClaims]{
				{
					SystemComponentID: suit.ComponentID{
						{0x68, 0x65, 0x6C, 0x6C, 0x6F, 0x2E, 0x74, 0x78, 0x74}, // hello.txt
					},
					ImageDigest: []byte{
						0x82, 0x2F, // [-16,
						0x58, 0x20, // h'
						0x21, 0xD2, 0xE4, 0xB1, 0x9E, 0x44, 0xC1, 0x90,
						0x6E, 0x58, 0x05, 0x05, 0x77, 0x92, 0x2D, 0x60,
						0x4C, 0x20, 0x4C, 0xCC, 0x2F, 0xD2, 0xCB, 0x97,
						0x7F, 0xEE, 0x8E, 0xD5, 0x6B, 0x62, 0xFD, 0x36,
					},
					ImageSize: 65132,
				},
			},
			RequestedTCList: util.DiagList[RequestedTCInfo]{
				{
					ComponentID: suit.ComponentID{
						{0x68, 0x65, 0x6C, 0x6C, 0x6F, 0x2E, 0x74, 0x78, 0x74}, // hello.txt
					},
					TCManifestSequenceNumber: &sec,
					HaveBinary:               &tr,
				},
			},
			Token: util.BytesHexMax32{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77},
		},
	}
	expected = `/ TEEPMessage: / [
  / type: / 2 / query-response /,
  / options: / {
    / tc-list / 7: [{
      / system-component-id / 0: ['hello.txt'],
      / image-digest / 3: << [-16, h'21D2E4B19E44C1906E58050577922D604C204CCC2FD2CB977FEE8ED56B62FD36'] >>,
      / image-size / 14: 65132
    }],
    / requested-tc-list / 13: [{
      / component-id / 15: ['hello.txt'],
      / tc-manifest-sequence-number / 16: 2,
      / have-binary / 17: true
    }],
    / token / 19: h'0011223344556677'
  }
]`
	assert.Equal(t, expected, m3.String())
	r := RequestedTCInfo{
		ComponentID: suit.ComponentID{
			{0x68, 0x65, 0x6C, 0x6C, 0x6F, 0x2E, 0x74, 0x78, 0x74}, // hello.txt
		},
	}
	assert.Equal(t, `{
  / component-id / 15: ['hello.txt']
}`, r.CBORDiagString(0))

	m4 := TEEPMessage{
		Type: TEEPTypeSuccess,
		Options: TEEPOptions{
			Token: util.BytesHexMax32{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77},
		},
	}
	expected = `/ TEEPMessage: / [
  / type: / 4 / success /,
  / options: / {
    / token / 19: h'0011223344556677'
  }
]`
	assert.Equal(t, expected, m4.String())

	m5 := TEEPMessage{
		Type: TEEPTypeUpdate,
		Options: TEEPOptions{
			ManifestList: util.DiagList[util.BytesHexMax32]{
				[]byte{
					0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
					0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F,
					0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2A, 0x2B, 0x2C, 0x2D, 0x2E, 0x2F,
				},
				[]byte{
					0x00,
				},
			},
			Token: util.BytesHexMax32{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77},
		},
	}
	expected = `/ TEEPMessage: / [
  / type: / 3 / update /,
  / options: / {
    / manifest-list / 9: [h'000102030405060708090A0B0C0D0E0F101112131415161718191A1B1C1D1E1F'/.../, h'00'],
    / token / 19: h'0011223344556677'
  }
]`
	assert.Equal(t, expected, m5.String())
}

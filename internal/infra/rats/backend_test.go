package rats

import "testing"

func TestVerifierBackendForAttestationPayloadFormat(t *testing.T) {
	tests := []struct {
		name   string
		format string
		want   string
	}{
		{
			name:   "sgx quote bundle uses intel qvl",
			format: AttestationPayloadFormatSGXQuote3TEEP,
			want:   VerifierBackendIntelQVL,
		},
		{
			name:   "sgx quote bundle matching is case insensitive",
			format: "Application/SGX-Quote3-Teep-Bundle",
			want:   VerifierBackendIntelQVL,
		},
		{
			name:   "other format uses veraison",
			format: `application/eat+cwt; eat_profile="urn:ietf:rfc:rfc9711"`,
			want:   VerifierBackendVeraison,
		},
		{
			name:   "empty format uses veraison",
			format: "",
			want:   VerifierBackendVeraison,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := VerifierBackendForAttestationPayloadFormat(tt.format); got != tt.want {
				t.Fatalf("VerifierBackendForAttestationPayloadFormat(%q) = %q, want %q", tt.format, got, tt.want)
			}
		})
	}
}

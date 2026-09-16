package domain

import "testing"

func TestNormalizeDKIMMode(t *testing.T) {
	tests := []struct {
		name    string
		in      DKIMMode
		want    DKIMMode
		wantErr bool
	}{
		{"empty defaults to rsa", "", DefaultDKIMMode, false},
		{"whitespace+case normalized", "  RSA  ", DKIMModeRSA, false},
		{"ed25519 accepted", "ed25519", DKIMModeEd25519, false},
		{"dual accepted", "dual", DKIMModeDual, false},
		{"rsa passthrough", "rsa", DKIMModeRSA, false},
		{"unknown rejected", "dsa", "", true},
		{"junk rejected", "not-a-mode", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeDKIMMode(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NormalizeDKIMMode(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("NormalizeDKIMMode(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestValidDKIMMode(t *testing.T) {
	for _, m := range []DKIMMode{DKIMModeRSA, DKIMModeEd25519, DKIMModeDual} {
		if !ValidDKIMMode(m) {
			t.Errorf("ValidDKIMMode(%q) = false, want true", m)
		}
	}
	for _, m := range []DKIMMode{"", "RSA2", "auto"} {
		if ValidDKIMMode(m) {
			t.Errorf("ValidDKIMMode(%q) = true, want false", m)
		}
	}
}

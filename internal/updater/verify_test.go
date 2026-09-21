package updater

import "testing"

// sigHex is a signature over "nvy-signing-selftest-v1" produced with the
// private key matching the embedded PublicKey. It is a stable test vector:
// if PublicKey ever drifts from the real key, this test fails.
const sigHex = "18277931da94c699065276ed18d6fa8b3929412e8d3798d2f053b7681fd28bb3d4addcfd7a86a135e093e8168d4ea1b1fe8a9bac47e378762a09e33a3a345b09"

func TestVerifyChecksums(t *testing.T) {
	if err := VerifyChecksums([]byte("nvy-signing-selftest-v1"), sigHex); err != nil {
		t.Fatalf("VerifyChecksums() = %v, want nil", err)
	}
}

func TestVerifyChecksumsTampered(t *testing.T) {
	if err := VerifyChecksums([]byte("nvy-signing-selftest-v2"), sigHex); err == nil {
		t.Fatal("VerifyChecksums() = nil, want error for tampered message")
	}
}

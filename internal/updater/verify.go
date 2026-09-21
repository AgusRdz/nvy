// Package updater provides signing verification primitives used by nvy's
// self-update flow.
package updater

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
)

// PublicKey is the hex-encoded Ed25519 public key used to verify the
// signature over release checksums.
const PublicKey = "8ac9dcb686a133e4828e6f47b399e6e7fdb1f232315e18aeded2e78ae4bb5257"

// VerifyChecksums verifies the Ed25519 signature (hex-encoded) over the raw
// checksums bytes using the embedded PublicKey.
func VerifyChecksums(checksums []byte, sigHex string) error {
	pub, err := hex.DecodeString(PublicKey)
	if err != nil {
		return fmt.Errorf("nvy: invalid public key: %w", err)
	}

	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return fmt.Errorf("nvy: invalid signature format: %w", err)
	}

	if !ed25519.Verify(pub, checksums, sig) {
		return fmt.Errorf("nvy: signature verification failed")
	}

	return nil
}

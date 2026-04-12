// Package signer wraps cosign v2's SignCmd to sign OCI images programmatically,
// exactly replicating `cosign sign --key cosign.key <image>` CLI behaviour.
package signer

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/sign"
)

// Config holds all signing configuration.
type Config struct {
	// KeyPath is the path to the cosign private key file.
	// Supports ENCRYPTED SIGSTORE PRIVATE KEY (cosign generate-key-pair output).
	KeyPath string

	// KeyPassword is the passphrase for the private key.
	// Equivalent to COSIGN_PASSWORD env var.
	KeyPassword string

	// RegistryUsername / RegistryPassword authenticate against Harbor when
	// pushing the .sig tag back to the registry.
	RegistryUsername string
	RegistryPassword string

	// RegistryInsecure allows plain HTTP (not recommended in production).
	RegistryInsecure bool

	// TlogDisabled disables Rekor transparency log upload.
	// Set true for air-gapped / offline environments.
	TlogDisabled bool

	// AllowedSeverities lists the Harbor severity labels that permit signing.
	// Any severity NOT in this list (e.g. "Critical") is blocked.
	AllowedSeverities []string

	// SkipOperators lists Harbor operator names (robot accounts / users) whose
	// events should be ignored to break the push→scan→sign→push loop.
	// Add your Harbor robot account name here.
	SkipOperators []string
}

// Signer signs OCI images using cosign v2 as a library.
type Signer struct {
	cfg Config
}

// New creates a Signer.
func New(cfg Config) *Signer {
	return &Signer{cfg: cfg}
}

// AllowedSeverities returns the configured allow-list.
func (s *Signer) AllowedSeverities() []string { return s.cfg.AllowedSeverities }

// IsSeverityAllowed returns true when the severity is in the allow-list.
func (s *Signer) IsSeverityAllowed(severity string) bool {
	for _, a := range s.cfg.AllowedSeverities {
		if strings.EqualFold(a, severity) {
			return true
		}
	}
	return false
}

// IsOperatorSkipped returns true when the operator that triggered the event
// matches one of the configured skip operators (i.e. our own robot account).
// This is the primary loop-prevention guard: when we push the signature tag
// Harbor may re-scan it, firing another SCANNING_COMPLETED event with our
// robot account as operator — which we then silently drop.
func (s *Signer) IsOperatorSkipped(operator string) bool {
	for _, skip := range s.cfg.SkipOperators {
		if strings.EqualFold(skip, operator) {
			return true
		}
	}
	return false
}

// SignImage signs the given OCI image reference (must be digest-pinned:
// registry/repo@sha256:...) using cosign v2's SignCmd — identical to running:
//
//	cosign sign --key <KeyPath> --tlog-upload=<true|false> <imageRef>
//
// Cosign handles:
//   - Loading and decrypting the ENCRYPTED SIGSTORE PRIVATE KEY
//   - Computing and signing the image digest payload
//   - Uploading to Rekor (unless TlogDisabled)
//   - Pushing the .sig tag back to the registry (e.g. sha256-abc123.sig)
func (s *Signer) SignImage(ctx context.Context, imageRef string) error {
	slog.Info("signing image with cosign", "ref", imageRef)

	// ── KeyOpts: equivalent to `cosign sign --key <path>` ────────────────────
	ko := options.KeyOpts{
		KeyRef:   s.cfg.KeyPath,
		PassFunc: staticPassFunc(s.cfg.KeyPassword),
	}

	// ── RegistryOptions: auth for pushing the .sig tag ────────────────────────
	regOpts := options.RegistryOptions{
		AuthConfig: authn.AuthConfig{
			Username: s.cfg.RegistryUsername,
			Password: s.cfg.RegistryPassword,
		},
		AllowInsecure:     s.cfg.RegistryInsecure,
		AllowHTTPRegistry: s.cfg.RegistryInsecure,
	}

	// ── SignOptions: mirrors cosign CLI defaults ───────────────────────────────
	signOpts := options.SignOptions{
		Upload:           true,
		TlogUpload:       !s.cfg.TlogDisabled,
		SkipConfirmation: true, // no interactive prompt
		Registry:         regOpts,
	}

	// ── RootOptions: standard defaults ────────────────────────────────────────
	ro := &options.RootOptions{
		Timeout: options.DefaultTimeout,
	}

	// sign.SignCmd is the exact function the `cosign sign` CLI calls.
	// It signs the image, uploads to Rekor if configured, and pushes the
	// signature to the registry as:  <repo>:sha256-<digest-hex>.sig
	if err := sign.SignCmd(ro, ko, signOpts, []string{imageRef}); err != nil {
		return fmt.Errorf("cosign sign %q: %w", imageRef, err)
	}

	slog.Info("image signed and pushed to registry",
		"ref", imageRef,
		"tlog_upload", !s.cfg.TlogDisabled,
	)
	return nil
}

// staticPassFunc returns a cosign PassFunc that always returns the given
// password — equivalent to setting COSIGN_PASSWORD env var.
func staticPassFunc(password string) func(bool) ([]byte, error) {
	return func(bool) ([]byte, error) {
		return []byte(password), nil
	}
}

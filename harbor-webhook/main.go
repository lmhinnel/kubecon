// Command harbor-webhook-signer receives Harbor SCANNING_COMPLETED webhook
// events and signs non-Critical images using cosign v2 as a library —
// identical to running `cosign sign --key cosign.key <image>` from the CLI.
//
// Loop prevention: events whose operator matches SKIP_OPERATORS are silently
// dropped, breaking the push→scan→sign→push cycle.
package main

import (
	"log/slog"
	"os"
	"strings"

	"github.com/lmhinnel/kubecon/harbor-webhook/server"
	"github.com/lmhinnel/kubecon/harbor-webhook/signer"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel(),
	})))

	signerCfg := signer.Config{
		// Cosign key — supports ENCRYPTED SIGSTORE PRIVATE KEY format
		KeyPath:     mustEnv("COSIGN_KEY_PATH", "/keys/cosign.key"),
		KeyPassword: getEnv("COSIGN_PASSWORD", ""),

		// Registry credentials for pushing .sig tags back to Harbor
		RegistryUsername: getEnv("REGISTRY_USERNAME", ""),
		RegistryPassword: getEnv("REGISTRY_PASSWORD", ""),
		RegistryInsecure: getEnv("REGISTRY_INSECURE", "") == "true",

		// Set TLOG_DISABLED=true for air-gapped environments
		TlogDisabled: getEnv("TLOG_DISABLED", "") == "true",

		// Severities that allow signing; Critical is excluded by default
		AllowedSeverities: splitTrim(
			getEnv("ALLOWED_MAX_SEVERITY", "High,Medium,Low,Unknown"), ",",
		),

		// IMPORTANT: add your Harbor robot account name(s) here.
		// Events whose operator matches are dropped to prevent the
		// push→re-scan→sign→push infinite loop.
		SkipOperators: splitTrim(getEnv("SKIP_OPERATORS", ""), ","),
	}

	s := signer.New(signerCfg)

	slog.Info("signer ready",
		"key_path", signerCfg.KeyPath,
		"tlog_disabled", signerCfg.TlogDisabled,
		"allowed_severities", signerCfg.AllowedSeverities,
		"skip_operators", signerCfg.SkipOperators,
	)

	srv := server.New(server.Config{
		Port:          getEnv("PORT", "8080"),
		WebhookSecret: getEnv("WEBHOOK_SECRET", ""),
	}, s)

	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Warn("env var not set, using default", "key", key, "default", fallback)
		return fallback
	}
	return v
}

func splitTrim(s, sep string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func logLevel() slog.Level {
	switch strings.ToUpper(getEnv("LOG_LEVEL", "INFO")) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

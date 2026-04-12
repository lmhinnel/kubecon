# Harbor Webhook → Sigstore Image Signer

Receives Harbor `SCANNING_COMPLETED` webhook events and automatically
**signs images with sigstore-go** when the scan severity is **not Critical**.
No `cosign` CLI binary required — signing is done entirely in-process via the
[sigstore-go](https://github.com/sigstore/sigstore-go) library.

---

## Signing Decision

| Harbor severity               | Action                                |
| ----------------------------- | ------------------------------------- |
| Unknown / Low / Medium / High | ✅ Signed via `sign.Bundle()`         |
| **Critical**                  | ❌ **Blocked — image stays unsigned** |

The allow-list is controlled by the `ALLOWED_MAX_SEVERITY` env var.

---

## Architecture

```
main.go
  ├── internal/harbor/   — Webhook payload types (Webhook, Resource, ScanOverview …)
  ├── internal/signer/   — sigstore-go sign.Bundle wrapper + ECDSAKeypair
  └── internal/server/   — HTTP server, token auth, async event dispatch
```

### Before vs After

| Before                                  | After                                 |
| --------------------------------------- | ------------------------------------- |
| `exec.Command("cosign", "sign", ...)`   | `sign.Bundle(content, keypair, opts)` |
| cosign binary in Docker image (~100 MB) | Pure Go — distroless, no binary       |
| `COSIGN_KEY_REF` (k8s:// or file)       | `COSIGN_KEY_PATH` (PEM file mount)    |
| Monolithic `main.go`                    | 4 focused packages                    |
| `log.Printf`                            | `log/slog` — structured JSON          |

---

## sigstore-go API Used

```go
// 1. Wrap the image reference as the artifact to sign
content := &sign.PlainData{Data: []byte(imageRef)}

// 2. Load your ECDSA P-256 private key (implements sign.Keypair)
keypair, _ := signer.LoadECDSAKeypair("/keys/cosign.key", password)

// 3. Configure transparency log and/or Fulcio CA
opts := sign.BundleOptions{
    Context: ctx,
    TransparencyLogs: []sign.Transparency{
        sign.NewRekor(&sign.RekorOptions{URL: "https://rekor.sigstore.dev"}),
    },
    // Optional Fulcio OIDC cert:
    // CertificateProvider: sign.NewFulcio(&sign.FulcioOptions{BaseURL: fulcioURL}),
    // CertificateProviderOptions: &sign.CertificateProviderOptions{IDToken: oidcToken},
}

// 4. Create the Sigstore protobuf Bundle
bundle, err := sign.Bundle(content, keypair, opts)
// bundle contains: signature + public key hint + Rekor log entry
```

The `ECDSAKeypair` in `internal/signer/keypair.go` implements the
`sign.Keypair` interface. To use a KMS or HSM instead, implement the
same interface:

```go
type Keypair interface {
    GetHashAlgorithm() protocommon.HashAlgorithm
    GetHint()          []byte
    GetKeyAlgorithm()  string
    GetPublicKeyPem()  (string, error)
    SignData(data []byte) (signature, digest []byte, err error)
}
```

---

## Quick Start

### 1. Generate a key pair

```bash
# Using cosign
cosign generate-key-pair
# → cosign.key  cosign.pub

# Or using openssl
openssl ecparam -name prime256v1 -genkey -noout | \
  openssl pkcs8 -topk8 -nocrypt -out cosign.key
openssl ec -in cosign.key -pubout -out cosign.pub
```

### 2. Create Kubernetes secrets

```bash
kubectl create namespace harbor-webhook

# Private key + password
kubectl create secret generic cosign-key \
  --from-file=cosign.key=./cosign.key \
  --from-literal=cosign.password='<your-password>' \
  -n harbor-webhook

# Shared token to authenticate Harbor webhook calls
kubectl create secret generic webhook-secret \
  --from-literal=token='<your-harbor-webhook-token>' \
  -n harbor-webhook
```

### 3. Build and push the image

```bash
IMAGE=your-registry/harbor-webhook-signer:latest

# go.sum is completed automatically during Docker build
docker build -t $IMAGE .
docker push $IMAGE
```

Edit `image:` in `k8s/03-deployment.yaml` to match `$IMAGE`.

### 4. Deploy

```bash
kubectl apply -f k8s/
```

### 5. Configure Harbor webhook

Harbor → Project → Webhooks → **New Webhook**:

| Field        | Value                                            |
| ------------ | ------------------------------------------------ |
| Notify Type  | HTTP                                             |
| Endpoint URL | `https://webhook-signer.your-domain.com/webhook` |
| Auth Header  | `X-Harbor-Webhook-Token: <your-token>`           |
| Events       | ✅ Scanning Finished                             |

---

## Environment Variables

| Variable               | Default                      | Description                               |
| ---------------------- | ---------------------------- | ----------------------------------------- |
| `PORT`                 | `8080`                       | HTTP listen port                          |
| `WEBHOOK_SECRET`       | _(empty)_                    | Validates `X-Harbor-Webhook-Token` header |
| `COSIGN_KEY_PATH`      | `/keys/cosign.key`           | Path to PEM private key                   |
| `COSIGN_PASSWORD`      | _(empty)_                    | Decrypts the private key if encrypted     |
| `REKOR_URL`            | `https://rekor.sigstore.dev` | Rekor endpoint; set to `""` to disable    |
| `FULCIO_URL`           | _(empty)_                    | Fulcio CA; requires `OIDC_TOKEN`          |
| `OIDC_TOKEN`           | _(empty)_                    | OIDC ID token for Fulcio cert signing     |
| `ALLOWED_MAX_SEVERITY` | `High,Medium,Low,Unknown`    | Severities that permit signing            |
| `LOG_LEVEL`            | `INFO`                       | `DEBUG` / `INFO` / `WARN` / `ERROR`       |

---

## Endpoints

| Path       | Method | Description                |
| ---------- | ------ | -------------------------- |
| `/webhook` | `POST` | Harbor event receiver      |
| `/healthz` | `GET`  | Liveness / readiness probe |

---

## Project Layout

```
.
├── main.go                        # Entry point: wires signer + HTTP server
├── go.mod                         # sigstore-go v0.5.1
├── go.sum
├── Dockerfile                     # 3-stage: deps → builder → distroless:nonroot
├── internal/
│   ├── harbor/
│   │   └── types.go               # Harbor webhook JSON types
│   ├── signer/
│   │   ├── keypair.go             # ECDSAKeypair — implements sign.Keypair
│   │   ├── signer.go              # Signer: sign.Bundle wrapper + severity guard
│   │   └── accessor.go            # Package doc anchor
│   └── server/
│       └── server.go              # HTTP handler, token auth, async dispatch
└── k8s/
    ├── 00-namespace.yaml
    ├── 01-secrets.yaml            # cosign key, webhook token
    ├── 02-configmap.yaml          # non-sensitive config
    ├── 03-deployment.yaml         # 2 replicas, distroless, read-only FS
    ├── 04-service-ingress.yaml    # ClusterIP + nginx Ingress on /webhook
    └── 05-rbac.yaml               # ServiceAccount + Role
```

---

## Security Notes

- Container runs as **non-root** (uid 65532, `distroless:nonroot`)
- Root filesystem is **read-only** in the Deployment
- Private key is mounted at `/keys/cosign.key` with mode `0400`
- Requests without a matching `X-Harbor-Webhook-Token` return `401`
- Images are always signed by **digest** (`image@sha256:…`) for immutability
- Signing is **asynchronous** — Harbor gets `202 Accepted` immediately

---

## Deploying to an Air-Gapped Environment

Set `REKOR_URL=""` in the ConfigMap to disable transparency log submission.
Set `FULCIO_URL=""` to use key-only signing (no OIDC).
The server will still produce a valid Sigstore bundle with just the key signature.

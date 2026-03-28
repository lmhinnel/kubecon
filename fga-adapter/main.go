package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	openfga "github.com/openfga/go-sdk/client"
)

// MinioRequest mirrors the OPA-style input that MinIO sends to an external policy engine.
// MinIO encodes the request as { "input": { ... } }.
type MinioRequest struct {
	Input struct {
		Account     string              `json:"account"`     // IAM account / username
		Action      string              `json:"action"`      // e.g. "s3:GetObject"
		Bucket      string              `json:"bucket"`      // bucket name (may be empty for admin actions)
		Object      string              `json:"object"`      // object key  (may be empty)
		Conditions  map[string][]string `json:"conditions"`  // optional: IP, referer, …
		Owner       bool                `json:"owner"`       // true if requester is the bucket owner
		IsOwnerFunc bool                `json:"isOwnerFunc"` // MinIO internal flag
	} `json:"input"`
}

// MinioResponse is what MinIO expects back: { "result": true|false }
type MinioResponse struct {
	Result bool `json:"result"`
}

// actionPermission maps an S3/admin action to the OpenFGA relation and the kind of
// object the relation is checked against ("system", "bucket", "object").
type actionPermission struct {
	relation   string
	objectKind string // "system" | "bucket" | "object"
}

// actionMap is the exhaustive mapping of every MinIO / S3 action.
// "can_read"   – read-only, no mutations
// "can_write"  – create / upload / restore
// "can_delete" – remove resources
// "admin"      – server-level administration
// "owner"      – full control over a specific bucket (ACL, policy, versioning …)
var actionMap = map[string]actionPermission{
	// ── S3 Bucket-level ────────────────────────────────────────────────────────
	"s3:CreateBucket":                     {"can_write", "bucket"},
	"s3:DeleteBucket":                     {"can_delete", "bucket"},
	"s3:ForceDeleteBucket":                {"can_delete", "bucket"},
	"s3:GetBucketLocation":                {"can_read", "bucket"},
	"s3:GetBucketNotification":            {"can_read", "bucket"},
	"s3:GetBucketObjectLockConfiguration": {"can_read", "bucket"},
	"s3:GetBucketPolicy":                  {"owner", "bucket"},
	"s3:GetBucketPolicyStatus":            {"owner", "bucket"},
	"s3:GetBucketRequestPayment":          {"can_read", "bucket"},
	"s3:GetBucketTagging":                 {"can_read", "bucket"},
	"s3:GetBucketVersioning":              {"can_read", "bucket"},
	"s3:GetBucketWebsite":                 {"can_read", "bucket"},
	"s3:GetEncryptionConfiguration":       {"owner", "bucket"},
	"s3:GetLifecycleConfiguration":        {"can_read", "bucket"},
	"s3:GetReplicationConfiguration":      {"owner", "bucket"},
	"s3:ListAllMyBuckets":                 {"can_read", "system"},
	"s3:ListBucket":                       {"can_read", "bucket"},
	"s3:ListBucketMultipartUploads":       {"can_read", "bucket"},
	"s3:ListBucketVersions":               {"can_read", "bucket"},
	"s3:PutBucketNotification":            {"owner", "bucket"},
	"s3:PutBucketObjectLockConfiguration": {"owner", "bucket"},
	"s3:PutBucketPolicy":                  {"owner", "bucket"},
	"s3:PutBucketRequestPayment":          {"owner", "bucket"},
	"s3:PutBucketTagging":                 {"owner", "bucket"},
	"s3:PutBucketVersioning":              {"owner", "bucket"},
	"s3:PutBucketWebsite":                 {"owner", "bucket"},
	"s3:PutEncryptionConfiguration":       {"owner", "bucket"},
	"s3:PutLifecycleConfiguration":        {"owner", "bucket"},
	"s3:PutReplicationConfiguration":      {"owner", "bucket"},
	"s3:DeleteBucketPolicy":               {"owner", "bucket"},
	"s3:DeleteBucketWebsite":              {"owner", "bucket"},
	"s3:DeleteBucketTagging":              {"owner", "bucket"},
	"s3:DeleteReplicationConfiguration":   {"owner", "bucket"},

	// ── S3 Object-level ────────────────────────────────────────────────────────
	"s3:GetObject":                      {"can_read", "object"},
	"s3:GetObjectAttributes":            {"can_read", "object"},
	"s3:GetObjectTagging":               {"can_read", "object"},
	"s3:GetObjectVersionTagging":        {"can_read", "object"},
	"s3:GetObjectTorrent":               {"can_read", "object"},
	"s3:GetObjectAcl":                   {"owner", "object"},
	"s3:GetObjectLegalHold":             {"can_read", "object"},
	"s3:GetObjectRetention":             {"can_read", "object"},
	"s3:GetObjectVersion":               {"can_read", "object"},
	"s3:GetObjectVersionAcl":            {"owner", "object"},
	"s3:GetObjectVersionForReplication": {"can_read", "object"},
	"s3:GetObjectVersionTorrent":        {"can_read", "object"},
	"s3:ListMultipartUploadParts":       {"can_read", "object"},
	"s3:PutObject":                      {"can_write", "object"},
	"s3:PutObjectTagging":               {"can_write", "object"},
	"s3:PutObjectVersionTagging":        {"can_write", "object"},
	"s3:PutObjectAcl":                   {"owner", "object"},
	"s3:PutObjectLegalHold":             {"owner", "object"},
	"s3:PutObjectRetention":             {"owner", "object"},
	"s3:CopyObject":                     {"can_write", "object"},
	"s3:DeleteObject":                   {"can_delete", "object"},
	"s3:DeleteObjectTagging":            {"can_write", "object"},
	"s3:DeleteObjectVersion":            {"can_delete", "object"},
	"s3:DeleteObjectVersionTagging":     {"can_write", "object"},
	"s3:RestoreObject":                  {"can_write", "object"},
	"s3:AbortMultipartUpload":           {"can_write", "object"},
	"s3:ReplicateDelete":                {"can_delete", "object"},
	"s3:ReplicateObject":                {"can_write", "object"},
	"s3:ReplicateTags":                  {"can_write", "object"},

	// ── MinIO Admin ────────────────────────────────────────────────────────────
	"admin:CreatePolicy":            {"admin", "system"},
	"admin:DeletePolicy":            {"admin", "system"},
	"admin:GetPolicy":               {"admin", "system"},
	"admin:AttachUserOrGroupPolicy": {"admin", "system"},
	"admin:CreateUser":              {"admin", "system"},
	"admin:DeleteUser":              {"admin", "system"},
	"admin:GetUser":                 {"admin", "system"},
	"admin:ListUsers":               {"admin", "system"},
	"admin:EnableUser":              {"admin", "system"},
	"admin:DisableUser":             {"admin", "system"},
	"admin:CreateGroup":             {"admin", "system"},
	"admin:RemoveGroup":             {"admin", "system"},
	"admin:AddUserToGroup":          {"admin", "system"},
	"admin:RemoveUserFromGroup":     {"admin", "system"},
	"admin:GetGroup":                {"admin", "system"},
	"admin:ListGroups":              {"admin", "system"},
	"admin:EnableGroup":             {"admin", "system"},
	"admin:DisableGroup":            {"admin", "system"},
	"admin:CreateServiceAccount":    {"admin", "system"},
	"admin:UpdateServiceAccount":    {"admin", "system"},
	"admin:RemoveServiceAccount":    {"admin", "system"},
	"admin:ListServiceAccounts":     {"admin", "system"},
	"admin:SetBucketQuota":          {"admin", "system"},
	"admin:GetBucketQuota":          {"admin", "system"},
	"admin:SetBucketTarget":         {"admin", "system"},
	"admin:GetBucketTarget":         {"admin", "system"},
	"admin:ReplicationDiff":         {"admin", "system"},
	"admin:Heal":                    {"admin", "system"},
	"admin:ServerInfo":              {"admin", "system"},
	"admin:ServerUpdate":            {"admin", "system"},
	"admin:StorageInfo":             {"admin", "system"},
	"admin:DataUsageInfo":           {"admin", "system"},
	"admin:TopLocksInfo":            {"admin", "system"},
	"admin:Profiling":               {"admin", "system"},
	"admin:Prometheus":              {"admin", "system"},
	"admin:Trace":                   {"admin", "system"},
	"admin:ConsoleLog":              {"admin", "system"},
	"admin:KMSCreateKey":            {"admin", "system"},
	"admin:KMSKeyStatus":            {"admin", "system"},
	"admin:OBDInfo":                 {"admin", "system"},
	"admin:SetTier":                 {"admin", "system"},
	"admin:ListTier":                {"admin", "system"},
}

var fgaClient *openfga.OpenFgaClient

// buildObject returns the OpenFGA object identifier for the request.
// objectKind is one of "system", "bucket", "object".
func buildObject(kind, bucket, object string) (string, bool) {
	switch kind {
	case "system":
		return "system:minio", true
	case "bucket":
		if bucket == "" {
			return "", false
		}
		return fmt.Sprintf("bucket:%s", bucket), true
	case "object":
		if bucket == "" {
			return "", false
		}
		// When the object key is empty we fall back to checking bucket-level access.
		if object == "" {
			return fmt.Sprintf("bucket:%s", bucket), true
		}
		// Normalise leading slash that MinIO sometimes includes.
		key := strings.TrimPrefix(object, "/")
		return fmt.Sprintf("object:%s/%s", bucket, key), true
	}
	return "", false
}

// checkPermission performs the OpenFGA check and returns whether the action is allowed.
func checkPermission(ctx context.Context, req MinioRequest) bool {
	account := strings.TrimSpace(req.Input.Account)
	action := strings.TrimSpace(req.Input.Action)

	// ── Sanity guards ──────────────────────────────────────────────────────────
	if account == "" || action == "" {
		log.Printf("[DENY] Empty account or action")
		return false
	}

	// The MinIO root user (usually "minio") always gets through – avoids
	// bootstrapping issues.  Set MINIO_ROOT_USER to override.
	rootUser := os.Getenv("MINIO_ROOT_USER")
	if rootUser == "" {
		rootUser = "admin"
	}
	if account == rootUser {
		log.Printf("[ALLOW] Root user bypass for %s", account)
		return true
	}

	// ── Look up action ─────────────────────────────────────────────────────────
	perm, known := actionMap[action]
	if !known {
		// Unknown actions: try to infer from prefix rather than hard-deny,
		// so that future MinIO releases don't lock everyone out.
		switch {
		case strings.HasPrefix(action, "admin:"):
			perm = actionPermission{"admin", "system"}
		case strings.Contains(action, "Delete") || strings.Contains(action, "Remove"):
			perm = actionPermission{"can_delete", "bucket"}
		case strings.Contains(action, "Put") || strings.Contains(action, "Create") ||
			strings.Contains(action, "Restore") || strings.Contains(action, "Copy"):
			perm = actionPermission{"can_write", "bucket"}
		default:
			perm = actionPermission{"can_read", "bucket"}
		}
		log.Printf("[WARN] Unknown action %q – inferred relation=%s kind=%s", action, perm.relation, perm.objectKind)
	}

	// ── Build the FGA object ───────────────────────────────────────────────────
	fgaObject, ok := buildObject(perm.objectKind, req.Input.Bucket, req.Input.Object)
	if !ok {
		log.Printf("[DENY] Cannot build FGA object for action=%s bucket=%q object=%q",
			action, req.Input.Bucket, req.Input.Object)
		return false
	}

	user := fmt.Sprintf("user:%s", account)

	log.Printf("[CHECK] user=%s relation=%s object=%s (action=%s)",
		user, perm.relation, fgaObject, action)

	// ── Call OpenFGA ───────────────────────────────────────────────────────────
	body := openfga.ClientCheckRequest{
		User:     user,
		Relation: perm.relation,
		Object:   fgaObject,
	}

	resp, err := fgaClient.Check(ctx).Body(body).Execute()
	if err != nil {
		log.Printf("[ERROR] FGA check failed: %v", err)
		return false
	}

	allowed := resp.Allowed != nil && *resp.Allowed

	// ── For object-level actions, also accept bucket-level "owner" permission ──
	// This lets a bucket owner operate on any object without per-object tuples.
	if !allowed && perm.objectKind == "object" && perm.relation != "owner" {
		bucketObject := fmt.Sprintf("bucket:%s", req.Input.Bucket)
		ownerBody := openfga.ClientCheckRequest{
			User:     user,
			Relation: "owner",
			Object:   bucketObject,
		}
		ownerResp, err2 := fgaClient.Check(ctx).Body(ownerBody).Execute()
		if err2 == nil && ownerResp.Allowed != nil && *ownerResp.Allowed {
			log.Printf("[ALLOW] Bucket owner fallback: user=%s bucket=%s", user, req.Input.Bucket)
			return true
		}
	}

	log.Printf("[RESULT] allowed=%v user=%s relation=%s object=%s",
		allowed, user, perm.relation, fgaObject)
	return allowed
}

// healthHandler returns 200 OK so orchestrators know the service is alive.
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, `{"status":"ok"}`)
}

// validateHandler is the main OPA-compatible endpoint consumed by MinIO.
func validateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req MinioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		bodyBytes, _ := io.ReadAll(r.Body)
		log.Printf("[ERROR] Bad request body: %v. Body: %s", err, string(bodyBytes))
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if os.Getenv("DEBUG") == "true" {
		// print the full request for debugging
		reqBytes, _ := json.MarshalIndent(req, "", "  ")
		log.Printf("[DEBUG] Received request: %s", string(reqBytes))

	}
	req.Input.Account = strings.TrimSpace(req.Input.Conditions["username"][0])

	allowed := checkPermission(r.Context(), req)

	log.Printf("[DECISION] allowed=%v account=%s action=%s bucket=%s object=%s",
		allowed, req.Input.Account, req.Input.Action, req.Input.Bucket, req.Input.Object)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(MinioResponse{Result: allowed}); err != nil {
		log.Printf("[ERROR] Failed to write response: %v", err)
	}
}

func main() {
	apiURL := os.Getenv("FGA_API_URL")
	storeID := os.Getenv("FGA_STORE_ID")
	modelID := os.Getenv("FGA_MODEL_ID")
	port := os.Getenv("PORT")

	if apiURL == "" || storeID == "" || modelID == "" {
		log.Fatal("FGA_API_URL, FGA_STORE_ID, and FGA_MODEL_ID must be set")
	}
	if port == "" {
		port = "8080"
	}

	var err error
	fgaClient, err = openfga.NewSdkClient(&openfga.ClientConfiguration{
		ApiUrl:               apiURL,
		StoreId:              storeID,
		AuthorizationModelId: modelID,
	})
	if err != nil {
		log.Fatalf("Cannot initialise FGA client: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/validate", validateHandler)
	mux.HandleFunc("/healthz", healthHandler)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	log.Printf("OpenFGA-MinIO adapter listening on :%s", port)
	log.Fatal(srv.ListenAndServe())
}

// loggingMiddleware logs every HTTP request with duration.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

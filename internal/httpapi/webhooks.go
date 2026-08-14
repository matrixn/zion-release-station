package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/matrixn/zion-release-station/internal/sites"
)

const maxWebhookPayload = 1 << 20
const publicWebhookPrefix = "/releasestation/api/v1/webhooks"

type webhookView struct {
	ID               string `json:"id,omitempty"`
	SiteID           string `json:"site_id"`
	Provider         string `json:"provider"`
	Enabled          bool   `json:"enabled"`
	Configured       bool   `json:"configured"`
	SecretConfigured bool   `json:"secret_configured"`
	Endpoint         string `json:"endpoint,omitempty"`
	LastDeliveryAt   string `json:"last_delivery_at,omitempty"`
	LastError        string `json:"last_error,omitempty"`
}

type webhookPushEvent struct {
	EventName  string
	Delivery   string
	Repository string
	Ref        string
	After      string
	Deleted    bool
}

func (s *Server) handleIncomingWebhook(provider string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is supported.")
		return
	}
	token := webhookTokenFromPath(r.URL.Path, provider)
	if token == "" {
		writeError(w, http.StatusNotFound, "WEBHOOK_NOT_FOUND", "Webhook endpoint not found.")
		return
	}
	config, err := s.findWebhook(r.Context(), provider, token)
	if errors.Is(err, errWebhookNotFound) {
		writeError(w, http.StatusNotFound, "WEBHOOK_NOT_FOUND", "Webhook endpoint not found.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "WEBHOOK_UNAVAILABLE", err.Error())
		return
	}
	secret, err := s.webhookSecret(config.encryptedSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "WEBHOOK_UNAVAILABLE", "Webhook secret cannot be read.")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookPayload))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "WEBHOOK_TOO_LARGE", "Webhook payload exceeds the 1 MB limit.")
		return
	}
	if !verifyWebhookSignature(provider, secret, r.Header, body) {
		_ = s.recordAudit(r.Context(), "webhook", "", "webhook.signature_rejected", "site", config.siteID, map[string]any{"provider": provider, "remote": r.RemoteAddr})
		writeError(w, http.StatusUnauthorized, "INVALID_WEBHOOK_SIGNATURE", "Webhook signature is invalid.")
		return
	}

	event, err := parseWebhookPush(provider, r.Header, body)
	if err != nil {
		_ = s.recordAudit(r.Context(), "webhook", "", "webhook.payload_rejected", "site", config.siteID, map[string]any{"provider": provider, "reason": err.Error()})
		writeError(w, http.StatusBadRequest, "INVALID_WEBHOOK_PAYLOAD", err.Error())
		return
	}
	if event.Delivery == "" {
		digest := sha256.Sum256(body)
		event.Delivery = hex.EncodeToString(digest[:])
	}
	payloadDigest := sha256.Sum256(body)
	duplicate, err := s.recordWebhookDelivery(r.Context(), config, event, hex.EncodeToString(payloadDigest[:]))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "WEBHOOK_UNAVAILABLE", err.Error())
		return
	}
	if duplicate {
		writeJSON(w, http.StatusAccepted, map[string]any{"data": map[string]any{"status": "already_processed", "delivery_id": event.Delivery}})
		return
	}

	site, err := s.sites.Get(r.Context(), config.siteID)
	if errors.Is(err, sites.ErrNotFound) {
		_ = s.markWebhookDeliveryError(r.Context(), config.id, event.Delivery, "site is no longer managed")
		writeJSON(w, http.StatusAccepted, map[string]any{"data": map[string]any{"status": "ignored", "reason": "site_not_managed"}})
		return
	}
	if err != nil {
		returnWebhookFailure(w, s.markWebhookDeliveryError(r.Context(), config.id, event.Delivery, "site lookup failed"), err)
		return
	}
	if event.EventName != "push" || event.Deleted || strings.TrimSpace(event.After) == "" || allZeroSHA(event.After) {
		_ = s.recordAudit(r.Context(), "webhook", event.Delivery, "webhook.ignored", "site", site.ID, map[string]any{"provider": provider, "event": event.EventName, "reason": "not_a_deployable_push"})
		writeJSON(w, http.StatusAccepted, map[string]any{"data": map[string]any{"status": "ignored", "reason": "not_a_deployable_push", "delivery_id": event.Delivery}})
		return
	}
	if !site.PushToDeploy || site.Repository == nil || strings.ToLower(strings.TrimSpace(site.Repository.Provider)) != provider {
		_ = s.recordAudit(r.Context(), "webhook", event.Delivery, "webhook.ignored", "site", site.ID, map[string]any{"provider": provider, "reason": "push_to_deploy_not_configured"})
		writeJSON(w, http.StatusAccepted, map[string]any{"data": map[string]any{"status": "ignored", "reason": "push_to_deploy_not_configured", "delivery_id": event.Delivery}})
		return
	}
	branch := strings.TrimPrefix(strings.TrimSpace(event.Ref), "refs/heads/")
	configuredBranch := strings.TrimSpace(site.Repository.Branch)
	if branch == "" || configuredBranch == "" || branch != configuredBranch || !sameRepository(site, event.Repository) {
		_ = s.recordAudit(r.Context(), "webhook", event.Delivery, "webhook.ignored", "site", site.ID, map[string]any{"provider": provider, "branch": branch, "reason": "repository_or_branch_mismatch"})
		writeJSON(w, http.StatusAccepted, map[string]any{"data": map[string]any{"status": "ignored", "reason": "repository_or_branch_mismatch", "delivery_id": event.Delivery}})
		return
	}
	if site.Strategy != "atomic" {
		_ = s.markWebhookDeliveryError(r.Context(), config.id, event.Delivery, "automatic deployment requires Atomic releases")
		writeJSON(w, http.StatusAccepted, map[string]any{"data": map[string]any{"status": "ignored", "reason": "atomic_strategy_required", "delivery_id": event.Delivery}})
		return
	}
	queued, err := s.deployQueue.Enqueue(r.Context(), site, event.After, "webhook", "webhook")
	if err != nil {
		_ = s.markWebhookDeliveryError(r.Context(), config.id, event.Delivery, err.Error())
		writeError(w, http.StatusUnprocessableEntity, "DEPLOYMENT_NOT_QUEUED", "The push was verified but could not be queued.")
		return
	}
	_ = s.attachWebhookDeployment(r.Context(), config.id, event.Delivery, queued.ID)
	_ = s.recordAudit(r.Context(), "webhook", event.Delivery, "deployment.queued", "deployment", queued.ID, map[string]any{"site_id": site.ID, "provider": provider, "branch": branch, "commit": event.After, "pending_policy": "latest"})
	writeJSON(w, http.StatusAccepted, map[string]any{"data": map[string]any{"status": "queued", "delivery_id": event.Delivery, "deployment_id": queued.ID, "commit": event.After}})
}

func returnWebhookFailure(w http.ResponseWriter, auditErr, original error) {
	if auditErr != nil {
		writeError(w, http.StatusInternalServerError, "WEBHOOK_UNAVAILABLE", auditErr.Error())
		return
	}
	writeError(w, http.StatusInternalServerError, "WEBHOOK_UNAVAILABLE", original.Error())
}

type webhookRecord struct {
	id              string
	siteID          string
	provider        string
	encryptedToken  []byte
	encryptedSecret []byte
	lastDeliveryAt  string
	lastError       string
	enabled         bool
}

var errWebhookNotFound = errors.New("webhook not found")

func (s *Server) findWebhook(ctx context.Context, provider, token string) (webhookRecord, error) {
	digest := sha256.Sum256([]byte(token))
	var item webhookRecord
	var enabled int
	err := s.db.QueryRowContext(ctx, `SELECT id, site_id, provider, encrypted_public_token, encrypted_secret, COALESCE(last_delivery_at, ''), COALESCE(last_error, ''), enabled FROM webhooks WHERE provider = ? AND public_token_hash = ? LIMIT 1`, provider, hex.EncodeToString(digest[:])).Scan(&item.id, &item.siteID, &item.provider, &item.encryptedToken, &item.encryptedSecret, &item.lastDeliveryAt, &item.lastError, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return webhookRecord{}, errWebhookNotFound
	}
	if err != nil {
		return webhookRecord{}, fmt.Errorf("find webhook: %w", err)
	}
	item.enabled = enabled != 0
	if !item.enabled {
		return webhookRecord{}, errWebhookNotFound
	}
	return item, nil
}

func (s *Server) webhookSecret(encrypted []byte) (string, error) {
	plain, err := s.secrets.Open(encrypted)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func verifyWebhookSignature(provider, secret string, headers http.Header, body []byte) bool {
	if provider == "gitlab" {
		if token := headers.Get("X-Gitlab-Token"); token != "" && hmac.Equal([]byte(token), []byte(secret)) {
			return true
		}
	}
	value := headers.Get("X-Hub-Signature-256")
	if value == "" {
		value = headers.Get("X-Gitlab-Signature")
	}
	value = strings.TrimPrefix(strings.TrimSpace(value), "sha256=")
	provided, err := hex.DecodeString(value)
	if err != nil || len(provided) != sha256.Size {
		return false
	}
	hasher := hmac.New(sha256.New, []byte(secret))
	_, _ = hasher.Write(body)
	return hmac.Equal(provided, hasher.Sum(nil))
}

func parseWebhookPush(provider string, headers http.Header, body []byte) (webhookPushEvent, error) {
	var raw struct {
		Ref        string `json:"ref"`
		After      string `json:"after"`
		Deleted    bool   `json:"deleted"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
		Project struct {
			PathWithNamespace string `json:"path_with_namespace"`
		} `json:"project"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return webhookPushEvent{}, fmt.Errorf("decode webhook JSON: %w", err)
	}
	repository := raw.Repository.FullName
	if provider == "gitlab" {
		repository = raw.Project.PathWithNamespace
	}
	return webhookPushEvent{EventName: webhookEventName(provider, headers), Delivery: webhookDeliveryID(provider, headers), Repository: normalizeRepository(repository), Ref: raw.Ref, After: strings.TrimSpace(raw.After), Deleted: raw.Deleted}, nil
}

func webhookEventName(provider string, headers http.Header) string {
	if provider == "gitlab" {
		value := strings.ToLower(strings.TrimSpace(headers.Get("X-Gitlab-Event")))
		if strings.Contains(value, "push") {
			return "push"
		}
		return value
	}
	return strings.ToLower(strings.TrimSpace(headers.Get("X-GitHub-Event")))
}

func webhookDeliveryID(provider string, headers http.Header) string {
	if provider == "gitlab" {
		for _, key := range []string{"X-Gitlab-Event-UUID", "Idempotency-Key", "X-Gitlab-Delivery"} {
			if value := strings.TrimSpace(headers.Get(key)); value != "" {
				return value
			}
		}
		return ""
	}
	return strings.TrimSpace(headers.Get("X-GitHub-Delivery"))
}

func normalizeRepository(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "://") {
		if parsed, err := url.Parse(value); err == nil {
			value = parsed.Path
		}
	} else if strings.HasPrefix(value, "git@") && strings.Contains(value, ":") {
		value = strings.SplitN(value, ":", 2)[1]
	}
	return strings.Trim(strings.TrimSuffix(strings.TrimSpace(value), ".git"), "/")
}

func webhookTokenFromPath(path, provider string) string {
	marker := "/webhooks/" + provider + "/"
	index := strings.LastIndex(path, marker)
	if index < 0 {
		return ""
	}
	return strings.Trim(strings.TrimPrefix(path[index:], marker), "/")
}

func sameRepository(site sites.Site, incoming string) bool {
	configured := ""
	if site.Repository != nil {
		configured = site.Repository.GitHubFullName
		if configured == "" {
			configured = site.Repository.CloneURL
		}
	}
	return strings.EqualFold(normalizeRepository(configured), normalizeRepository(incoming)) && normalizeRepository(incoming) != ""
}

func allZeroSHA(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && strings.Trim(value, "0") == ""
}

func (s *Server) recordWebhookDelivery(ctx context.Context, config webhookRecord, event webhookPushEvent, payloadHash string) (bool, error) {
	var existing string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM webhook_deliveries WHERE webhook_id = ? AND provider_delivery_id = ?`, config.id, event.Delivery).Scan(&existing)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("check webhook replay: %w", err)
	}
	id, err := newAuditID()
	if err != nil {
		return false, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO webhook_deliveries(id, webhook_id, provider_delivery_id, event, signature_valid, payload_sha256, received_at) VALUES (?, ?, ?, ?, 1, ?, datetime('now'))`, "delivery_"+strings.TrimPrefix(id, "audit_"), config.id, event.Delivery, event.EventName, payloadHash)
	if err != nil {
		return false, fmt.Errorf("record webhook delivery: %w", err)
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE webhooks SET last_delivery_at = datetime('now'), last_error = NULL WHERE id = ?`, config.id)
	return false, nil
}

func (s *Server) markWebhookDeliveryError(ctx context.Context, webhookID, deliveryID, message string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE webhook_deliveries SET event = COALESCE(event, '') || ': ' || ? WHERE webhook_id = ? AND provider_delivery_id = ?`, message, webhookID, deliveryID)
	_, _ = s.db.ExecContext(ctx, `UPDATE webhooks SET last_error = ? WHERE id = ?`, message, webhookID)
	return err
}

func (s *Server) attachWebhookDeployment(ctx context.Context, webhookID, deliveryID, deploymentID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE webhook_deliveries SET deployment_id = ? WHERE webhook_id = ? AND provider_delivery_id = ?`, deploymentID, webhookID, deliveryID)
	return err
}

func newRandomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func (s *Server) webhookForSite(ctx context.Context, siteID string) (webhookView, error) {
	var item webhookRecord
	var enabled int
	err := s.db.QueryRowContext(ctx, `SELECT id, site_id, provider, encrypted_public_token, encrypted_secret, COALESCE(last_delivery_at, ''), COALESCE(last_error, ''), enabled FROM webhooks WHERE site_id = ? LIMIT 1`, siteID).Scan(&item.id, &item.siteID, &item.provider, &item.encryptedToken, &item.encryptedSecret, &item.lastDeliveryAt, &item.lastError, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return webhookView{SiteID: siteID, Enabled: false, Configured: false}, nil
	}
	if err != nil {
		return webhookView{}, fmt.Errorf("read site webhook: %w", err)
	}
	view := webhookView{ID: item.id, SiteID: siteID, Provider: item.provider, Enabled: enabled != 0, Configured: len(item.encryptedToken) > 0, SecretConfigured: len(item.encryptedSecret) > 0, LastDeliveryAt: item.lastDeliveryAt, LastError: item.lastError}
	if len(item.encryptedToken) > 0 {
		token, err := s.secrets.Open(item.encryptedToken)
		if err != nil {
			return webhookView{}, fmt.Errorf("read site webhook token: %w", err)
		}
		view.Endpoint = publicWebhookPrefix + "/" + item.provider + "/" + string(token)
	}
	return view, nil
}

func (s *Server) rotateSiteWebhook(ctx context.Context, site sites.Site, provider string) (webhookView, string, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" && site.Repository != nil {
		provider = strings.ToLower(strings.TrimSpace(site.Repository.Provider))
	}
	if provider != "github" && provider != "gitlab" {
		return webhookView{}, "", fmt.Errorf("webhook provider must be github or gitlab")
	}
	token, err := newRandomToken(32)
	if err != nil {
		return webhookView{}, "", err
	}
	secret, err := newRandomToken(32)
	if err != nil {
		return webhookView{}, "", err
	}
	encryptedToken, err := s.secrets.Seal([]byte(token))
	if err != nil {
		return webhookView{}, "", err
	}
	encryptedSecret, err := s.secrets.Seal([]byte(secret))
	if err != nil {
		return webhookView{}, "", err
	}
	digest := sha256.Sum256([]byte(token))
	id, err := newAuditID()
	if err != nil {
		return webhookView{}, "", err
	}
	id = "wh_" + strings.TrimPrefix(id, "audit_")
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return webhookView{}, "", fmt.Errorf("begin webhook rotation: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM webhooks WHERE site_id = ?`, site.ID); err != nil {
		return webhookView{}, "", fmt.Errorf("replace site webhook: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO webhooks(id, site_id, provider, public_token_hash, encrypted_secret, encrypted_public_token, enabled, created_at) VALUES (?, ?, ?, ?, ?, ?, 1, datetime('now'))`, id, site.ID, provider, hex.EncodeToString(digest[:]), encryptedSecret, encryptedToken); err != nil {
		return webhookView{}, "", fmt.Errorf("store site webhook: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return webhookView{}, "", fmt.Errorf("commit webhook rotation: %w", err)
	}
	view := webhookView{ID: id, SiteID: site.ID, Provider: provider, Enabled: true, Configured: true, SecretConfigured: true, Endpoint: publicWebhookPrefix + "/" + provider + "/" + token}
	return view, secret, nil
}

func (s *Server) handleSiteWebhook(w http.ResponseWriter, r *http.Request, siteID string) {
	site, err := s.sites.Get(r.Context(), siteID)
	if errors.Is(err, sites.ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Site not found.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SITES_UNAVAILABLE", err.Error())
		return
	}
	if r.Method == http.MethodGet {
		view, err := s.webhookForSite(r.Context(), siteID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "WEBHOOK_UNAVAILABLE", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": view})
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET and POST are supported.")
		return
	}
	var payload struct {
		Provider string `json:"provider"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&payload)
	}
	view, secret, err := s.rotateSiteWebhook(r.Context(), site, payload.Provider)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_WEBHOOK", err.Error())
		return
	}
	_ = s.recordAudit(r.Context(), "user", "local", "webhook.rotated", "site", site.ID, map[string]any{"provider": view.Provider})
	writeJSON(w, http.StatusCreated, map[string]any{"data": map[string]any{"webhook": view, "secret": secret, "warning": "Copy this secret now. It will not be shown again."}})
}

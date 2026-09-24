package plugin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const (
	datadogPersonalAccessTokenType = "datadog_personal_access_token"
	datadogServiceAccessTokenType  = "datadog_service_access_token"

	// minAccessTokenLifetime is the shortest expiry datadog accepts for a
	// personal access token.
	minAccessTokenLifetime = 24 * time.Hour
	// accessTokenExpiryMargin keeps the datadog expiry past the end of the
	// lease despite request latency and clock skew.
	accessTokenExpiryMargin = 5 * time.Minute
)

// datadogAccessToken is a personal (ddpat_) or service account (ddsat_)
// access token.
type datadogAccessToken struct {
	ID    string `json:"token_id"`
	Token string `json:"token"`
}

func (b *datadogBackend) datadogPersonalAccessToken() *framework.Secret {
	return &framework.Secret{
		Type: datadogPersonalAccessTokenType,
		Fields: map[string]*framework.FieldSchema{
			"token": {
				Type:        framework.TypeString,
				Description: "datadog personal access token",
			},
		},
		Renew:  b.accessTokenRenew,
		Revoke: b.personalAccessTokenRevoke,
	}
}

func (b *datadogBackend) datadogServiceAccessToken() *framework.Secret {
	return &framework.Secret{
		Type: datadogServiceAccessTokenType,
		Fields: map[string]*framework.FieldSchema{
			"token": {
				Type:        framework.TypeString,
				Description: "datadog service account access token",
			},
		},
		Renew:  b.accessTokenRenew,
		Revoke: b.serviceAccessTokenRevoke,
	}
}

func (b *datadogBackend) accessTokenRenew(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	role, err := internalString(req.Secret, "role")
	if err != nil {
		return nil, err
	}

	roleEntry, err := b.getRole(ctx, req.Storage, role)
	if err != nil {
		return nil, fmt.Errorf("error retrieving role: %w", err)
	}
	if roleEntry == nil {
		return nil, errors.New("error retrieving role: role is nil")
	}

	resp := &logical.Response{Secret: req.Secret}
	if roleEntry.TTL > 0 {
		resp.Secret.TTL = roleEntry.TTL
	}
	if roleEntry.MaxTTL > 0 {
		resp.Secret.MaxTTL = roleEntry.MaxTTL
	}
	if err := capAtTokenExpiry(resp.Secret); err != nil {
		return nil, err
	}
	return resp, nil
}

func (b *datadogBackend) personalAccessTokenRevoke(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	tokenID, err := internalString(req.Secret, "token_id")
	if err != nil {
		return nil, err
	}

	client, err := b.getClient(ctx, req.Storage)
	if err != nil {
		return nil, fmt.Errorf("error getting client: %w", err)
	}

	if err := client.revokePersonalAccessToken(ctx, tokenID); err != nil {
		return nil, err
	}
	return nil, nil
}

func (b *datadogBackend) serviceAccessTokenRevoke(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	tokenID, err := internalString(req.Secret, "token_id")
	if err != nil {
		return nil, err
	}
	serviceAccountID, err := internalString(req.Secret, "service_account_id")
	if err != nil {
		return nil, err
	}

	client, err := b.getClient(ctx, req.Storage)
	if err != nil {
		return nil, fmt.Errorf("error getting client: %w", err)
	}

	if err := client.revokeServiceAccessToken(ctx, serviceAccountID, tokenID); err != nil {
		return nil, err
	}
	return nil, nil
}

// internalString returns a non-empty string value from the secret's internal data.
func internalString(secret *logical.Secret, key string) (string, error) {
	value, ok := secret.InternalData[key].(string)
	if !ok || value == "" {
		return "", fmt.Errorf("secret is missing %s internal data", key)
	}
	return value, nil
}

// accessTokenExpiry returns when a newly issued token should expire at the
// latest on the datadog side. Vault revokes the token when its lease ends; the
// expiry bounds its lifetime should that revocation ever fail. It covers the
// longest possible lease and never goes below what datadog accepts.
func accessTokenExpiry(now time.Time, roleMaxTTL, systemMaxTTL time.Duration) time.Time {
	lifetime := roleMaxTTL
	if lifetime <= 0 || (systemMaxTTL > 0 && lifetime > systemMaxTTL) {
		lifetime = systemMaxTTL
	}
	if lifetime < minAccessTokenLifetime {
		lifetime = minAccessTokenLifetime
	}
	return now.Add(lifetime + accessTokenExpiryMargin)
}

// capAtTokenExpiry keeps a renewed lease from outliving the token's datadog
// expiry, e.g. after the role's max_ttl was raised.
func capAtTokenExpiry(secret *logical.Secret) error {
	raw, err := internalString(secret, "expires_at")
	if err != nil {
		return err
	}
	expiresAt, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return fmt.Errorf("invalid expires_at internal data: %w", err)
	}

	remaining := expiresAt.Sub(secret.IssueTime) - accessTokenExpiryMargin
	if secret.MaxTTL == 0 || secret.MaxTTL > remaining {
		secret.MaxTTL = remaining
	}
	return nil
}

package plugin

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const (
	personalAccessTokenPath        = "pat/"
	pathPersonalAccessTokenHelpSyn = `
	Generate a datadog personal access token (ddpat_) from a role.
	`
	pathPersonalAccessTokenHelpDesc = `
	This path generates a datadog personal access token for the user that
	owns the configured Application Key, scoped to the role's
	access_token_scopes.
	`
	serviceAccessTokenPath        = "sat/"
	pathServiceAccessTokenHelpSyn = `
	Generate a datadog service account access token (ddsat_) from a role.
	`
	pathServiceAccessTokenHelpDesc = `
	This path generates a datadog access token for the role's
	service_account_id, scoped to the role's access_token_scopes.
	`
)

func pathPersonalAccessToken(b *datadogBackend) *framework.Path {
	return accessTokenPath(personalAccessTokenPath, b.pathPersonalAccessTokenRead, pathPersonalAccessTokenHelpSyn, pathPersonalAccessTokenHelpDesc)
}

func pathServiceAccessToken(b *datadogBackend) *framework.Path {
	return accessTokenPath(serviceAccessTokenPath, b.pathServiceAccessTokenRead, pathServiceAccessTokenHelpSyn, pathServiceAccessTokenHelpDesc)
}

func accessTokenPath(prefix string, callback framework.OperationFunc, helpSyn, helpDesc string) *framework.Path {
	return &framework.Path{
		Pattern: prefix + framework.GenericNameRegex("name"),
		Fields: map[string]*framework.FieldSchema{
			"name": {
				Type:        framework.TypeString,
				Description: "Name of the role",
				Required:    true,
			},
		},
		Callbacks: map[logical.Operation]framework.OperationFunc{
			logical.ReadOperation:   callback,
			logical.UpdateOperation: callback,
		},
		HelpSynopsis:    helpSyn,
		HelpDescription: helpDesc,
	}
}

func (b *datadogBackend) pathPersonalAccessTokenRead(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	roleEntry, errResp, err := b.accessTokenRole(ctx, req, d)
	if errResp != nil || err != nil {
		return errResp, err
	}

	client, name, err := b.accessTokenClientAndName(ctx, req, roleEntry)
	if err != nil {
		return nil, err
	}

	expiresAt := b.accessTokenExpiry(roleEntry)
	token, err := client.createPersonalAccessToken(ctx, name, roleEntry.AccessTokenScopes, expiresAt)
	if err != nil {
		return nil, err
	}

	return accessTokenResponse(b.Secret(datadogPersonalAccessTokenType), roleEntry, token, expiresAt, nil), nil
}

func (b *datadogBackend) pathServiceAccessTokenRead(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	roleEntry, errResp, err := b.accessTokenRole(ctx, req, d)
	if errResp != nil || err != nil {
		return errResp, err
	}
	if roleEntry.ServiceAccountID == "" {
		return logical.ErrorResponse("role %q has no service_account_id", roleEntry.Name), nil
	}

	client, name, err := b.accessTokenClientAndName(ctx, req, roleEntry)
	if err != nil {
		return nil, err
	}

	expiresAt := b.accessTokenExpiry(roleEntry)
	token, err := client.createServiceAccessToken(ctx, roleEntry.ServiceAccountID, name, roleEntry.AccessTokenScopes, expiresAt)
	if err != nil {
		return nil, err
	}

	internal := map[string]interface{}{"service_account_id": roleEntry.ServiceAccountID}
	return accessTokenResponse(b.Secret(datadogServiceAccessTokenType), roleEntry, token, expiresAt, internal), nil
}

// accessTokenRole loads the requested role and checks that it can issue access tokens.
func (b *datadogBackend) accessTokenRole(ctx context.Context, req *logical.Request, d *framework.FieldData) (*datadogRoleEntry, *logical.Response, error) {
	roleName := d.Get("name").(string)

	roleEntry, err := b.getRole(ctx, req.Storage, roleName)
	if err != nil {
		return nil, nil, fmt.Errorf("error retrieving role: %w", err)
	}
	if roleEntry == nil {
		return nil, logical.ErrorResponse("role %q not found", roleName), nil
	}
	if len(roleEntry.AccessTokenScopes) == 0 {
		return nil, logical.ErrorResponse("role %q has no access_token_scopes", roleName), nil
	}
	return roleEntry, nil, nil
}

func (b *datadogBackend) accessTokenClientAndName(ctx context.Context, req *logical.Request, roleEntry *datadogRoleEntry) (*datadogClient, string, error) {
	client, err := b.getClient(ctx, req.Storage)
	if err != nil {
		return nil, "", fmt.Errorf("error getting client: %w", err)
	}

	id, err := uuid.GenerateUUID()
	if err != nil {
		return nil, "", fmt.Errorf("error generating UUID for access token name: %w", err)
	}
	return client, roleEntry.Name + "-" + id, nil
}

func (b *datadogBackend) accessTokenExpiry(roleEntry *datadogRoleEntry) time.Time {
	return accessTokenExpiry(time.Now(), roleEntry.MaxTTL, b.System().MaxLeaseTTL())
}

func accessTokenResponse(secret *framework.Secret, roleEntry *datadogRoleEntry, token *datadogAccessToken, expiresAt time.Time, internal map[string]interface{}) *logical.Response {
	internalData := map[string]interface{}{
		"token_id":   token.ID,
		"role":       roleEntry.Name,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
	}
	for k, v := range internal {
		internalData[k] = v
	}

	resp := secret.Response(map[string]interface{}{
		"token":      token.Token,
		"token_id":   token.ID,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
	}, internalData)

	if roleEntry.TTL > 0 {
		resp.Secret.TTL = roleEntry.TTL
	}
	if roleEntry.MaxTTL > 0 {
		resp.Secret.MaxTTL = roleEntry.MaxTTL
	}
	return resp
}

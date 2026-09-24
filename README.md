# vault-plugin-secrets-datadog

This is a HashiCorp Vault Plugin that interacts with the Datadog
Platform to generate API and Application Keys. Application Keys can
be scoped through the use of Vault roles.

The goal of this plugin is to reduce the risk of accidental exposure
of Datadog API and Application Keys by manually generating and handling
them.

## Testing Locally

If you're compiling this yourself and want to do a local sanity test, you
can do something like the following using 2 separate terminals:

```bash
terminal-1$ make
...
terminal-2$ export VAULT_ADDR=http://127.0.0.1:8200
terminal-2$ export VAULT_TOKEN=root
terminal-2$ export DATADOG_API_KEY=<valid datadog api key>
terminal-2$ export DATADOG_API_KEY_ID=<the ID for the above API key>
terminal-2$ export DATADOG_APP_KEY=<valid datadog app key scoped to allow for generating both api and app keys>
terminal-2$ export DATADOG_APP_KEY_ID=<the ID for the above application key>
terminal-2$ export DATADOG_SITE=datadoghq.eu # optional, defaults to datadoghq.com
terminal-2$ make setup
...
terminal-2$ vault read datadog/apikey/test
terminal-2$ vault read datadog/appkey/test
```
This will generate both an API and App Key, the former being scoped for `incident_read` and `usage_read` permissions. (hardcoded in the makefile)

## Installation

### Using pre-built releases

You can find pre-built releases of the plugin [here][ddreleases]. Once you have downloaded the latest archive corresponding to your target OS, uncompress it to retrieve the `vault-plugin-secrets-datadog`  binary file. Move this to each of your Vault nodes where they store plugins.

### From Source

If you prefer to build the plugin from sources, clone the GitHub repository locally and run the command `make build` from the root of the sources directory. Upon successful compilation, the resulting `vault-plugin-secrets-datadog` binary is stored in the `vault/plugins` directory.

## Configuration

Copy the plugin binary into a location of your choice; this directory must be specified as the [`plugin_directory`][vaultdocplugindir] in the Vault configuration file:

```hcl
plugin_directory = "path/to/plugin/directory"
```

Start a Vault server with this configuration file:

```sh
vault server -config=path/to/vault/config.hcl
```

Once the server is started, register the plugin in the Vault server's [plugin catalog][vaultdocplugincatalog]:

```sh
$ vault write sys/plugins/catalog/secret/datadog \
    sha_256="$(sha256sum path/to/plugin/directory/vault-plugin-secrets-datadog | cut -d " " -f 1)" \
    command="datadog"
```

You can now enable the Datadog secrets plugin:

```sh
vault secrets enable datadog
```

## Usage

### Datadog

You will need the "admin" user's password (not an admin, but admin specifically).

1. Log into the Datadog UI as an admin.
2. Hover over your username on the left panel and click "Organization Settings"

Now you will create the API and Application Keys that Vault will use to execute the creation and deletion of API and App Keys.

1. Under "Organization Settings" click API Keys, then click "New Key" in the upper right corner.
2. Give the Key a name like `vault-dd-api-key`
3. Save the Key and repeat the process for an Application Key (found under "Organization Settings)
4. In a terminal, export API_KEY, APP_KEY, API_KEY_ID, and APP_KEY_ID with the respective keys/IDs.

See [Datadog documentation][datadog-create-token] about creating API and App Keys for any help you may need.

* Write the config into Vault:
```sh
vault write datadog/config \
    api_key=$API_KEY \
    app_key=$APP_KEY \
    api_key_id=$API_KEY_ID \
    app_key_id=$APP_KEY_ID \
    site=datadoghq.eu
```

`site` is the [Datadog site][datadog-sites] of your organization and defaults to `datadoghq.com` (US1). Use `datadoghq.eu` for organizations hosted in Europe. The other supported values are `us3.datadoghq.com`, `us5.datadoghq.com`, `ap1.datadoghq.com`, `ap2.datadoghq.com` and `ddog-gov.com`.

* Rotate the API and App Keys, so that only vault (and datadog admins with access to the console) knows them.

```sh
vault read datadog/config/rotate
```

* Validate that the keys were rotated

```sh
vault read datadog/config
Key           Value
---           -----
api_key_id    7dd441ac-d9ff-4e7b-9a23-80cff4a3458e
app_key_id    8f412eca-e899-4af9-8e38-33302321d3f7
site          datadoghq.eu
```

* Create a Role:

```sh
$ vault write datadog/roles/test \
    app_key_scopes=incident_read,usage_read \
    ttl=1h max_ttl=3h
```

```sh
$ vault list datadog/roles
Keys
----
test
```

* Test with the creation of an API and Application key:
```sh
$ vault read datadog/apikey/test
Key                Value
---                -----
lease_id           datadog/apikey/test/j2IPQja7sF1KVrNhj4k8VTiM
lease_duration     2h
lease_renewable    true
api_key            <REDACTED>
```
```sh
$ vault read datadog/appkey/test
Key                Value
---                -----
lease_id           datadog/appkey/test/DCDdWYBROZRIQQfmOv2C4SUP
lease_duration     2h
lease_renewable    true
app_key            <REDACTED>
```

### Personal and service account access tokens

The plugin can also issue Datadog [access tokens][datadog-access-tokens]: personal access tokens (`ddpat_...`) and service account access tokens (`ddsat_...`). Give the role the scopes the tokens should get, and for service account tokens the ID of the service account:

```sh
$ vault write datadog/roles/tokens \
    access_token_scopes=dashboards_read,monitors_read \
    service_account_id=00000000-0000-0000-0000-000000000000 \
    ttl=1h max_ttl=48h
```

* Generate a personal access token. It belongs to the user that owns the configured Application Key:

```sh
$ vault read datadog/pat/tokens
Key                Value
---                -----
lease_id           datadog/pat/tokens/Qm1Ys1F8i5nC2KjQx0nqbzGH
lease_duration     1h
lease_renewable    true
token              <REDACTED>
token_id           2c0c5d4e-6a55-4f3e-9d37-1b2f0a4a5e61
```

* Generate a service account access token for the role's `service_account_id`:

```sh
$ vault read datadog/sat/tokens
```

A personal access token belongs to the Datadog user that owns the configured Application Key. If you later replace `app_key` in `datadog/config` with a key of a different user, that user may not be allowed to revoke tokens issued before the change. `config/rotate` keeps the same user, so it is safe.

Vault revokes the token in Datadog when the lease expires or is revoked. A token that is already gone in Datadog (404) counts as revoked. Renewals are capped so a lease never outlives its token. As a safety net, each token is also created with a Datadog expiry that covers the longest possible lease: the role's `max_ttl`, or the system max lease TTL when it is not set. The expiry is never less than 24 hours, the minimum Datadog accepts.

## Issues

[vault-plugin-secrets-datadog Issues][issues]

[ddreleases]: https://github.com/schubergphilis/vault-plugin-secrets-datadog/releases
[vaultdocplugindir]: https://www.vaultproject.io/docs/configuration/index.html#plugin_directory
[vaultdocplugincatalog]: https://www.vaultproject.io/docs/internals/plugins.html#plugin-catalog
[datadog-create-token]: https://docs.datadoghq.com/account_management/api-app-keys/
[issues]: https://github.com/schubergphilis/vault-plugin-secrets-datadog/issues
[datadog-access-tokens]: https://docs.datadoghq.com/account_management/api-app-keys/
[datadog-sites]: https://docs.datadoghq.com/getting_started/site/
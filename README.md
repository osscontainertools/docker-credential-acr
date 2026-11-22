# ACR Docker Credential Helper

The ACR docker credential helper is an alternative to the existing file store based ACR helper 
located [here](https://github.com/Azure/acr-docker-credential-helper) which relies on `az` command
line and is not optimised for use in CI environments. Primary use case for this helper is for use
with kaniko and other tools running in CI scenarios wishing to push to Azure Container Registry

> [!IMPORTANT]
> This is the maintained continuation of
> [`chrismellard/docker-credential-acr-env`](https://github.com/chrismellard/docker-credential-acr-env),
> which has gone defunct in March of 2023.
> The focus of this fork is to keep dependencies up-to-date and fix bugs.

## How it works

The credential helper sources its configuration from well-known Azure environmental information.
It attempts to authenticate firstly via client credentials grant if the following environment config is present

```
AZURE_CLIENT_ID=<clientID>
AZURE_CLIENT_SECRET=<clientSecret>
AZURE_TENANT_ID=<tenantId>
```

If the details needed for the client credential grant are not set it will try to 
find a [federated OIDC JWT](https://learn.microsoft.com/en-us/graph/api/resources/federatedidentitycredentials-overview?view=graph-rest-1.0) 
in the enviroment. To use this set the following values in the enviroment.

```
AZURE_CLIENT_ID=<clientID>
AZURE_FEDERATED_TOKEN=<federatedJWT>
AZURE_TENANT_ID=<tenantId>
```

If you use federated OIDC with [Azure Workload Identity](https://github.com/Azure/azure-workload-identity) you don't
have to set any ENVs as they will get injected automatically.

If the above are not set then authentication falls back to managed service identities and the MSI endpoint is
attempted to be contacted which will work in various Azure contexts such as App Service and Azure Kubernetes Service
where the MSI endpoint will authenticate the MSI context the service is running under.

## Feature Flags

### Flag `FF_DOCKER_ACR_AZIDENTITY`

Token acquisition goes through go-autorest, which Microsoft retired. Set this flag to `true` to go through azidentity instead, its successor in the Azure SDK for Go.
Defaults to `true`.
Will be deprecated in `v1.1.0`.

### Flag `FF_DOCKER_ACR_CREDENTIALS_NOT_FOUND`

For a host that is not an ACR registry the helper fails with a plain error. Docker and go-containerregistry only fall back to an anonymous pull on the credential helper protocol's not-found error, so a `credHelpers` entry that maps such a host to this helper fails the pull.
Set this flag to `true` to report credentials not found for hosts that are not an ACR registry.
Defaults to `false`.
Becomes default in `v1.1.0`.

### Flag `FF_DOCKER_ACR_SKIP_MCR`

The helper treats `mcr.microsoft.com` as an ACR host and asks Azure for a token, although MCR is anonymous and has no token exchange. Outside Azure every MCR lookup waits on the managed identity endpoint for up to 30 seconds.
Set this flag to `true` to report no credentials for `mcr.microsoft.com`.
Defaults to `false`.
Becomes default in `v1.1.0`.

### Flag `FF_DOCKER_ACR_STRICT_HOST_CHECK`

The registry check only looks at whether the parsed host ends in an ACR domain, so it also accepts userinfo, a path, a query, a fragment and percent-escapes. Set this flag to `true` to only accept input that is exactly the parsed `host[:port]`.
Defaults to `true`.
Will be deprecated in `v1.1.0`.

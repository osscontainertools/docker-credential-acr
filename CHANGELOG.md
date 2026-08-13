# v0.8.0 Release 2026-08-13

## Community Update
Many thanks to @chrismellard for maintaining [`docker-credential-acr-env`](https://github.com/chrismellard/docker-credential-acr-env) over all those years. We are happy to continue his work and will honour his contributions. This helper is used inside our [kaniko](https://github.com/osscontainertools/kaniko) executor image, and we promise to maintain it to the same standard.

## What's Changed
### Security
* the ACR host pattern was unanchored, so any hostname that contained `azurecr.io` was treated as ACR and handed an Azure access token: https://github.com/osscontainertools/docker-credential-acr/commit/5ce8219311b6db99855036fbf27bedd35f38d250
* golang.org/x/crypto v0.0.0-20220722155217-630584e8d5aa: CVE-2023-48795 CVE-2024-45337 CVE-2025-22869 CVE-2025-47913 CVE-2025-47914 CVE-2025-58181 CVE-2026-39827 CVE-2026-39828 CVE-2026-39829 CVE-2026-39830 CVE-2026-39831 CVE-2026-39832 CVE-2026-39833 CVE-2026-39834 CVE-2026-39835 CVE-2026-42508 CVE-2026-46595 CVE-2026-46597 CVE-2026-46598
* golang.org/x/text v0.3.6: CVE-2021-38561 CVE-2022-32149 CVE-2026-56852
* golang.org/x/sys v0.0.0-20220325203850-36772127a21f: CVE-2022-29526 CVE-2026-39824
* github.com/golang-jwt/jwt/v4 v4.2.0: CVE-2024-51744 CVE-2025-30204
* the toolchain is pinned to go1.26.5, so released binaries carry a current standard library

### Usability
* Add `--version`: https://github.com/osscontainertools/docker-credential-acr/commit/93ea027980dd6b785b6217dae866a09c9a99bdc5

### Maintenance
* bump go from 1.18 to 1.26
* bump github.com/Azure/azure-sdk-for-go from v46.4.0 to v68.0.0
* bump github.com/Azure/go-autorest/autorest from v0.11.28 to v0.11.30
* bump github.com/Azure/go-autorest/autorest/adal from v0.9.21 to v0.9.24
* bump github.com/Azure/go-autorest/autorest/azure/auth from v0.5.11 to v0.5.13
* bump github.com/docker/docker-credential-helpers from v0.6.3 to v0.9.8
* bump github.com/golang-jwt/jwt/v4 from v4.2.0 to v4.5.2
* bump github.com/spf13/cobra from v1.2.0 to v1.10.2
* bump github.com/google/go-containerregistry to v0.21.9: https://github.com/osscontainertools/docker-credential-acr/pull/3

### Fork Related
* Rename the module and the binary to `docker-credential-acr`: https://github.com/osscontainertools/docker-credential-acr/commit/5164fb52536071cafb629a7487899291807ad46e
* Add integration tests against a live ACR, covering the federated and the managed identity route: https://github.com/osscontainertools/docker-credential-acr/commit/5cf9c6fafd9265d7dd5b15379b3f04eaf1003b6d https://github.com/osscontainertools/docker-credential-acr/commit/dc685b2c672eedef7f7053fbd4e4443e6fc6a1ad
* Drop viper and the config file parsers it pulled in: https://github.com/osscontainertools/docker-credential-acr/commit/7f1ea5845809990e0af6ac06ccbbce57f7146224
* Vendor the dependencies: https://github.com/osscontainertools/docker-credential-acr/commit/ca08f865237ebf2764e6863bab0c35d4c5f7082d
* Harden the CI, add a nightly vulnerability scan, publish releases as drafts: https://github.com/osscontainertools/docker-credential-acr/commit/c2658c8e1c343423c8297a31b93fba5ff1f0d9d7 https://github.com/osscontainertools/docker-credential-acr/commit/a515e18141395948516b6b44e21df4bac20c332a
* Add a security policy: https://github.com/osscontainertools/docker-credential-acr/commit/37f465e7c17e225295616886499ec8dd4e1f8413
* Clean up the repo tooling: https://github.com/osscontainertools/docker-credential-acr/commit/40029769205903b13b6de20e6af1e1ed34b6a05f https://github.com/osscontainertools/docker-credential-acr/commit/f3bd06692ddc84f8c45759b0f4d67ed0040032ea

### Refactorings
* adal: NewServicePrincipalTokenFromFederatedToken is deprecated: https://github.com/osscontainertools/docker-credential-acr/commit/698a07d4ad4309f7b16af88a148eef05435ecbfd
* Linter pass: https://github.com/osscontainertools/docker-credential-acr/commit/236ca560acc5e0d24376c35477fd18aa8fe07cc1

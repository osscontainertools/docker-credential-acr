/*
Copyright © 2020 Chris Mellard chris.mellard@icloud.com

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package registry

import (
	"context"
	"fmt"
	"net/url"

	"github.com/Azure/azure-sdk-for-go/services/preview/containerregistry/runtime/2019-08-15-preview/containerregistry"
	"github.com/Azure/go-autorest/autorest"
)

// staticToken satisfies adal.OAuthTokenProvider for an already acquired token, so
// the exchange does not have to know how the token was obtained
type staticToken string

func (t staticToken) OAuthToken() string { return string(t) }

// GetRefreshToken exchanges an Azure AD access token for an OAuth2 refresh token for
// the registry specified by serverURL
func GetRefreshToken(ctx context.Context, serverURL, tenantID, accessToken string) (string, error) {
	registryName, err := getRegistryURL(serverURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse server URL - %w", err)
	}

	refreshTokenClient := containerregistry.NewRefreshTokensClient(registryName.String())
	refreshTokenClient.Authorizer = autorest.NewBearerAuthorizer(staticToken(accessToken))

	rt, err := refreshTokenClient.GetFromExchange(ctx, "access_token", serverURL, tenantID, "", accessToken)
	if err != nil {
		return "", fmt.Errorf("failed to get refresh token for container registry - %w", err)
	}
	if rt.RefreshToken == nil {
		return "", fmt.Errorf("no refresh token for container registry %s", serverURL)
	}

	return *rt.RefreshToken, nil
}

// parseRegistryName parses a serverURL and returns the registry name (i.e. minus transport scheme)
func getRegistryURL(serverURL string) (*url.URL, error) {
	sURL, err := url.Parse(secureScheme + serverURL)
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to parse server URL - %w", err)
	}

	return sURL, nil
}

/*
Copyright 2026 OSS Container Tools

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

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// the audience azure requires on a federated credential assertion
const federatedAudience = "api://AzureADTokenExchange"

// startMSIStub stands in for the token endpoint azure provides on managed identity
// hosts. Only that endpoint is faked, the token it hands out is real, so the
// registry exchange behind it is too. MSI_ENDPOINT with MSI_SECRET puts adal in app
// service mode, a GET carrying the secret as a header.
func startMSIStub(t *testing.T, tenantID, clientID, clientSecret string) string {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resource := r.URL.Query().Get("resource")
		token, expiresOn, err := mintAADToken(tenantID, clientID, clientSecret, resource)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"access_token":%q,"expires_on":%q,"resource":%q,"token_type":"Bearer"}`,
			token, expiresOn, resource)
	}))
	t.Cleanup(srv.Close)

	return srv.URL
}

// mintAADToken uses the v1 endpoint, as that is resource scoped like the tokens
// adal asks the MSI endpoint for.
func mintAADToken(tenantID, clientID, clientSecret, resource string) (string, string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.PostForm("https://login.microsoftonline.com/"+tenantID+"/oauth2/token", url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"resource":      {resource},
	})
	if err != nil {
		return "", "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var body struct {
		AccessToken      string `json:"access_token"`
		ExpiresOn        string `json:"expires_on"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", "", err
	}
	if body.Error != "" {
		return "", "", fmt.Errorf("%s: %s", body.Error, body.ErrorDescription)
	}
	if body.AccessToken == "" {
		return "", "", fmt.Errorf("no access token in response for resource %q", resource)
	}

	return body.AccessToken, body.ExpiresOn, nil
}

// mintFederatedJWT signs an assertion with the key the issuer publishes in its
// JWKS. Azure fetches that JWKS to validate it, so nothing here is stubbed, this is
// the exchange a workload identity pod performs.
func mintFederatedJWT(t *testing.T, issuer, subject, keyPath, keyID string) string {
	t.Helper()

	pem, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("cannot read %s: %v", keyPath, err)
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM(pem)
	if err != nil {
		t.Fatalf("cannot parse %s: %v", keyPath, err)
	}

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		Issuer:    issuer,
		Subject:   subject,
		Audience:  jwt.ClaimStrings{federatedAudience},
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
	})
	token.Header["kid"] = keyID

	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("cannot sign assertion: %v", err)
	}

	return signed
}

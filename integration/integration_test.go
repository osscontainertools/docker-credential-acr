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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

const (
	// docker resolves a credHelpers entry to docker-credential-<value>, so the value
	// in config.json and the name of the binary have to agree
	credHelper = "acr"
	helperName = "docker-credential-" + credHelper
)

var servicePrincipalEnv = []string{"AZURE_CLIENT_ID", "AZURE_CLIENT_SECRET", "AZURE_TENANT_ID", "ACR_TEST_REGISTRY"}

// helperDir holds the binary under test, built once for the whole package
var helperDir string

func TestMain(m *testing.M) {
	_ = os.Setenv("FF_DOCKER_ACR_AZIDENTITY", "true")
	_ = os.Setenv("FF_DOCKER_ACR_REGISTRY_SCOPED_TOKEN", "true")

	dir, err := os.MkdirTemp("", "docker-credential-acr")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cmd := exec.Command("go", "build", "-o", filepath.Join(dir, helperName), ".")
	cmd.Dir = ".."
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building %s: %v\n%s", helperName, err, out)
		_ = os.RemoveAll(dir)
		os.Exit(1)
	}
	helperDir = dir

	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func TestGet(t *testing.T) {
	requireEnv(t, servicePrincipalEnv...)

	cmd := exec.Command(filepath.Join(helperDir, helperName), "get")
	cmd.Stdin = strings.NewReader(os.Getenv("ACR_TEST_REGISTRY"))
	out := runCommand(cmd, t)

	var creds struct {
		Username string
		Secret   string
	}
	if err := json.Unmarshal(out, &creds); err != nil {
		t.Fatalf("stdout is not credential helper JSON: %q: %v", out, err)
	}
	if creds.Username != "<token>" {
		t.Errorf("Username = %q, want <token>", creds.Username)
	}
	if creds.Secret == "" {
		t.Error("Secret is empty")
	}
}

func TestPullWithClientSecret(t *testing.T) {
	requireEnv(t, servicePrincipalEnv...)

	pullThroughHelper(t)
}

// TestPullWithFederatedToken needs a federated identity credential on the app
// registration naming ACR_TEST_OIDC_ISSUER and ACR_TEST_OIDC_SUBJECT, whose JWKS
// publishes ACR_TEST_OIDC_KID for the key in ACR_TEST_OIDC_KEY.
func TestPullWithFederatedToken(t *testing.T) {
	requireEnv(t, "AZURE_CLIENT_ID", "AZURE_TENANT_ID", "ACR_TEST_REGISTRY",
		"ACR_TEST_OIDC_ISSUER", "ACR_TEST_OIDC_SUBJECT", "ACR_TEST_OIDC_KEY", "ACR_TEST_OIDC_KID")

	assertion := mintFederatedJWT(t,
		os.Getenv("ACR_TEST_OIDC_ISSUER"),
		os.Getenv("ACR_TEST_OIDC_SUBJECT"),
		os.Getenv("ACR_TEST_OIDC_KEY"),
		os.Getenv("ACR_TEST_OIDC_KID"))

	tokenFile := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenFile, []byte(assertion), 0o600); err != nil {
		t.Fatal(err)
	}

	// an empty value reads as absent, go-autorest only records env values it finds set
	t.Setenv("AZURE_CLIENT_SECRET", "")
	t.Setenv("AZURE_FEDERATED_TOKEN_FILE", tokenFile)

	pullThroughHelper(t)
}

// TestPullWithMSI runs against a stub endpoint, see startMSIStub for what that does
// and does not fake.
func TestPullWithMSI(t *testing.T) {
	requireEnv(t, servicePrincipalEnv...)

	endpoint := startMSIStub(t,
		os.Getenv("AZURE_TENANT_ID"),
		os.Getenv("AZURE_CLIENT_ID"),
		os.Getenv("AZURE_CLIENT_SECRET"))

	// an empty value reads as absent, go-autorest only records env values it finds set
	t.Setenv("AZURE_CLIENT_SECRET", "")
	t.Setenv("MSI_ENDPOINT", endpoint)
	t.Setenv("MSI_SECRET", "stub")

	pullThroughHelper(t)
}

// pullThroughHelper pulls through the full chain a docker client takes, a
// config.json credHelpers entry and a PATH lookup for the binary. Which credential
// route the helper takes is left to the environment the caller sets up. The
// identity needs AcrPull on ACR_TEST_REGISTRY.
func pullThroughHelper(t *testing.T) {
	t.Helper()

	registry := os.Getenv("ACR_TEST_REGISTRY")
	image := os.Getenv("ACR_TEST_IMAGE")
	if image == "" {
		image = "hello:test"
	}

	dockerConfig := t.TempDir()
	config := fmt.Sprintf(`{"credHelpers":{%q:%q}}`, registry, credHelper)
	if err := os.WriteFile(filepath.Join(dockerConfig, "config.json"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKER_CONFIG", dockerConfig)
	t.Setenv("PATH", helperDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ref, err := name.ParseReference(registry + "/" + image)
	if err != nil {
		t.Fatalf("bad reference %q: %v", registry+"/"+image, err)
	}

	// a freshly created role assignment takes minutes to propagate
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	for {
		_, err := remote.Head(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithContext(ctx))
		if err == nil {
			t.Logf("pulled %s successfully", ref)
			return
		}

		// only a rejected token is worth waiting on, anything else stays broken
		var rerr *transport.Error
		if !errors.As(err, &rerr) || (rerr.StatusCode != http.StatusUnauthorized && rerr.StatusCode != http.StatusForbidden) {
			t.Fatalf("could not pull %s: %v", ref, err)
		}

		t.Logf("pull unauthorized, retrying while the role assignment propagates: %v", err)
		select {
		case <-ctx.Done():
			t.Fatalf("could not pull %s within timeout: %v", ref, err)
		case <-time.After(10 * time.Second):
		}
	}
}

// requireEnv fails rather than skips, as running this package at all already means
// the live tests were asked for.
func requireEnv(t *testing.T, keys ...string) {
	t.Helper()

	var missing []string
	for _, k := range keys {
		if os.Getenv(k) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("required environment variables not set: %s", strings.Join(missing, ", "))
	}
}

// runCommand returns stdout only, so a helper writing diagnostics to stderr does
// not look like protocol output.
func runCommand(cmd *exec.Cmd, t *testing.T) []byte {
	t.Helper()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Log(cmd.Args)
		t.Log(stderr.String())
		t.Log(string(out))
		t.Fatal(err)
	}
	return out
}

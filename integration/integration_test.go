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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// docker resolves a credHelpers entry to docker-credential-<value>, so the binary
// has to carry this exact name for the lookup in TestPull to find it
const helperName = "docker-credential-acr"

// TestGet drives the built binary over the credential helper protocol: the
// registry goes in on stdin, credentials come back on stdout as JSON.
func TestGet(t *testing.T) {
	registry := requireEnv(t)

	cmd := exec.Command(filepath.Join(buildHelper(t), helperName), "get")
	cmd.Stdin = strings.NewReader(registry)
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

// TestPull pulls from a real registry through the full chain a docker client
// takes: a config.json credHelpers entry, a PATH lookup for the helper binary,
// then the protocol. The service principal needs AcrPull on ACR_TEST_REGISTRY.
func TestPull(t *testing.T) {
	registry := requireEnv(t)
	image := os.Getenv("ACR_TEST_IMAGE")
	if image == "" {
		image = "hello:test"
	}

	dockerConfig := t.TempDir()
	config := fmt.Sprintf(`{"credHelpers":{%q:%q}}`, registry, strings.TrimPrefix(helperName, "docker-credential-"))
	if err := os.WriteFile(filepath.Join(dockerConfig, "config.json"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKER_CONFIG", dockerConfig)
	t.Setenv("PATH", buildHelper(t)+string(os.PathListSeparator)+os.Getenv("PATH"))

	ref, err := name.ParseReference(registry + "/" + image)
	if err != nil {
		t.Fatalf("bad reference %q: %v", registry+"/"+image, err)
	}

	// a freshly created role assignment takes minutes to propagate
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	var lastErr error
	for {
		_, lastErr = remote.Head(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithContext(ctx))
		if lastErr == nil {
			t.Logf("pulled %s successfully", ref)
			return
		}
		t.Logf("pull failed, retrying while the role assignment propagates: %v", lastErr)
		select {
		case <-ctx.Done():
			t.Fatalf("could not pull %s within timeout: %v", ref, lastErr)
		case <-time.After(10 * time.Second):
		}
	}
}

// requireEnv fails rather than skips, as the integration build tag already means
// this run was asked for. It returns ACR_TEST_REGISTRY.
func requireEnv(t *testing.T) string {
	t.Helper()

	var missing []string
	for _, k := range []string{"AZURE_CLIENT_ID", "AZURE_CLIENT_SECRET", "AZURE_TENANT_ID", "ACR_TEST_REGISTRY"} {
		if os.Getenv(k) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("required environment variables not set: %s", strings.Join(missing, ", "))
	}
	return os.Getenv("ACR_TEST_REGISTRY")
}

// buildHelper builds the binary under test and returns the directory holding it.
func buildHelper(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	cmd := exec.Command("go", "build", "-o", filepath.Join(dir, helperName), ".")
	cmd.Dir = ".."
	runCommand(cmd, t)
	return dir
}

// runCommand returns stdout only, so a helper writing diagnostics to stderr does
// not look like protocol output. On failure it reports both streams.
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

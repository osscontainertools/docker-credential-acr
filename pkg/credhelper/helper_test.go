/*
Copyright © 2022 Chris Mellard chris.mellard@icloud.com

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
package credhelper

import "testing"

// TestIsACRHelper tests whether URLs are detected as being ACR/MCR registry
// hosts, to determine whether the cred helper should fail fast.
func TestIsACRHelper(t *testing.T) {
	for _, c := range []struct {
		url  string
		want bool
	}{
		{"myregistry.azurecr.io", true},
		{"myregistry.azurecr.cn", true},
		{"myregistry.azurecr.de", true},
		{"myregistry.azurecr.us", true},
		{"mcr.microsoft.com", true},
		{"myregistry.westeurope.data.azurecr.io", true}, // dedicated data endpoint
		{"myregistry.azurecr.me", false},                // not a known tld
		{"evil.azurecr.io.attacker.com", false},         // suffix, not prefix
		{"myregistry.azurecr.io.example.org", false},
		{"azurecr.io", false},
		{"notacr.xcr.example", false},
		{"127.0.0.1:12345", false},
		{"localhost:12345", false},
		{"notaurl-)(*$@)(*@)(*", false},
		{"myregistry.azurecr.io:443", true},
		{"MyRegistry.azurecr.io", true},
		{"myregistry.AZURECR.IO", false},
		{"[::1%25.azurecr.io]", false},
		{"[2001:db8::1%25x.azurecr.io]:443", false},
		{"foo%C3%A9.azurecr.io", false},
		{"user@myregistry.azurecr.io", false},
		{"myregistry.azurecr.io/path", false},
		{"myregistry.azurecr.io?q", false},
		{"myregistry.azurecr.io#f", false},
	} {
		t.Run(c.url, func(t *testing.T) {
			_, got := parseACRRegistry(c.url, true)
			if got != c.want {
				t.Fatalf("got %t, want %t", got, c.want)
			}
		})
	}
}

func TestParseACRRegistryPassesParsedHost(t *testing.T) {
	for _, c := range []struct {
		url  string
		want string
	}{
		{"myregistry.azurecr.io", "myregistry.azurecr.io"},
		{"myregistry.azurecr.io:443", "myregistry.azurecr.io:443"},
		{"user@myregistry.azurecr.io/path?q#f", "myregistry.azurecr.io"},
	} {
		t.Run(c.url, func(t *testing.T) {
			got, ok := parseACRRegistry(c.url, false)
			if !ok || got != c.want {
				t.Fatalf("got %q %t, want %q", got, ok, c.want)
			}
		})
	}
}

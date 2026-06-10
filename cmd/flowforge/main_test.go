package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flowforge/internal/api"
	"flowforge/internal/secrets"
	"flowforge/internal/store"
)

func TestApplyUpdatesExistingResource(t *testing.T) {
	var methods []string
	useResourceTransport(t, func(r *http.Request) *http.Response {
		methods = append(methods, r.Method+" "+r.URL.Path)
		switch len(methods) {
		case 1:
			return testResponse(http.StatusConflict, `{"error":"workflow already exists"}`)
		case 2:
			return testResponse(http.StatusOK, "")
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
			return nil
		}
	})

	manifest := writeManifest(t, `
apiVersion: flowforge/v1alpha1
kind: Workflow
metadata:
  name: notify
spec:
  trigger:
    type: message.received
    as: message
`)
	var stdout strings.Builder
	if err := run([]string{"apply", "-f", manifest, "--server", "http://flowforge.test"}, &stdout, io.Discard); err != nil {
		t.Fatalf("run(apply) error = %v", err)
	}
	wantMethods := "POST /v1/workflows,PUT /v1/workflows/notify"
	if got := strings.Join(methods, ","); got != wantMethods {
		t.Fatalf("requests = %q, want %q", got, wantMethods)
	}
	if got := stdout.String(); got != "Workflow/notify applied\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestGetWorkflowByNameAsJSON(t *testing.T) {
	useResourceTransport(t, func(r *http.Request) *http.Response {
		if got, want := r.Method+" "+r.URL.Path, "GET /v1/workflows/notify"; got != want {
			t.Fatalf("request = %q, want %q", got, want)
		}
		return testResponse(http.StatusOK, `{"kind":"Workflow","metadata":{"name":"notify"}}`)
	})

	var stdout strings.Builder
	err := run(
		[]string{"get", "workflow", "notify", "--server=http://flowforge.test"},
		&stdout,
		io.Discard,
	)
	if err != nil {
		t.Fatalf("run(get workflow) error = %v", err)
	}
	if !strings.Contains(stdout.String(), `"name": "notify"`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestGetSecretsAsYAML(t *testing.T) {
	useResourceTransport(t, func(r *http.Request) *http.Response {
		if got, want := r.Method+" "+r.URL.Path, "GET /v1/secrets"; got != want {
			t.Fatalf("request = %q, want %q", got, want)
		}
		return testResponse(http.StatusOK, `{"secrets":[{"kind":"Secret","metadata":{"name":"telegram-proxy"},"dataKeys":["url"]}]}`)
	})

	var stdout strings.Builder
	err := run(
		[]string{"get", "secrets", "-o", "yaml", "--server", "http://flowforge.test"},
		&stdout,
		io.Discard,
	)
	if err != nil {
		t.Fatalf("run(get secrets) error = %v", err)
	}
	if !strings.Contains(stdout.String(), "name: telegram-proxy") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestGetRejectsUnsupportedOutput(t *testing.T) {
	err := run([]string{"get", "workflows", "-o", "table"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "expected json or yaml") {
		t.Fatalf("run(get workflows -o table) error = %v", err)
	}
}

func TestDeleteResourceByKindAndName(t *testing.T) {
	useResourceTransport(t, func(r *http.Request) *http.Response {
		if got, want := r.Method+" "+r.URL.Path, "DELETE /v1/secrets/telegram-proxy"; got != want {
			t.Fatalf("request = %q, want %q", got, want)
		}
		return testResponse(http.StatusNoContent, "")
	})

	var stdout strings.Builder
	err := run([]string{"delete", "secret", "telegram-proxy", "--server=http://flowforge.test"}, &stdout, io.Discard)
	if err != nil {
		t.Fatalf("run(delete) error = %v", err)
	}
	if got := stdout.String(); got != "Secret/telegram-proxy deleted\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestDeleteResourceByManifest(t *testing.T) {
	useResourceTransport(t, func(r *http.Request) *http.Response {
		if got, want := r.Method+" "+r.URL.Path, "DELETE /v1/workflows/notify"; got != want {
			t.Fatalf("request = %q, want %q", got, want)
		}
		return testResponse(http.StatusNoContent, "")
	})

	manifest := writeManifest(t, `
kind: Workflow
metadata:
  name: notify
`)
	if err := run([]string{"delete", "-f", manifest, "--server", "http://flowforge.test"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("run(delete -f) error = %v", err)
	}
}

func TestDeleteResourceByKindAndManifest(t *testing.T) {
	useResourceTransport(t, func(r *http.Request) *http.Response {
		if got, want := r.Method+" "+r.URL.Path, "DELETE /v1/secrets/telegram-proxy"; got != want {
			t.Fatalf("request = %q, want %q", got, want)
		}
		return testResponse(http.StatusNoContent, "")
	})

	manifest := writeManifest(t, `
kind: Secret
metadata:
  name: telegram-proxy
type: Opaque
stringData:
  url: socks5://127.0.0.1:9050
`)
	if err := run(
		[]string{"delete", "secret", manifest, "--server", "http://flowforge.test"},
		io.Discard,
		io.Discard,
	); err != nil {
		t.Fatalf("run(delete secret manifest) error = %v", err)
	}
}

func TestDeleteResourceRejectsManifestKindMismatch(t *testing.T) {
	manifest := writeManifest(t, `
kind: Workflow
metadata:
  name: notify
`)
	err := run([]string{"delete", "secret", manifest}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("run(delete secret workflow.yaml) error = %v", err)
	}
}

func TestApplyReplacesImmutableSecret(t *testing.T) {
	var requests []string
	useResourceTransport(t, func(r *http.Request) *http.Response {
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		switch r.Method {
		case http.MethodPost:
			return testResponse(http.StatusConflict, `{"error":"secret already exists"}`)
		case http.MethodPut:
			if r.URL.Query().Get("replace") == "true" {
				return testResponse(http.StatusOK, `{"metadata":{"name":"telegram-proxy"}}`)
			}
			return testResponse(http.StatusBadRequest, `{"error":"secret is immutable"}`)
		default:
			t.Fatalf("unexpected method %s", r.Method)
			return nil
		}
	})

	manifest := writeManifest(t, `
kind: Secret
metadata:
  name: telegram-proxy
type: Opaque
immutable: true
stringData:
  url: socks5://127.0.0.1:9050
`)
	err := run(
		[]string{"apply", "-f", manifest, "--server", "http://flowforge.test"},
		io.Discard,
		io.Discard,
	)
	if err != nil {
		t.Fatalf("run(apply immutable secret) error = %v", err)
	}
	want := "POST /v1/secrets,PUT /v1/secrets/telegram-proxy,PUT /v1/secrets/telegram-proxy?replace=true"
	if got := strings.Join(requests, ","); got != want {
		t.Fatalf("requests = %q, want %q", got, want)
	}
}

func TestApplyImmutableSecretEndToEnd(t *testing.T) {
	key := make([]byte, 32)
	key[0] = 9
	secretStore := secrets.NewEncryptedMemoryStore(
		secrets.NewEnvelopeCipher(secrets.StaticKeyProvider{Key: key}),
	)
	handler := api.NewServer(secretStore, store.NewMemoryWorkflowStore(), nil).Handler()
	useHandlerTransport(t, handler)

	manifest := writeManifest(t, `
kind: Secret
metadata:
  name: telegram-proxy
type: Opaque
immutable: true
stringData:
  url: socks5://127.0.0.1:10808
`)
	if err := run(
		[]string{"apply", "-f", manifest, "--server", "http://flowforge.test"},
		io.Discard,
		io.Discard,
	); err != nil {
		t.Fatalf("first apply error = %v", err)
	}
	if err := os.WriteFile(manifest, []byte(`
kind: Secret
metadata:
  name: telegram-proxy
type: Opaque
immutable: true
stringData:
  url: socks5://127.0.0.1:9050
`), 0600); err != nil {
		t.Fatalf("update manifest: %v", err)
	}
	if err := run(
		[]string{"apply", "-f", manifest, "--server", "http://flowforge.test"},
		io.Discard,
		io.Discard,
	); err != nil {
		t.Fatalf("second apply error = %v", err)
	}
	value, err := secretStore.Resolve(
		context.Background(),
		secrets.SecretRef{Name: "telegram-proxy", Key: "url"},
	)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got, want := string(value), "socks5://127.0.0.1:9050"; got != want {
		t.Fatalf("resolved proxy = %q, want %q", got, want)
	}
}

func TestCompletion(t *testing.T) {
	for _, test := range []struct {
		shell string
		want  string
	}{
		{shell: "fish", want: "complete -c flowforge"},
		{shell: "bash", want: "complete -F _flowforge_completion flowforge"},
		{shell: "zsh", want: "compdef _flowforge flowforge"},
	} {
		t.Run(test.shell, func(t *testing.T) {
			var stdout strings.Builder
			if err := run([]string{"completion", test.shell}, &stdout, io.Discard); err != nil {
				t.Fatalf("run(completion %s) error = %v", test.shell, err)
			}
			if !strings.Contains(stdout.String(), test.want) {
				t.Fatalf("completion output does not contain %q", test.want)
			}
		})
	}
}

type roundTripFunc func(*http.Request) *http.Response

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request), nil
}

func useResourceTransport(t *testing.T, transport roundTripFunc) {
	t.Helper()
	previous := resourceHTTPClient
	resourceHTTPClient = &http.Client{Transport: transport}
	t.Cleanup(func() {
		resourceHTTPClient = previous
	})
}

func useHandlerTransport(t *testing.T, handler http.Handler) {
	t.Helper()
	useResourceTransport(t, func(request *http.Request) *http.Response {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder.Result()
	})
}

func testResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func writeManifest(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "resource.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(contents)), 0600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}

func TestDeleteRequiresResource(t *testing.T) {
	err := run([]string{"delete"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "requires kind and name") {
		t.Fatalf("run(delete) error = %v", err)
	}
}

func TestManifestRequiresResourceName(t *testing.T) {
	manifest := writeManifest(t, `
kind: Workflow
spec:
  trigger:
    type: event
    as: event
`)
	err := run([]string{"apply", "-f", manifest}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "metadata.name is required") {
		t.Fatalf("run(apply) error = %v", err)
	}
}

func TestInstallFishCompletion(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)
	var stdout strings.Builder
	if err := run([]string{"completion", "install", "fish"}, &stdout, io.Discard); err != nil {
		t.Fatalf("run(completion install fish) error = %v", err)
	}
	path := filepath.Join(configDir, "fish", "completions", "flowforge.fish")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	if !strings.Contains(string(data), "__flowforge_resource_names") {
		t.Fatalf("installed completion is missing resource-name completion")
	}
	loaderPath := filepath.Join(configDir, "fish", "conf.d", "flowforge-completion.fish")
	loader, err := os.ReadFile(loaderPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", loaderPath, err)
	}
	if !strings.Contains(string(loader), path) {
		t.Fatalf("completion loader does not source %s: %s", path, loader)
	}
}

func Example_usage() {
	usage(os.Stdout)
	fmt.Print("")
	// Output:
	// usage:
	//   flowforge apply -f <manifest.yaml> [--server http://127.0.0.1:8080]
	//   flowforge get <kind> [name] [-o json|yaml] [--server http://127.0.0.1:8080]
	//   flowforge delete <kind> <name> [--server http://127.0.0.1:8080]
	//   flowforge delete <kind> <manifest.yaml> [--server http://127.0.0.1:8080]
	//   flowforge delete -f <manifest.yaml> [--server http://127.0.0.1:8080]
	//   flowforge completion <fish|bash|zsh>
	//   flowforge completion install fish
}

package song

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderDiagnosticRedactsSecrets(t *testing.T) {
	raw := `{"error":{"code":4,"message":"quota exceeded; api_key=hidden123 Bearer token456 https://host/audio?token=secret sk-longsecretvalue1234567890 env-password","hint":"check quota"}}`
	d := providerDiagnostic([]byte(raw), map[string]string{"SERVICE_PASSWORD": "env-password"})
	for _, secret := range []string{"hidden123", "token456", "https://host", "sk-longsecretvalue1234567890", "env-password"} {
		if strings.Contains(d.Message, secret) {
			t.Fatal("leaked", secret)
		}
	}
	if d.Code != 4 || !strings.Contains(d.Message, "quota exceeded") {
		t.Fatal(d)
	}
	d = providerDiagnostic([]byte("opaque-secret-provider-output"), nil)
	if strings.Contains(d.Message, "opaque-secret") {
		t.Fatal(d)
	}
}
func TestBoundedCapture(t *testing.T) {
	var b diagnosticBuffer
	payload := strings.Repeat("x", 100000)
	if n, e := b.Write([]byte(payload)); n != len(payload) || e != nil {
		t.Fatal(n, e)
	}
	if len(b.data) != 65536 {
		t.Fatal(len(b.data))
	}
}

func TestGenerateRetainsRedactedProviderFailure(t *testing.T) {
	root, env := fakeEnvironment(t)
	script := `#!/bin/sh
printf '%s' '{"error":{"code":1,"message":"Music API unavailable (HTTP 410); api_key=private-test-key"}}' >&2
exit 1
`
	if e := os.WriteFile(filepath.Join(env["PATH"], "mmx"), []byte(script), 0755); e != nil {
		t.Fatal(e)
	}
	e := Generate(context.Background(), root, 1, "diagnostic", env, &bytes.Buffer{})
	if e == nil || !strings.Contains(e.Error(), "HTTP 410") || strings.Contains(e.Error(), "private-test-key") {
		t.Fatal(e)
	}
	files, _ := filepath.Glob(filepath.Join(root, Store, "candidates", "*", "provider-error.json"))
	if len(files) != 1 {
		t.Fatal(files)
	}
	body, e := os.ReadFile(files[0])
	if e != nil || !bytes.Contains(body, []byte("HTTP 410")) || bytes.Contains(body, []byte("private-test-key")) {
		t.Fatal(string(body), e)
	}
}

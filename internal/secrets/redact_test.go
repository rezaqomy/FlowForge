package secrets

import "testing"

func TestRedactorRedactsRegisteredValues(t *testing.T) {
	redactor := NewRedactor([]byte("secret-token"))

	if got := redactor.String("secret-token"); got != Redacted {
		t.Fatalf("String() = %q, want %q", got, Redacted)
	}
	if got := redactor.String("public"); got != "public" {
		t.Fatalf("String() = %q, want public", got)
	}
}

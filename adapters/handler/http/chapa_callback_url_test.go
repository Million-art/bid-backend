package http

import "testing"

func TestChapaPublicCallbackURL(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"http://localhost:8080/api/webhooks/chapa", false},
		{"https://127.0.0.1/api/webhooks/chapa", false},
		{"https://api.localhost/api/webhooks/chapa", false},
		{"https://clientbackend.carramarket.com/api/webhooks/chapa", true},
		{"https://carramarket.com/api/webhooks/chapa", true},
	}
	for _, tc := range cases {
		if got := chapaPublicCallbackURL(tc.in); got != tc.want {
			t.Fatalf("chapaPublicCallbackURL(%q)=%v want %v", tc.in, got, tc.want)
		}
	}
}

func TestResolveChapaCallbackURL(t *testing.T) {
	prod := "https://clientbackend.carramarket.com/api/webhooks/chapa"

	if got := resolveChapaCallbackURL(prod, "http", "localhost:8080"); got != prod {
		t.Fatalf("configured CALLBACK_URL must win over localhost host, got %q", got)
	}
	if got := resolveChapaCallbackURL("", "https", "clientbackend.carramarket.com"); got != prod {
		t.Fatalf("public request host should derive callback, got %q", got)
	}
	if got := resolveChapaCallbackURL("", "http", "localhost:8080"); got != "" {
		t.Fatalf("localhost must not be sent to Chapa, got %q", got)
	}
	if got := resolveChapaCallbackURL("http://localhost:8080/api/webhooks/chapa", "https", "clientbackend.carramarket.com"); got != prod {
		t.Fatalf("invalid CALLBACK_URL should fall back to public host, got %q", got)
	}
}

package discovery

import "testing"

func TestParseWatchSecrets(t *testing.T) {
	refs, err := ParseWatchSecrets("minikube", "workload-ns", "db-secret,shared/api-token#password")
	if err != nil {
		t.Fatalf("ParseWatchSecrets returned error: %v", err)
	}

	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}

	if refs[0].Cluster != "minikube" ||
		refs[0].Namespace != "workload-ns" ||
		refs[0].Name != "db-secret" ||
		refs[0].Key != "" {
		t.Fatalf("unexpected first ref: %#v", refs[0])
	}

	if refs[1].Cluster != "minikube" ||
		refs[1].Namespace != "shared" ||
		refs[1].Name != "api-token" ||
		refs[1].Key != "password" {
		t.Fatalf("unexpected second ref: %#v", refs[1])
	}
}

func TestParseWatchSecretsRejectsInvalidReference(t *testing.T) {
	_, err := ParseWatchSecrets("minikube", "default", "valid-secret,")
	if err == nil {
		t.Fatal("expected invalid trailing secret reference to fail")
	}
}

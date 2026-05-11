package sandboxproviders

import (
	"slices"
	"testing"

	"csgclaw/internal/config"
)

func TestSupportedProvidersAlwaysIncludeBoxLite(t *testing.T) {
	supported := SupportedProviders()
	if !slices.Contains(supported, config.BoxLiteProvider) {
		t.Fatalf("SupportedProviders() = %v, want %q to be compiled in", supported, config.BoxLiteProvider)
	}
	if len(supported) != 3 {
		t.Fatalf("SupportedProviders() = %v, want exactly the compiled providers", supported)
	}
}

func TestSupportedProvidersIncludeCSGHubWithoutBuildTag(t *testing.T) {
	supported := SupportedProviders()
	if !slices.Contains(supported, config.CSGHubProvider) {
		t.Fatalf("SupportedProviders() = %v, want %q to be compiled in", supported, config.CSGHubProvider)
	}
}

func TestSupportedProvidersIncludeDocker(t *testing.T) {
	supported := SupportedProviders()
	if !slices.Contains(supported, config.DockerProvider) {
		t.Fatalf("SupportedProviders() = %v, want %q to be compiled in", supported, config.DockerProvider)
	}
}

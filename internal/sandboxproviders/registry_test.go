package sandboxproviders

import (
	"slices"
	"testing"

	"csgclaw/internal/config"
)

func TestSupportedProvidersAlwaysIncludeBoxLiteCLI(t *testing.T) {
	supported := SupportedProviders()
	if !slices.Contains(supported, config.BoxLiteCLIProvider) {
		t.Fatalf("SupportedProviders() = %v, want %q to be compiled in", supported, config.BoxLiteCLIProvider)
	}
	if slices.Contains(supported, "boxlite-sdk") {
		t.Fatalf("SupportedProviders() = %v, want legacy %q to stay removed", supported, "boxlite-sdk")
	}
}

func TestSupportedProvidersIncludeCSGHubWithoutBuildTag(t *testing.T) {
	supported := SupportedProviders()
	if !slices.Contains(supported, config.CSGHubProvider) {
		t.Fatalf("SupportedProviders() = %v, want %q to be compiled in", supported, config.CSGHubProvider)
	}
}

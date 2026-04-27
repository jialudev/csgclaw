package sandboxproviders

import (
	"reflect"
	"testing"
	"unsafe"

	"csgclaw/internal/agent"
	"csgclaw/internal/config"
	"csgclaw/internal/sandbox"
	"csgclaw/internal/sandbox/boxlitecli"
)

func TestBoxLiteCLIProviderFactoryUsesConfiguredPath(t *testing.T) {
	factory, ok := factories[config.BoxLiteCLIProvider]
	if !ok {
		t.Fatalf("boxlite-cli provider factory not registered")
	}

	opt, err := factory(config.SandboxConfig{
		Provider:         config.BoxLiteCLIProvider,
		BoxLiteCLIPath:   "/opt/boxlite/bin/boxlite",
		DebianRegistries: []string{"registry.a"},
	})
	if err != nil {
		t.Fatalf("factory() error = %v", err)
	}

	provider := sandboxProviderFromOption(t, opt)
	boxliteProvider, ok := provider.(boxlitecli.Provider)
	if !ok {
		t.Fatalf("provider = %T, want boxlitecli.Provider", provider)
	}
	if got, want := providerPath(t, boxliteProvider), "/opt/boxlite/bin/boxlite"; got != want {
		t.Fatalf("provider path = %q, want %q", got, want)
	}
	if got, want := provider.Name(), config.BoxLiteCLIProvider; got != want {
		t.Fatalf("provider.Name() = %q, want %q", got, want)
	}
}

func sandboxProviderFromOption(t *testing.T, opt agent.ServiceOption) sandbox.Provider {
	t.Helper()
	svc := &agent.Service{}
	if err := opt(svc); err != nil {
		t.Fatalf("ServiceOption() error = %v", err)
	}
	field := reflect.ValueOf(svc).Elem().FieldByName("sandbox")
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface().(sandbox.Provider)
}

func providerPath(t *testing.T, provider boxlitecli.Provider) string {
	t.Helper()
	value := reflect.ValueOf(&provider).Elem().FieldByName("path")
	return reflect.NewAt(value.Type(), unsafe.Pointer(value.UnsafeAddr())).Elem().String()
}

package hub

import (
	"errors"
	"fmt"
	"strings"

	"csgclaw/internal/config"
)

var (
	ErrRegistryKindUnsupported = errors.New("hub registry kind is not supported yet")
	ErrRegistryPathRequired    = errors.New("hub registry path is required")
)

func DefaultStoreFactory(cfg config.HubRegistryConfig) (Store, error) {
	switch normalizeRegistryKind(cfg.Kind) {
	case RegistryKindBuiltin:
		return NewBuiltinStore(), nil
	case RegistryKindLocal:
		path := strings.TrimSpace(cfg.Path)
		if path == "" {
			return nil, ErrRegistryPathRequired
		}
		return NewLocalStore(path), nil
	case RegistryKindRemote:
		return nil, fmt.Errorf("%w: %s", ErrRegistryKindUnsupported, cfg.Kind)
	default:
		return nil, fmt.Errorf("%w: %s", ErrRegistryKindUnsupported, cfg.Kind)
	}
}

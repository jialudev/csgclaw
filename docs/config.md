# CSGClaw Configuration

English | [中文](config.zh.md)

`csgclaw onboard` writes the local config file used by `csgclaw serve`. The config covers server access, model providers, bootstrap image selection, sandbox isolation, and optional channels.

## Server Address

`listen_addr` is the address that the local HTTP server binds to.

`advertise_base_url` is the base URL that CSGClaw gives to manager and worker boxes so they can call back into the local HTTP server. When it is set, CSGClaw uses it after trimming a trailing slash. When it is empty, CSGClaw falls back to an inferred local IPv4 address plus the configured listen port.

Use `advertise_base_url` when the automatically inferred address is not reachable from BoxLite boxes, such as when you need a LAN address, a tunnel URL, or a host alias.

`host.docker.internal` is a Docker Desktop-specific alias and is usually not resolvable from BoxLite boxes. If it appears in `advertise_base_url`, CSGClaw rewrites it to the inferred local IPv4 address when generating manager and worker box configs.

`access_token` protects authenticated API routes, including the PicoClaw bot routes. When authentication is enabled, clients must send `Authorization: Bearer <access_token>`.

`no_auth` controls whether CSGClaw skips the bearer-token check. The default is `false`. Set it to `true` only for trusted local or development environments.

String values in `config.toml` can reference environment variables with `${NAME}` or `$NAME`. CSGClaw expands them when loading the config and keeps the placeholder form when it later rewrites the same value. If an environment variable is not set, it expands to an empty string.

```toml
[server]
listen_addr = "0.0.0.0:${PORT}"
advertise_base_url = "http://${IP}:${PORT}"
access_token = "${ACCESS_TOKEN}"
no_auth = false
```

## Model Provider Examples

### Local CSGHub-lite

```toml
[server]
listen_addr = "0.0.0.0:18080"
advertise_base_url = "http://127.0.0.1:18080"
access_token = "your_access_token"
no_auth = false

[models]
default = "csghub-lite.Qwen/Qwen3-0.6B-GGUF"

[models.providers.csghub-lite]
base_url = "http://127.0.0.1:11435/v1"
api_key = "local"
models = ["Qwen/Qwen3-0.6B-GGUF"]

[bootstrap]
manager_image = "opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/picoclaw:2026.4.24.0"
agent_runtime = "picoclaw"

[sandbox]
provider = "boxlite-cli"
home_dir_name = "boxlite"
boxlite_cli_path = "boxlite"
debian_registries = ["harbor.opencsg.com", "docker.io"]
```

### Remote LLM API

```toml
[server]
listen_addr = "0.0.0.0:18080"
advertise_base_url = "http://127.0.0.1:18080"
access_token = "your_access_token"
no_auth = false

[models]
default = "remote.gpt-5.4"

[models.providers.remote]
base_url = "https://api.openai.com/v1"
api_key = "sk-your-api-key"
models = ["gpt-5.4"]

[bootstrap]
manager_image = "opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/picoclaw:2026.4.24.0"
agent_runtime = "picoclaw"

[sandbox]
provider = "boxlite-cli"
home_dir_name = "boxlite"
boxlite_cli_path = "boxlite"
debian_registries = ["harbor.opencsg.com", "docker.io"]
```

### Local Codex via CLIProxyAPI

```toml
[server]
listen_addr = "0.0.0.0:18080"
advertise_base_url = "http://127.0.0.1:18080"
access_token = "your_access_token"
no_auth = false

[models]
default = "codex.gpt-5.4"

[models.providers.codex]
base_url = "http://127.0.0.1:8317/v1"
api_key = "local"
models = ["gpt-5.4"]

[bootstrap]
manager_image = "opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/picoclaw:2026.4.24.0"
agent_runtime = "picoclaw"

[sandbox]
provider = "boxlite-cli"
home_dir_name = "boxlite"
boxlite_cli_path = "boxlite"
debian_registries = ["harbor.opencsg.com", "docker.io"]
```

## OpenClaw Runtime

CSGClaw defaults to PicoClaw. To run the bootstrap manager and created workers with OpenClaw instead, configure both the OpenClaw-capable image and `agent_runtime = "openclaw"`.

The recommended image shape is a slim OpenClaw base image with the CSGClaw channel plugin baked under `/home/node/openclaw-plugins/csgclaw-extension`. Runtime state still comes from `~/.csgclaw/agents/<agent>/.openclaw/openclaw.json`; do not mount an empty host directory over `/home/node/openclaw-plugins`, because that hides baked plugins.

```toml
[models]
default = "minimax.MiniMax-M2.7"

[models.providers.minimax]
base_url = "https://api.minimaxi.com/v1"
api_key = "${MINIMAX_API_KEY}"
models = ["MiniMax-M2.7"]

[bootstrap]
manager_image = "opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsg_public/openclaw:20260429.2-csgclaw"
agent_runtime = "openclaw"
```

`base_url` should be the OpenAI-compatible API root, such as `https://api.minimaxi.com/v1`. If a full `/chat/completions` URL is supplied, CSGClaw normalizes it back to the API root before forwarding requests.

## Sandbox Providers

CSGClaw runs Workers through the configured sandbox provider. The default build shape uses `boxlite-cli`, which runs BoxLite through the external CLI process. SDK-enabled builds still default to `boxlite-sdk`, which uses the vendored BoxLite Go SDK.

The default source build and official release bundles already align with the CLI-backed provider:

```toml
[sandbox]
provider = "boxlite-cli"
home_dir_name = "boxlite"
boxlite_cli_path = "boxlite"
debian_registries = ["harbor.opencsg.com", "docker.io"]
```

`boxlite_cli_path` is the executable path used only by `provider = "boxlite-cli"`. For official release bundles, the default value `boxlite` first resolves to the bundled sibling binary next to `csgclaw`, then falls back to `PATH` if that bundle is missing. Set an absolute path only when you need to override the bundled binary or point to a custom installation.

`debian_registries` controls where BoxLite pulls `debian:bookworm-slim`. If omitted or empty, CSGClaw defaults to `harbor.opencsg.com` then `docker.io`. Use `onboard` to persist a custom list:

```bash
csgclaw onboard --debian-registries "harbor.opencsg.com,docker.io"
```

CSGClaw passes an explicit `--home` to the BoxLite CLI for each agent, using the agent directory plus `home_dir_name` such as `~/.csgclaw/agents/<agent-id>/boxlite`. That explicit home takes precedence over `BOXLITE_HOME` for CSGClaw-managed sandboxes, while `BOXLITE_HOME` still applies when you run `boxlite` manually without `--home`.

The `boxlite-cli` provider does not need the vendored Go SDK at runtime. `boxlite-sdk` is the only sandbox provider that gets special build-time treatment because it pulls in CGO, the native SDK archive, and the larger embedded runtime payload. The repository now supports two build shapes:

- `make build`, `make test`, `make run`, `make onboard`, and `make package` use the default `boxlite-cli` build shape. The resulting binary excludes only the SDK-backed `boxlite-sdk` provider, while `boxlite-cli` and other non-SDK providers remain compiled in.
- `make build-with-boxlite-sdk`, `make test-with-boxlite-sdk`, `make run-with-boxlite-sdk`, and `make onboard-with-boxlite-sdk` add the `boxlite_sdk` build tag and keep the SDK-backed `boxlite-sdk` provider compiled in.

## Channel Configuration

Channel integration is optional. CSGClaw works with the built-in Web UI by default, and you only need channel config when you want to connect external messaging platforms such as Feishu.

Channel-specific settings live under top-level config sections such as `channels.feishu`. Keep the main config focused on shared server, model, bootstrap, and sandbox settings, then add only the channel blocks you actually use.

For detailed field definitions and examples, see [Feishu Channel Configuration](channel/feishu.md).

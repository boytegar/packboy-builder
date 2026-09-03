---
name: configuration-management
description: Configuration management patterns for Go: environment variables, config files, layered config, secrets handling, and hot-reload. Use when designing configuration strategy for an application.
origin: builtin
---

# Configuration Management

## When to use
- Designing config for a new Go application
- Choosing between env vars, config files, or flags
- Managing secrets (API keys, database passwords)
- Implementing layered/overridable config

## Configuration sources (priority high → low)
1. **Command-line flags**: explicit, per-invocation
2. **Environment variables**: deployment-specific, per-environment
3. **Config file**: project/user defaults
4. **Built-in defaults**: safe values, always present

## Environment variables
- Use for: deployment-specific values (DB host, port, log level)
- Use `ENV_VAR_NAME` convention (uppercase, underscore-separated)
- Use `os.Getenv` for simple cases; `envconfig` or `viper` for complex needs
- Parse with `strconv.Atoi`, `time.ParseDuration` for typed values
- Provide defaults: `getEnv("PORT", "8080")`

## Config files
- Use for: complex, nested, or large configurations
- Formats: YAML (human-friendly), JSON (ubiquitous), TOML (typed)
- Use `viper`, `koanf`, or `gopkg.in/yaml.v3` for parsing
- Support multiple locations: `./config.yaml`, `~/.app/config.yaml`, `/etc/app/config.yaml`
- Overlay/merge: later sources override earlier ones

## Secrets handling
- **Never commit secrets**: use `.env` files (gitignored), vault, or cloud secret managers
- **Environment for deployment**: `DATABASE_URL` as env var, not in config file
- **Use `.env.example`**: document required vars without real values
- **Rotate secrets**: don't bake long-lived secrets into images
- **Mask in logs**: redact secrets before logging config

## Layered config pattern
```go
type Config struct {
    DBHost     string `env:"DB_HOST"     yaml:"db_host"     default:"localhost"`
    DBPort     int    `env:"DB_PORT"     yaml:"db_port"     default:"5432"`
    LogLevel   string `env:"LOG_LEVEL"   yaml:"log_level"   default:"info"`
}
```
1. Start with defaults
2. Override with config file values
3. Override with environment variables
4. Override with command-line flags

## Hot-reload
- Watch config file for changes with `fsnotify`
- Apply changes via atomic pointer swap: `atomic.Pointer[Config]`
- Never mutate config in-place while readers use it
- Log config changes for auditability

## Validation
- Validate at startup, fail fast on invalid config
- Use struct tags + `validator` library or manual validation
- Check for required fields, valid ranges, reachable endpoints
- Don't proceed with invalid config — it'll fail at runtime with worse errors

## Best practices
- **12-factor app**: config in environment, not in code
- **Typed config**: parse strings to proper types (int, duration, URL)
- **Documented**: `.env.example` or `config.example.yaml`
- **No magic**: explicit over implicit — don't infer config from filesystem state
- **Testable**: use interfaces so config can be mocked in tests

## Common anti-patterns
- Hardcoded values in source code
- Config file in `~/.myapp/` with no fallback
- Secrets in version-controlled YAML
- No validation until first use
- Config changes require redeployment (use env vars for runtime changes)

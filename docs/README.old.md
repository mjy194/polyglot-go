# Polyglot

Universal AI API Gateway

## Features

- 🔌 Multiple AI Backend Support (UiPath, Anthropic, OpenAI)
- 🔄 Protocol Translation (Anthropic ↔ OpenAI ↔ Gemini)
- 🌊 Streaming Response Support
- 🎯 Adapter Plugin System
- 📊 Web Management Dashboard
- 🔒 Authentication & Authorization
- 📈 Metrics & Monitoring

## Quick Start

```bash
# Install dependencies
make deps

# Run
make run

# Or build and run
make build
./polyglot
```

## Configuration

Edit `configs/config.yaml`:

```yaml
server:
  host: "0.0.0.0"
  port: 3000

backend:
  provider: "uipath"
  uipath:
    org_name: "your-org"
    tenant_name: "DefaultTenant"
```

## Development

```bash
# Format code
make fmt

# Run tests
make test

# Lint
make lint
```

## Documentation

See [docs/](docs/) directory for detailed documentation.

## License

MIT

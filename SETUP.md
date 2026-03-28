# Semspec Setup for WitFoo Developers

## Quick Start (One Command)

```bash
./scripts/dev/semspec-setup.sh
```

This idempotent script handles everything: submodule init, Docker image pull, service startup, and MCP validation.

## Prerequisites

- Docker >= 24.0
- Docker Compose v2
- Git
- Anthropic API key (or Ollama for offline use)

## Port Reference

Semspec ports are remapped to avoid conflicts with the analytics stack:

| Service | Default | WitFoo | URL |
|---------|---------|--------|-----|
| Gateway | 8080 | **8880** | http://localhost:8880 |
| NATS | 4222 | **4922** | localhost:4922 |
| NATS Monitor | 8222 | **8922** | http://localhost:8922 |
| Sandbox | 8090 | **8890** | http://localhost:8890 |
| Semsource | 7890 | 7890 | (internal) |

Reserved port range: **8800-8999** for semspec.

## LLM Configuration

### Anthropic Claude API (Default)

Set your API key in `utils/semspec/.env`:

```bash
ANTHROPIC_API_KEY=sk-ant-your-key-here
```

Or export it in your shell:

```bash
export ANTHROPIC_API_KEY=sk-ant-your-key-here
```

### Ollama (Offline Alternative)

For disconnected network environments:

```bash
# Install Ollama and pull a model
ollama pull qwen2.5-coder:14b

# Update utils/semspec/.env
SEMSPEC_LLM_PROVIDER=ollama
OLLAMA_HOST=http://host.docker.internal:11434
```

## MCP Integration

Claude Code in VSCode auto-detects the MCP configuration from `.vscode/mcp.json`. When semspec is running, its knowledge graph tools will appear in Claude Code's tool list.

Validate connectivity:

```bash
./scripts/dev/semspec-mcp-test.sh
```

## Script Reference

| Script | Purpose |
|--------|---------|
| `scripts/dev/semspec-setup.sh` | First-time setup (idempotent) |
| `scripts/dev/semspec-launch.sh` | Start semspec services |
| `scripts/dev/semspec-stop.sh` | Stop semspec services |
| `scripts/dev/semspec-status.sh` | Check container status |
| `scripts/dev/semspec-update.sh` | Pull latest from fork |
| `scripts/dev/semspec-update.sh --upstream` | Also pull from C360Studio |
| `scripts/dev/semspec-mcp-test.sh` | Validate MCP connectivity |

## Troubleshooting

### Port Conflicts

If `semspec-launch.sh` reports port conflicts:

```bash
# Check what's using the port
ss -tlnp | grep :8880

# Force launch (skip conflict checks)
./scripts/dev/semspec-launch.sh --force
```

### Docker Memory

With 64GB+ RAM, no constraints needed. For smaller VMs, you can stop conductor services first:

```bash
./scripts/dev/dev-conductor.sh stop
./scripts/dev/semspec-launch.sh
```

### Submodule Not Initialized

```bash
git submodule update --init utils/semspec
```

### MCP Not Connecting

1. Ensure semspec is running: `./scripts/dev/semspec-status.sh`
2. Check gateway health: `curl http://localhost:8880/readyz`
3. Verify `.vscode/mcp.json` exists with correct URL
4. Restart Claude Code (reload VSCode window)

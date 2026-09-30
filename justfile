set shell := ["bash", "-euo", "pipefail", "-c"]

# match the host arch, overriding any global DOCKER_DEFAULT_PLATFORM.
# override for cross-arch builds: just DOCKER_DEFAULT_PLATFORM=linux/amd64 start
export DOCKER_DEFAULT_PLATFORM := "linux/" + if arch() == "aarch64" { "arm64" } else { "amd64" }

# force the docker-driver builder: a docker-container builder leaves freshly
# built images dangling, so compose silently runs the stale wechatopenai tag
export BUILDX_BUILDER := "default"

[private]
_default:
    @just --list --unsorted --list-heading '' --list-prefix='- '

# sync fork with upstream (Tencent/WeKnora) and rebase local commits
pull:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "→ fetching upstream"
    git fetch upstream
    echo "→ rebasing local commits onto upstream/main"
    git rebase --autostash upstream/main
    echo "✓ pull complete; publish with: git push --force-with-lease origin main"

# prepare .env and install go/frontend dependencies
setup:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -f .env ]; then
        echo "→ .env already present"
    else
        echo "→ creating .env from .env.example"
        cp .env.example .env
    fi
    echo "→ downloading go modules"
    go mod download
    echo "→ installing frontend dependencies"
    (cd frontend && npm install)
    echo "✓ setup complete"

# dev target: infra (default) | app | frontend — run each in its own terminal
dev target="infra":
    @just _dev-{{target}}

[private]
_dev-infra:
    ./scripts/dev.sh start --no-langfuse

[private]
_dev-app:
    ./scripts/dev.sh app

[private]
_dev-frontend:
    ./scripts/dev.sh frontend

# start the full stack via docker compose (--no-pull: build fork code locally,
# never let upstream wechatopenai:latest images shadow local builds)
start:
    ./scripts/build_frontend_dist.sh
    ./scripts/start_all.sh --no-pull

# stop the full stack
stop:
    ./scripts/start_all.sh --stop

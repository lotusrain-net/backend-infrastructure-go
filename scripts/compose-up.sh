#!/usr/bin/env sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
root_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)
env_file="$root_dir/.env"
example_env_file="$root_dir/.env.example"
compose_file="$root_dir/deployments/compose.yml"

fail() {
  printf '%s\n' "$1" >&2
  exit 1
}

generate_secret() {
  openssl rand -hex "$1"
}

if ! command -v docker >/dev/null 2>&1; then
  fail "Docker is required. Install Docker Desktop with WSL integration enabled."
fi
if ! docker compose version >/dev/null 2>&1; then
  fail "Docker Compose v2 is required."
fi
if ! docker info >/dev/null 2>&1; then
  fail "Docker is not running. Start Docker Desktop and try again."
fi
if ! command -v openssl >/dev/null 2>&1; then
  fail "openssl is required to generate local development secrets."
fi
if [ ! -f "$compose_file" ] || [ ! -f "$example_env_file" ]; then
  fail "Compose configuration or .env.example is missing."
fi

if [ ! -f "$env_file" ]; then
  postgres_password=$(generate_secret 24)
  jwt_secret=$(generate_secret 32)
  admin_password=$(generate_secret 24)

  cp "$example_env_file" "$env_file"
  sed -i \
    -e "s|^POSTGRES_PASSWORD=.*$|POSTGRES_PASSWORD=$postgres_password|" \
    -e "s|^JWT_SECRET=.*$|JWT_SECRET=$jwt_secret|" \
    -e "s|^ADMIN_PASSWORD=.*$|ADMIN_PASSWORD=$admin_password|" \
    "$env_file"
  chmod 600 "$env_file"
  printf 'Created .env with generated development-only secrets.\n'
fi

cd "$root_dir"
docker compose --env-file "$env_file" -f "$compose_file" config >/dev/null
docker compose --env-file "$env_file" -f "$compose_file" up -d --build --wait

web_port=$(sed -n 's/^WEB_PORT=//p' "$env_file" | tail -n 1)
api_port=$(sed -n 's/^HTTP_PORT=//p' "$env_file" | tail -n 1)
web_port=${web_port:-3000}
api_port=${api_port:-8080}

printf 'Web: http://127.0.0.1:%s\n' "$web_port"
printf 'API: http://127.0.0.1:%s\n' "$api_port"

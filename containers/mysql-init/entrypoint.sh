#!/usr/bin/env bash
set -euo pipefail

# MySQL initialization container entrypoint
# Inspired by home-operations/containers postgres-init

export INIT_MYSQL_SUPER_USER=${INIT_MYSQL_SUPER_USER:-root}
export INIT_MYSQL_PORT=${INIT_MYSQL_PORT:-3306}

# Check required environment variables
required_vars=(
  "INIT_MYSQL_HOST"
  "INIT_MYSQL_SUPER_PASS"
  "INIT_MYSQL_USER"
  "INIT_MYSQL_PASS"
  "INIT_MYSQL_DBNAME"
)

missing_vars=()
for var in "${required_vars[@]}"; do
  if [[ -z "${!var:-}" ]]; then
    missing_vars+=("$var")
  fi
done

if [[ ${#missing_vars[@]} -gt 0 ]]; then
  printf "\e[1;31m%-6s\e[m\n" "ERROR: Missing required environment variables:"
  for var in "${missing_vars[@]}"; do
    printf "\e[1;31m%-6s\e[m\n" "  - $var"
  done
  printf "\e[1;33m%-6s\e[m\n" ""
  printf "\e[1;33m%-6s\e[m\n" "Required environment variables:"
  printf "\e[1;33m%-6s\e[m\n" "  INIT_MYSQL_HOST       - MySQL server hostname"
  printf "\e[1;33m%-6s\e[m\n" "  INIT_MYSQL_SUPER_PASS - MySQL root/admin password"
  printf "\e[1;33m%-6s\e[m\n" "  INIT_MYSQL_USER       - Username to create"
  printf "\e[1;33m%-6s\e[m\n" "  INIT_MYSQL_PASS       - Password for the user"
  printf "\e[1;33m%-6s\e[m\n" "  INIT_MYSQL_DBNAME     - Database name(s) to create (space-separated)"
  printf "\e[1;33m%-6s\e[m\n" ""
  printf "\e[1;33m%-6s\e[m\n" "Optional environment variables:"
  printf "\e[1;33m%-6s\e[m\n" "  INIT_MYSQL_SUPER_USER - MySQL admin username (default: root)"
  printf "\e[1;33m%-6s\e[m\n" "  INIT_MYSQL_PORT       - MySQL port (default: 3306)"
  exit 1
fi

# Wait for MySQL to be ready
printf "\e[1;32m%-6s\e[m\n" "Waiting for MySQL host '${INIT_MYSQL_HOST}' on port '${INIT_MYSQL_PORT}' ..."
until mysqladmin ping -h "${INIT_MYSQL_HOST}" -P "${INIT_MYSQL_PORT}" -u "${INIT_MYSQL_SUPER_USER}" -p"${INIT_MYSQL_SUPER_PASS}" --silent; do
  printf "\e[1;32m%-6s\e[m\n" "MySQL not ready, waiting..."
  sleep 1
done

printf "\e[1;32m%-6s\e[m\n" "MySQL is ready!"

# Create user if it doesn't exist
printf "\e[1;32m%-6s\e[m\n" "Creating user '${INIT_MYSQL_USER}' if it doesn't exist..."
mysql -h "${INIT_MYSQL_HOST}" -P "${INIT_MYSQL_PORT}" -u "${INIT_MYSQL_SUPER_USER}" -p"${INIT_MYSQL_SUPER_PASS}" <<MYSQL_SCRIPT
CREATE USER IF NOT EXISTS '${INIT_MYSQL_USER}'@'%' IDENTIFIED BY '${INIT_MYSQL_PASS}';
MYSQL_SCRIPT

# Create databases and grant privileges
for dbname in ${INIT_MYSQL_DBNAME}; do
  printf "\e[1;32m%-6s\e[m\n" "Creating database '${dbname}' and granting privileges to '${INIT_MYSQL_USER}'..."
  mysql -h "${INIT_MYSQL_HOST}" -P "${INIT_MYSQL_PORT}" -u "${INIT_MYSQL_SUPER_USER}" -p"${INIT_MYSQL_SUPER_PASS}" <<MYSQL_SCRIPT
CREATE DATABASE IF NOT EXISTS \`${dbname}\`;
GRANT ALL PRIVILEGES ON \`${dbname}\`.* TO '${INIT_MYSQL_USER}'@'%';
MYSQL_SCRIPT
done

# Flush privileges
printf "\e[1;32m%-6s\e[m\n" "Flushing privileges..."
mysql -h "${INIT_MYSQL_HOST}" -P "${INIT_MYSQL_PORT}" -u "${INIT_MYSQL_SUPER_USER}" -p"${INIT_MYSQL_SUPER_PASS}" <<MYSQL_SCRIPT
FLUSH PRIVILEGES;
MYSQL_SCRIPT

printf "\e[1;32m%-6s\e[m\n" "MySQL initialization completed successfully!"
printf "\e[1;32m%-6s\e[m\n" "Created user: ${INIT_MYSQL_USER}"
printf "\e[1;32m%-6s\e[m\n" "Created databases: ${INIT_MYSQL_DBNAME}"


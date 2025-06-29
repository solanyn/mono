#!/usr/bin/env bash
set -euo pipefail

# MySQL initialization container entrypoint
# This container is designed to run MySQL client commands for database initialization

# Wait for MySQL to be ready
wait_for_mysql() {
    local host="${MYSQL_HOST:-localhost}"
    local port="${MYSQL_PORT:-3306}"
    local timeout="${MYSQL_TIMEOUT:-30}"
    
    echo "Waiting for MySQL at ${host}:${port} to be ready..."
    
    for i in $(seq 1 "$timeout"); do
        if mysql -h"$host" -P"$port" -u"${MYSQL_USER:-root}" -p"${MYSQL_PASSWORD}" -e "SELECT 1" >/dev/null 2>&1; then
            echo "MySQL is ready!"
            return 0
        fi
        echo "Waiting... ($i/$timeout)"
        sleep 1
    done
    
    echo "ERROR: MySQL is not ready after ${timeout} seconds"
    exit 1
}

# Initialize database if SQL files are provided
initialize_database() {
    local init_dir="${MYSQL_INIT_DIR:-/docker-entrypoint-initdb.d}"
    
    if [[ -d "$init_dir" ]]; then
        echo "Looking for SQL files in $init_dir..."
        for sql_file in "$init_dir"/*.sql; do
            if [[ -f "$sql_file" ]]; then
                echo "Executing $sql_file..."
                mysql -h"${MYSQL_HOST:-localhost}" -P"${MYSQL_PORT:-3306}" -u"${MYSQL_USER:-root}" -p"${MYSQL_PASSWORD}" < "$sql_file"
            fi
        done
    fi
}

# Main execution
main() {
    if [[ $# -eq 0 ]]; then
        # Default behavior: wait for MySQL and run initialization
        wait_for_mysql
        initialize_database
        echo "MySQL initialization completed successfully"
    else
        # Execute custom command
        exec "$@"
    fi
}

main "$@"
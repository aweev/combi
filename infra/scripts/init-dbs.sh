#!/bin/bash
set -e
set -u

# List of required microservice databases
databases=(identity auth offlinetoken trust kyc did biometric acl audit consent payments wallet billing fx escrow notification messaging connectivity eventbus gateway config migration)

for db in "${databases[@]}"; do
  if ! psql -U "$POSTGRES_USER" -lqt | cut -d \| -f 1 | grep -qw "$db"; then
    echo "Creating database: $db"
    psql -U "$POSTGRES_USER" -c "CREATE DATABASE $db;"
  fi
done

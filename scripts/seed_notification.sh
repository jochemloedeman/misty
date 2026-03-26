#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

if [ $# -lt 1 ]; then
    echo "Usage: $0 <recipient_id> [message]"
    echo "  recipient_id  UUID of the user to notify"
    echo "  message       notification text (default: \"This is a test notification\")"
    exit 1
fi

RECIPIENT_ID="$1"
MESSAGE="${2:-This is a test notification}"

PGPASSWORD="$(cat "$REPO_ROOT/secrets/postgres_password.txt")"
export PGPASSWORD

psql -h localhost -U postgres -d postgres -c "
    INSERT INTO notifications (id, recipient_id, message, sent_at)
    VALUES (gen_random_uuid(), '${RECIPIENT_ID}', '${MESSAGE}', NULL)
    RETURNING *;
"

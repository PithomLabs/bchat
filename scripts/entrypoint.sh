#!/usr/bin/env sh

file_env() {
   var="$1"
   fileVar="${var}_FILE"

   val_var="$(printenv "$var")"
   val_fileVar="$(printenv "$fileVar")"

   if [ -n "$val_var" ] && [ -n "$val_fileVar" ]; then
      echo "error: both $var and $fileVar are set (but are exclusive)" >&2
      exit 1
   fi

   if [ -n "$val_var" ]; then
      val="$val_var"
   elif [ -n "$val_fileVar" ]; then
      if [ ! -f "$val_fileVar" ]; then
         echo "error: secret file $val_fileVar does not exist" >&2
         exit 1
      fi
      val="$(cat "$val_fileVar")"
   fi

   export "$var"="$val"
   unset "$fileVar"
}

# =============================================================================
# Process sensitive environment variables with _FILE suffix support
# This allows secrets to be passed via files (Docker secrets, K8s secrets)
# Example: OPENROUTER_API_KEY_FILE=/run/secrets/api_key
# =============================================================================

# Database connection string
file_env "MEMOS_DSN"

# API Keys
file_env "OPENROUTER_API_KEY"

# Encryption master key for tenant API keys
file_env "ENCRYPTION_MASTER_KEY"

# AWS/S3 credentials (for Tigrisdata/LanceDB S3 storage)
file_env "AWS_ACCESS_KEY_ID"
file_env "AWS_SECRET_ACCESS_KEY"

# Cron token for webhook trigger endpoint
file_env "CRON_TOKEN"

# M3: H5 — Drop privileges to non-root user via gosu
# Container enters as root to fix volume permissions, then drops to memos user
if [ "$(id -u)" = '0' ]; then
   # Fix ownership of data volume (may be root-owned from fresh mount)
   chown -R memos:memos /var/opt/memos 2>/dev/null || true

   # Launch supercronic in background if available and CRON_TOKEN is set
   if command -v supercronic >/dev/null 2>&1 && [ -n "$CRON_TOKEN" ]; then
      supercronic /etc/bchat/crontab &
   fi

   # Drop to memos user and execute the main command
   if command -v gosu >/dev/null 2>&1; then
      exec gosu memos "$@"
   else
      echo "WARNING: gosu not found, running as root" >&2
      exec "$@"
   fi
fi

# Already running as non-root user — launch supercronic if available
if command -v supercronic >/dev/null 2>&1 && [ -n "$CRON_TOKEN" ]; then
   supercronic /etc/bchat/crontab &
fi

exec "$@"

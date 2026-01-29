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

exec "$@"

#!/usr/bin/env bash

function create_gpg_key() {
  local name=${1:-Flux}
  local email=${2:-support@weave.works}

  # https://www.gnupg.org/documentation/manuals/gnupg-devel/Unattended-GPG-key-generation.html
  local batchcfg
  batchcfg=$(mktemp)

  cat > "$batchcfg" << EOF
  %echo Generating a throwaway OpenPGP key for "$name <$email>"
  Key-Type: 1
  Key-Length: 2048
  Subkey-Type: 1
  Subkey-Length: 2048
  Name-Real: $name
  Name-Email: $email
  Expire-Date: 0
  %no-protection
  %commit
  %echo Done
EOF

  # Generate the key with the written config
  gpg --batch --gen-key "$batchcfg"
  rm "$batchcfg"

  # Find the ID of the key we just generated
  local key_id
  key_id=$(gpg --no-tty --list-secret-keys --with-colons "$name" 2> /dev/null |
    awk -F: '/^sec:/ { print $5 }' | tail -1)
  echo "$key_id"
}

function create_secret_from_gpg_key() {
  local key_id="${1}"

  if [ -z "$key_id" ]; then
    echo "no key ID provided" >&2
    exit 1
  fi

  # Export key to secret
  gpg --export-secret-keys "$key_id" |
    kubectl --namespace "${FLUX_NAMESPACE}" \
      create secret generic flux-gpg-signing-key \
      --from-file=flux.asc=/dev/stdin
}

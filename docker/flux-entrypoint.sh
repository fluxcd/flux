#!/bin/sh

gpg_import_dir="$HOME/gpg-import"

if [ -d "$gpg_import_dir" ]; then
    IFS="$(printf '\n')"
    for key in "$gpg_import_dir"/*; do
        if [ -e "$key" ]; then
            echo "Importing GPG key: $(basename "$key")"
            gpg --import "$key" >/dev/null 2>&1
        fi
    done
fi

exec fluxd "$@"

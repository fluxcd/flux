---
title: Git commit signing
menu_order: 90
---

# Summary

Flux can be configured to sign commits that it makes to the user git repo when, for example, it detects an updated Docker image is available for a release with automatic deployments enabled. Enabling this feature requires two steps:

1. Importing the GPG key into the flux container's GPG keyring
2. Configuring flux to use this key with the `--git-signing-key` flag

# Importing GPG key

Any file found in `/root/gpg-import` will be imported into GPG; therefore, by volume-mounting a key into that directory it will be made available for use by flux.

# Using the `--git-signing-key` flag

Once a key has been imported, all that needs to be done is to specify that git commit signing should be performed by providing the `--git-signing-key` flag and the ID of the key to use. For example:

`--git-signing-key 649C056644DBB17D123D699B42532AEA4FFBFC0B`

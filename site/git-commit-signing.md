---
title: Git commit signing
menu_order: 90
---

# Summary

Flux can be configured to sign commits that it makes to the user git
repo when, for example, it detects an updated Docker image is available
for a release with automatic deployments enabled. Enabling this feature
requires the configuration of two flags:

1. `--git-gpg-key-import` should be set to the path Flux should look
   for GPG key(s) to import, this can be a direct path to a key or the
   path to a folder Flux should scan for files. 
2. `--git-signing-key` should be set to the ID of the key Flux should
   use to sign commits, for example: `649C056644DBB17D123D699B42532AEA4FFBFC0B`

# Importing GPG key(s)

Any file found in the configured `--git-gpg-key-import` path will be
imported into GPG; therefore, by volume-mounting a key into that
directory it will be made available for use by Flux.

> **Note:** Flux *does not* recursively scan a given directory but does
understand symbolic links to files.

# Using the `--git-signing-key` flag

Once a key has been imported, all that needs to be done is to specify
that git commit signing should be performed by providing the
`--git-signing-key` flag and the ID of the key to use. For example:

`--git-signing-key 649C056644DBB17D123D699B42532AEA4FFBFC0B`

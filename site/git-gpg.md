---
title: Git commit signing and verification
menu_order: 90
---

- [Summary](#summary)
- [Commit signing](#commit-signing)
  * [Creating a GPG signing key](#creating-a-gpg-signing-key)
  * [Importing a GPG signing key](#importing-a-gpg-signing-key)
- [Signature verification](#signature-verification)
  * [Importing trusted GPG keys and enabling verification](#importing-trusted-gpg-keys-and-enabling-verification)
  * [Enabling verification for existing repositories, disaster recovery, and deleted sync tags](#enabling-verification-for-existing-repositories-disaster-recovery-and-deleted-sync-tags)

# Summary

Flux can be configured to sign commits that it makes to the user git
repo when, for example, it detects an updated Docker image is available
for a release with automatic deployments enabled. To complete this
functionality it is also able to verify signatures of commits (and the
sync tag in git) to prevent Flux from applying unauthorized changes on
the cluster.

# Commit signing

The signing of commits (and the sync tag) requires two flags to be set:

1. `--git-gpg-key-import` should be set to the path Flux should look
   for GPG key(s) to import, this can be a direct path to a key or the
   path to a folder Flux should scan for files. 
2. `--git-signing-key` should be set to the ID of the key Flux should
   use to sign commits, this can be the full fingerprint or the long
   ID, for example: `700D397C988079BFF0DDAFED6A7436E8790F8689` (or
   `6A7436E8790F8689`)

Once enabled Flux will sign both commits and the sync tag with given
`--git-signing-key`.

## Creating a GPG signing key

> **Note:** This requires [gnupg](https://www.gnupg.org) to be
installed on your system.

1. Enter the following shell command to start the key generation dialog:

   ```sh
     $ gpg --full-generate-key
   ```

2. The dialog will guide you through the process of generating a key.
   Pressing the `Enter` key will assign the default value, please note
   that when in doubt, in almost all cases, the default value is
   recommended.

   Select what kind of key you want and press `Enter`:

   ```sh
   Please select what kind of key you want:
   (1) RSA and RSA (default)
   (2) DSA and Elgamal
   (3) DSA (sign only)
   (4) RSA (sign only)
   Your selection? 1
   ```

3. Enter the desired key size (or simply press `Enter` as the default
   will be secure for almost any setup):

   ```sh
   RSA keys may be between 1024 and 4096 bits long.
   What keysize do you want? (2048)
   ```

4. Specify how long the key should be valid (or simply press `Enter`):

  ```sh
  Please specify how long the key should be valid.
         0 = key does not expire
      <n>  = key expires in n days
      <n>w = key expires in n weeks
      <n>m = key expires in n months
      <n>y = key expires in n years
  Key is valid for? (0)
  ```

5. Verify your selection of choices and accept (`y` and `Enter`)

6. Enter your user ID information, it is recommended to set the email
   address to the same address as the daemon uses for Git operations.

7. **Do not enter a passphrase**, as Flux will be unable to sign with a
   passphrase protected private key, instead, keep it in a secure place.

8. You can validate the public and private keypair were created with
   success by running:

   ```sh
   $ gpg --list-secret-keys --keyid-format long <email address>
   sec   rsa2048/6A7436E8790F8689 2019-03-28 [SC]
         700D397C988079BFF0DDAFED6A7436E8790F8689
   uid                 [ultimate] Weaveworks Flux <support@weave.works>
   ssb   rsa2048/ECA4FF5BD988B8E9 2019-03-28 [E]
   ```

## Importing a GPG signing key

Any file found in the configured `--git-gpg-key-import` path will be
imported into GPG; therefore, by volume-mounting a key into that
directory it will be made available for use by Flux.

1. Retrieve the key ID (second row of the `sec` column):

   ```sh
   $  gpg --list-secret-keys --keyid-format long <email address>
   sec   rsa2048/6A7436E8790F8689 2019-03-28 [SC]
         700D397C988079BFF0DDAFED6A7436E8790F8689
   uid                 [ultimate] Weaveworks Flux <support@weave.works>
   ssb   rsa2048/ECA4FF5BD988B8E9 2019-03-28 [E]
   ```

2. Export the public and private keypair from your local GPG keyring
   to a Kubernetes secret with `--export-secret-keys <key id>`:

   ```sh
   $ gpg --export-secret-keys 700D397C988079BFF0DDAFED6A7436E8790F8689 |
     kubectl create secret generic flux-gpg-keys --from-file=flux.asc=/dev/stdin --dry-run -o yaml
   apiVersion: v1
   data:
     flux.asc: <base64 string>
   kind: Secret
   metadata:
     creationTimestamp: null
     name: flux-gpg-keys

3. Adapt your Flux deployment to mount the secret and enable the
   signing of commits:

   ```yaml
   spec:
     template:
        spec:
          volumes:
          - name: gpg-keys
              secret:
                secretName: flux-gpg-keys
                defaultMode: 0400
          containers:
          - name: flux
            volumeMounts:
            - name: gpg-keys
              mountPath: /root/gpg-import
              readOnly: true
            args:
            - --git-gpg-key-import=/root/gpg-import
            - --git-signing-key=700D397C988079BFF0DDAFED6A7436E8790F8689 # key id
   ```

   or set the `gpgKeys.secretName` in your Helm `values.yaml` to
   `gpg-keys`, and `signingKey` to your `<key id>`.

4. To validate your setup is working, run `git log --show-signature` or
   `git verify-tag <configured label>` to assure Flux signs its git
   actions.

   ```sh
   $ git verify-tag <configured label>
   gpg: Signature made vr 29 mrt 2019 15:28:34 CET
   gpg:                using RSA key 700D397C988079BFF0DDAFED6A7436E8790F8689
   gpg: Good signature from "Weaveworks Flux <support@weave.works>" [ultimate]
   ```

> **Note:** Flux *does not* recursively scan a given directory but does
understand symbolic links to files.

> **Note:** Flux will automatically add any imported key to the GnuPG
  trustdb. This is required as git will otherwise not trust signatures
  made with the imported keys.

# Signature verification

The verification of commit signatures is enabled by importing all
trusted public keys (`--git-gpg-key-import=<path>`), and by setting the
`--gpg-verify-signatures` flag. Once enabled Flux will verify all
commit signatures, and the signature from the sync tag it is comparing
revisions with.

In case a signature can not be verified, Flux will sync state up to the
last valid revision it can find _before_ the unverified commit was
made, and lock on this revision.

## Importing trusted GPG keys and enabling verification

1. Collect the public keys from all trusted git authors.

2. Add the collected keys to your secret with GPG keys.

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: flux-gpg-keys
   data:
     # ...
     author.asc: <base64 string>
   ```

3. Adapt your Flux deployment to enable the verification of commits:

   ```yaml
   spec:
     template:
        spec:
          volumes:
          - name: gpg-keys
              secret:
                secretName: flux-gpg-keys
                defaultMode: 0400
          containers:
          - name: flux
            volumeMounts:
            - name: gpg-keys
              mountPath: /root/gpg-import
              readOnly: true
            args:
            - --git-verify-signatures
   ```

> **Note:** Flux *does not* recursively scan a given directory but does
understand symbolic links to files.

## Enabling verification for existing repositories, disaster recovery, and deleted sync tags

In case you have existing commits in your repository without a
signature you may want to:

a. First enable signing by setting the `--git-gpg-key-import` and
   `--git-signing-key`, after Flux has synchronized the first commit
   with a signature, enable verification.

b. Sign the sync tag by yourself, with a key that is imported, to point
   towards the first commit with a signature (or the current `HEAD`).
   Flux will then start synchronizing the changes between the sync tag
   revision and `HEAD`.

   ```sh
   $ git tag --force --local-user=<key id> -a -m "Sync pointer" <tag name> <revision>
   $ git push --force origin <tag name>
   ```

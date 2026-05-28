---
title: "Secrets"
date: '2026-05-26T00:00:00+08:00'
weight: 50
---

Composia manages encrypted secret files in the desired-state repository using age encryption. Encryption and decryption happen on the controller. Agents never access the age private key.

## Configuration

Secrets require an age key pair. Set up in the controller config:

```yaml
controller:
  secrets:
    provider: age
    identity_file: "/app/configs/age-identity.key"
```

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `provider` | `string` | Yes | Must be `age`. |
| `identity_file` | `string` | Yes | Path to the age private key file. |
| `recipient_file` | `string` | No | Path to file containing age recipients (public keys). If omitted, the recipient is derived from the private key. |
| `armor` | `bool` | No | Use ASCII-armored output. Defaults to `true`. |

Generate a key pair:

```bash
age-keygen -o age-identity.key
```

Optional: extract the public key as a recipient:

```bash
age-keygen -y age-identity.key > age-recipients.txt
```

## How secrets are stored

Secret files in the repository have a `.enc` extension by convention. They are stored as age-encrypted ciphertext:

```
my-app/
├── docker-compose.yaml
├── composia-meta.yaml
└── .secret.env.enc        (encrypted with age)
```

The controller encrypts plaintext on write and decrypts on read. The repository contains only ciphertext. Secrets never appear as plaintext in the repo, in task logs, or in transit to agents.

## How secrets reach agents

During the render step of a deploy or update task, the controller:

1. Reads encrypted files from the service directory in the repo.
2. Decrypts each file using the age private key.
3. Injects the decrypted content into the service bundle as `.composia-secret.env`.

The bundle is streamed to the agent over the agent report connection. The agent writes the bundle to disk and proceeds with `docker compose up`. The decrypted secret environment is available to the Compose services without the agent ever seeing the private key.

## CLI usage

Write an encrypted secret file:

```bash
composia secret update my-app .secret.env.enc --file ./local-plain.env
```

Read and decrypt a secret file:

```bash
composia secret get my-app .secret.env.enc
```

Edit a secret in place (opens your editor):

```bash
composia secret edit my-app .secret.env.enc
```

All secret write operations include a base revision check to prevent conflicts with concurrent changes.

## File path rules

Secret file paths must:

- Be relative to the service directory (not absolute).
- Not contain path traversal sequences like `../`.
- Point to a file inside the service directory.

The controller locates the service, resolves the file path relative to the service directory, and operates on the repo file.

## Error conditions

- **Secrets not configured**: `GetSecret` and `UpdateSecret` return `FailedPrecondition` when `controller.secrets` is not set.
- **File not found**: `GetSecret` returns an empty content response rather than an error when the file does not exist. This lets clients distinguish between missing files and decryption failures.
- **Base revision conflict**: `UpdateSecret` uses CAS (compare-and-swap) against the repo HEAD. If the repo changed since the last read, the write fails with a revision conflict.

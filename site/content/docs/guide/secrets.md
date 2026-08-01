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

The repository filename, including `.enc`, is the canonical name used by the file tree, Repo API, CLI, and `composia-meta.yaml`. `GetRepoFile` returns plaintext for `.enc` files, and `UpdateRepoFile` accepts plaintext and encrypts it before committing. The repository contains only ciphertext.

## How secrets reach agents

During the render step of a deploy or update task, the controller:

1. Reads encrypted files from the service directory in the repo.
2. Decrypts each file using the age private key.
3. Removes the `.enc` suffix and injects the decrypted content under its runtime filename. For example, `.secret.env.enc` becomes `.secret.env`.

Encrypted repository files are omitted from the runtime bundle. The bundle is streamed to the agent over the agent report connection. The agent writes the decrypted runtime files to disk and proceeds with `docker compose up` without ever receiving the private key.

File references in `composia-meta.yaml` use repository names such as `.env.enc`; Composia converts them to runtime names before use. References inside native files such as Compose files, Caddyfiles, and scripts use the runtime name `.env` because those files are interpreted by their native tools.

## CLI usage

Create or edit an encrypted file relative to a service directory:

```bash
composia service my-app edit .secret.env.enc
```

Read and decrypt it through the Repo API:

```bash
composia repo get my-app/.secret.env.enc
```

Replace it from a local plaintext file:

```bash
composia repo update --file ./local-plain.env my-app/.secret.env.enc
```

All Repo write operations include a base revision check to prevent conflicts with concurrent changes.

## File path rules

Encrypted file paths must:

- Be repository-relative when using Repo commands, or service-relative when using `service edit`.
- Not contain path traversal sequences like `../`.
- Point to a file; `.enc` is reserved and cannot be used by a directory path segment.

The `.enc` suffix is exact and case-sensitive. Directories cannot use it, and moves cannot cross between `.enc` and non-`.enc` names.

A file and its encrypted counterpart cannot coexist in the repository: for example, `.env` and `.env.enc` are mutually exclusive.

## Error conditions

- **Secrets not configured**: Repo reads and writes for `.enc` files return `FailedPrecondition` when `controller.secrets` is not set.
- **File not found**: `GetRepoFile` returns `NotFound`.
- **Invalid ciphertext**: `GetRepoFile` returns `DataLoss` when an `.enc` file cannot be decrypted.
- **Base revision conflict**: `UpdateRepoFile` uses CAS (compare-and-swap) against the repo HEAD. If the repo changed since the last read, the write fails with a revision conflict.

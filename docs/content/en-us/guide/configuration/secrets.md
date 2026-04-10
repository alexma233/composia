# Secrets Configuration

This page documents the `controller.secrets` configuration.

## Example

```yaml
controller:
  secrets:
    provider: age
    identity_file: "/app/configs/age-identity.key"
    recipient_file: "/app/configs/age-recipients.txt"
    armor: true
```

## Fields

| Field | Description |
|-------|-------------|
| `provider` | Encryption provider; currently only `age` is supported |
| `identity_file` | Path to the age private key file |
| `recipient_file` | Path to the age public key file |
| `armor` | Whether to use ASCII Armor format |

If the `secrets` section is present, `provider`, `identity_file`, and `recipient_file` are all required, and `provider` must be `age`.

## Generate age Keys

```bash
# Generate age key pair
age-keygen -o key.txt

# Extract public key
cat key.txt | grep "public key" > recipients.txt
```

When mounting into containers:

- `key.txt` is the `identity_file` (private key)
- `recipients.txt` is the `recipient_file` (public key)

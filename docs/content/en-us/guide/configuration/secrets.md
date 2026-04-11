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
| `recipient_file` | Optional path to the age public key file; when omitted it is derived from `identity_file` |
| `armor` | Whether to use ASCII Armor format |

If the `secrets` section is present, `provider` and `identity_file` are required, `recipient_file` is optional, and `provider` must be `age`.

## Generate age Keys

```bash
# Generate age key pair
age-keygen -o key.txt

# Extract public key
cat key.txt | grep "public key" > recipients.txt
```

When mounting into containers:

- `key.txt` is the `identity_file` (private key)
- `recipients.txt` can be used as the `recipient_file` (public key)

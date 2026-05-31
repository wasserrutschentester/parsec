# Update Command

The `update` command checks for a newer version of `parsec` and performs an in-place update if available.

## Usage

```bash
parsec update [flags]
```

## Features

-   **Automatic Check**: Fetches the latest release information from the project's repository.
-   **Security**: Verifies the checksum of the downloaded binary against the official `checksums.txt` if available.
-   **Seamless Update**: Replaces the currently running binary with the new version.

## Flags

| Flag | Shorthand | Type | Description |
|------|-----------|------|-------------|
| `--force` | `-f` | boolean | Force update even if the current version is the same or newer than the latest release. |

## Examples

**Check and update to the latest version:**
```bash
parsec update
```

**Force a re-installation of the latest version:**
```bash
parsec update --force
```

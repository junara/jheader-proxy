---
description: Guide to using `jheader-proxy`, a local HTTP/HTTPS proxy CLI that appends request headers to specific domains for iPhone/Android debugging. Covers subcommands run / gen-ca / gui / version, HTTPS MITM via a self-generated CA, the config file, and safe handling of secret headers.
metadata:
    github-path: plugins/jheader-proxy
    github-repo: https://github.com/junara/jheader-proxy
name: jheader-proxy
---
# jheader-proxy — request-header-injecting local proxy skill

A macOS/Linux Go CLI that runs a local HTTP/HTTPS proxy. Point an iPhone/Android
Wi-Fi proxy at it, and only requests to the domains you specify get the extra
request headers you pass. Typical use: adding a debug header (`X-Debug-User`, a
PR number, a feature flag) when browsing a dev/staging site from iPhone Safari.

## Prerequisites

- `jheader-proxy` must be built (`go build -o jheader-proxy ./cmd/jheader-proxy`)
  or installed (`brew install junara/tap/jheader-proxy`).
- For HTTPS, a self-generated CA is **required** (see `gen-ca`). There is no
  built-in CA — the private key must exist only on your machine.

## Command layout

```
jheader-proxy run     [flags]   # start the proxy
jheader-proxy gen-ca  [flags]   # generate your own CA cert + key (required for HTTPS)
jheader-proxy gui     [flags]   # browser-based admin UI (127.0.0.1 only)
jheader-proxy version           # print version (--version also works)
jheader-proxy help    [command] # usage; 'help <command>' for a specific command
```

The legacy flag form (`--gen-ca`, `--gui`, or `run` flags with no subcommand)
still works for backward compatibility but is **deprecated** and prints a warning.
`--version` stays warning-free. Prefer the subcommands.

## Generating a CA (do this first, for HTTPS)

```bash
jheader-proxy gen-ca \
  --cert jheader-proxy-ca-cert.pem \
  --key jheader-proxy-ca-key.pem
```

- Produces a self-signed CA: RSA 2048-bit, ~10-year validity.
- The key file is written with `0600` permissions.
- Refuses to overwrite existing files unless `--force` is given.
- Install `…-ca-cert.pem` on the phone and enable trust. **Never** send or commit
  the key (`…-ca-key.pem`).

### gen-ca flags

| Flag | Alias | Default | Description |
|---|---|---|---|
| `--cert` | `--ca-cert` | | Output path for the CA certificate PEM (required) |
| `--key` | `--ca-key` | | Output path for the CA private key PEM (required) |
| `--force` | | false | Overwrite existing output files |

## Running the proxy

```bash
# Minimal HTTPS-capable run
jheader-proxy run \
  --domain example.test \
  --header "X-Debug-User=jun" \
  --ca-cert jheader-proxy-ca-cert.pem \
  --ca-key jheader-proxy-ca-key.pem

# Multiple domains and headers (repeat the flags)
jheader-proxy run \
  --domain example.test --domain api.example.test \
  --header "X-Debug-User=jun" --header "X-From-iPhone=true" \
  --ca-cert ca-cert.pem --ca-key ca-key.pem

# Restrict which clients may connect, and change the listen port
jheader-proxy run \
  --listen :9090 --allow 192.168.1.23 \
  --domain example.test --header "X-Debug-User=jun" \
  --ca-cert ca-cert.pem --ca-key ca-key.pem
```

### run flags

| Flag | Default | Description |
|---|---|---|
| `--config` | | Path to a JSON config file (shared schema with the GUI; flags win over it) |
| `--listen` | `:8080` | Proxy listen address |
| `--domain` | | Target domain (repeatable; subdomains included). At least one required |
| `--header` | | Header as `Name=Value` (repeatable). At least one required (with `--header-file`) |
| `--header-file` | | File of `Name=Value` lines (repeatable). Use this for secret values — see below |
| `--ca-cert` | | CA certificate PEM path (required) |
| `--ca-key` | | CA private key PEM path (required) |
| `--duration` | `10m` | Auto-stop after this long. `0` = unlimited |
| `--allow` | | Allowed client IP/CIDR (repeatable). Unset = allow all |
| `--redact` | false | Mask all header values in the startup log |
| `--quiet` | false | Suppress per-request logs. **Mutually exclusive with `--verbose`** |
| `--verbose` | false | Also log responses from target domains. **Mutually exclusive with `--quiet`** |

## Passing secret headers safely (--header-file)

Auth tokens passed via `--header "Authorization=Bearer …"` end up in your shell
history and in `ps` output. Put secret headers in a file instead:

```bash
jheader-proxy run \
  --domain example.test \
  --header-file headers.txt \
  --ca-cert ca-cert.pem --ca-key ca-key.pem
```

`headers.txt` (one `Name=Value` per line; `#` lines and blanks ignored):

```text
# debug headers for staging
Authorization=Bearer dummy-token
X-Debug-User=jun
```

- Same parsing rules as `--header` (first `=` splits; name/value trimmed).
- Combine with `--header`; on a name collision the inline `--header` wins.
- `chmod 600 headers.txt` — it holds credentials.
- `--config` (below) can also carry secret headers.

## Config file (--config, shared with the GUI)

Instead of long argument lists, put everything in one JSON file. It is the same
schema the GUI writes to `~/Library/Application Support/jheader-proxy/config.json`,
so a config built in the GUI can be replayed on the CLI verbatim.

```bash
jheader-proxy run --config jheader-proxy.json
```

```json
{
  "listen": ":8080",
  "domains": ["example.test", "api.example.test"],
  "headers": [{ "name": "X-Debug-User", "value": "jun" }],
  "allow": ["192.168.1.23"],
  "duration": "30m",
  "quiet": false,
  "verbose": false,
  "redact": false,
  "caCertPath": "jheader-proxy-ca-cert.pem",
  "caKeyPath": "jheader-proxy-ca-key.pem"
}
```

- Precedence is **flag > config file > default**. A flag given on the command
  line overrides the same field in the file.
- `--domain` / `--header` / `--header-file` / `--allow` **replace** (not merge)
  the file's corresponding list when specified on the command line.
- Give the file tight permissions if it contains secret headers (`chmod 600`).

## Domain matching

`--domain example.test` targets the domain itself and its subdomains only.

| Targeted (headers added) | Not targeted (HTTPS passed through untouched) |
|---|---|
| `example.test` | `evilexample.test` |
| `app.example.test` | `example.test.evil.com` |
| `api.example.test` | `example.com` |

The check is `host == domain || strings.HasSuffix(host, "."+domain)` after
normalizing the host (lowercase, port stripped). `strings.Contains` is not used,
so lookalike domains are never matched by accident. Non-target HTTPS is relayed
as a plain CONNECT tunnel and is never MITM'd.

## Installing the CA on a device (portal shortcut)

With the device's Wi-Fi proxy pointed at jheader-proxy, open **`http://jheader.proxy`**
(must be `http://`) in the device browser to download the CA certificate — no
AirDrop needed. Only the certificate is served; the private key never is. Then
enable trust (iOS: Settings → General → About → Certificate Trust Settings).

## gui subcommand

```bash
jheader-proxy gui                       # opens http://127.0.0.1:9090 and launches a browser
jheader-proxy gui --listen 127.0.0.1:9191
jheader-proxy gui --no-open             # don't auto-launch a browser
```

| Flag | Default | Description |
|---|---|---|
| `--listen` | `127.0.0.1:9090` | Admin UI listen address |
| `--no-open` | false | Do not auto-launch a browser |

The admin UI binds to `127.0.0.1` only and is protected by a per-start random
token. The proxy itself is started/stopped from the page; closing the browser
does not stop the proxy. `Ctrl+C` in the terminal stops both.

## Logs

Per-request tags (suppressed by `--quiet`, except `[DENY]` and the startup banner):

| Tag | Meaning |
|---|---|
| `[MITM]` | Target host — decrypting and injecting headers |
| `[TUNNEL]` | Non-target HTTPS — passed through untouched |
| `[ADD HEADER]` | Headers were added to this request |
| `[RESP]` | Target-domain response (only with `--verbose`) |
| `[DENY]` | A client not in `--allow` was rejected |
| `[CA PORTAL]` | The `http://jheader.proxy` CA download page was served |

The startup banner lists listen address, target domains, CA path, CA expiry
(warns within 14 days / when expired), auto-stop, allowed clients, and headers.
`Authorization` / `Proxy-Authorization` / `Cookie` / `Set-Cookie` / `X-Api-Key`
values are masked as `***` even without `--redact`.

## Safety defaults

- **Auto-stop** after `10m` by default (`--duration`), so a forgotten proxy stops
  itself. Use `--duration 0` for unlimited.
- **Client allowlist** (`--allow`): on shared Wi-Fi, restrict to the phone's IP so
  no one else can use the proxy as a relay.
- After testing, turn the phone's Wi-Fi proxy and certificate trust back **off**.
  A trusted CA lets whoever holds its key MITM all HTTPS on that device.

## Error hints

- Missing CA files at `run` → the error suggests the exact `gen-ca --cert … --key …`
  command to create them.
- Listen address already in use → the error names the address and suggests a
  different port via `--listen`, or stopping the process using it.
- `--quiet` and `--verbose` together → rejected as a conflicting combination.

## Common recipes

### Quickest HTTPS debug session

```bash
jheader-proxy gen-ca --cert ca-cert.pem --key ca-key.pem   # once; install cert on phone
jheader-proxy run --domain example.test --header "X-Debug-User=jun" \
  --ca-cert ca-cert.pem --ca-key ca-key.pem
# On phone: Wi-Fi proxy → Mac IP : 8080, open http://jheader.proxy to grab the cert
```

### Reproducible, secret-safe run (config + header file)

`jheader-proxy.json` (no secrets) + `headers.txt` (`chmod 600`, secrets only):

```bash
jheader-proxy run --config jheader-proxy.json --header-file headers.txt
```

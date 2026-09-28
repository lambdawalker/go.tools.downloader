# Auth Package (`internal/auth`)

The `auth` package provides an extensible authentication layer, master-password-encrypted credential vault (AES-256-GCM + PBKDF2), domain-scoped security transports, persistent credential caches, and typed actionable error classifications for HTTP downloads.

## Responsibilities

1. **Master-Password Encrypted Vault (`vault.go`, `password.go`)**:
   - Encrypted local keystore stored at `~/.downloader/auth/vault.enc`.
   - **Key Derivation**: PBKDF2-HMAC-SHA256 with 100,000 iterations using a cryptographically random 32-byte salt.
   - **Authenticated Encryption**: AES-256-GCM with a unique 12-byte nonce per write and magic header validation (`DLVAULT1`).
   - Tamper detection: Any modification or incorrect password fails decryption immediately with `ErrInvalidMasterPassword`.
   - In-memory lifecycle: Decrypted profiles exist exclusively in RAM during runtime; secrets are never stored in plaintext on disk or exposed in environment variables.
   - Terminal raw-mode password prompts with no echo via `golang.org/x/term`. Supports automated input via `-password-file` and `-password-stdin`.

2. **Authentication Abstraction (`strategy.go`)**:
   - `Strategy` interface with `Apply(req *http.Request) error`, `Type() MethodType`, and `Headers() []string`.
   - Implementations for **Bearer Tokens**, **HTTP Basic Auth**, **Custom HTTP Headers** (e.g. `X-API-Key`, `Private-Token`), and **Anonymous** access.

3. **Domain Scoping & Anti-Leak Transport (`transport.go`)**:
   - `ScopedTransport` wraps `http.RoundTripper` to ensure credentials and sensitive headers are injected exclusively when connecting to the primary target domain.
   - Prevents credential leakage across cross-domain redirects (such as redirects to Amazon S3, Cloudflare, or presigned CDN URLs), avoiding HTTP 400/403 errors caused by conflicting authorization signatures.
   - `NewRedirectHandler` reinforces header stripping at the `http.Client.CheckRedirect` lifecycle stage.

4. **Persistent Credential & Mode Cache (`store.go`)**:
   - Manages an auxiliary persistent domain cache at `~/.downloader/auth/config.json`.
   - Maps domain names and site identifiers to configured authentication methods.

5. **Interactive Prompter & Manager (`prompter.go`, `manager.go`)**:
   - Checks both the encrypted vault and domain cache before downloading.
   - If an unrecognized domain is encountered, prompts the user once, remembers the selection, and runs silently on future requests.

6. **Typed & Actionable Error Handling (`errors.go`)**:
   - `AuthError` struct distinguishing between:
     - `KindUnauthorized` (HTTP 401: missing, invalid, or expired token)
     - `KindForbidden` (HTTP 403: insufficient permissions / scopes)
     - `KindRateLimited` (HTTP 429: rate limits, with parsed `Retry-After` duration)
     - `KindInvalidCredentials` (local configuration failure)
   - Convenience helpers: `IsAuthError`, `AsAuthError`, `IsUnauthorized`, `IsForbidden`, `IsRateLimited`.

# Multiplatform Installer and Release Design

## 1. Purpose

Provide a versioned, secure, and automated installation experience for
`happy-memory` without requiring users to install Go, SQLite, GoReleaser, or
other build tools.

The normal installation commands will be:

```sh
curl -fsSL https://raw.githubusercontent.com/jorgeluis594/happy-memory/main/install.sh | sh
```

```powershell
irm https://raw.githubusercontent.com/jorgeluis594/happy-memory/main/install.ps1 | iex
```

The scripts will detect the host, download the correct precompiled artifact
from GitHub Releases, verify its checksum, extract it, and install the
executable in a user-owned directory.

## 2. Goals

- Publish immutable releases from Git tags through GitHub Actions and
  GoReleaser.
- Produce standalone binaries for supported operating systems and
  architectures.
- Install the latest stable release with one command on macOS, Linux, and
  Windows.
- Allow installation of an explicit version and into a custom directory.
- Verify downloaded artifacts before installing them.
- Avoid administrative privileges by default.
- Make installation and upgrades idempotent.
- Expose the installed build version through the CLI.

## 3. Non-goals

The first implementation will not provide:

- Homebrew, Scoop, WinGet, APT, RPM, or other package-manager repositories.
- `.deb`, `.rpm`, `.msi`, `.pkg`, or `.dmg` installers.
- macOS signing or notarization.
- Windows Authenticode signing.
- Automatic background updates.
- Installation from branches, arbitrary commits, or untrusted repositories.
- Support for operating systems or architectures not listed in this document.

## 4. Supported platforms

The release pipeline must build and publish the following targets:

| Operating system | Go target | Architectures | Archive |
| --- | --- | --- | --- |
| Linux | `linux` | `amd64`, `arm64` | `.tar.gz` |
| macOS | `darwin` | `amd64`, `arm64` | `.tar.gz` |
| Windows | `windows` | `amd64`, `arm64` | `.zip` |

Windows ARM64 is supported only if the cross-compiled binary passes the
release validation defined below. If it cannot be validated in CI, the initial
release matrix must omit it rather than publish an unverified artifact.

The executables must be named `happy-memory` on Unix-like systems and
`happy-memory.exe` on Windows.

## 5. Versioning and release trigger

- Public versions must follow Semantic Versioning and use Git tags in the form
  `vMAJOR.MINOR.PATCH`, for example `v0.2.0`.
- A pushed tag matching `v*` must trigger the release workflow.
- GoReleaser must derive the release version from the Git tag.
- The workflow must fetch the complete Git history and tags so GoReleaser can
  generate correct release metadata.
- A release must be published only after repository tests and build validation
  pass.
- Re-running a workflow must not silently replace immutable artifacts from an
  already published version.
- Prerelease tags recognized by Semantic Versioning may publish GitHub
  prereleases, but the default installer must ignore them when resolving the
  latest stable version.

## 6. CLI build metadata

The application must expose:

```text
happy-memory version
```

The command must report at least:

- Semantic version.
- Git commit.
- Build date in UTC.

GoReleaser must inject these values at build time through linker flags. Local
development builds must return explicit fallback values such as `dev`,
`unknown`, and `unknown`; they must not fail because release metadata is
absent.

The exact output format must remain stable enough for the installers to check
that the installed executable starts successfully. Installers must rely on the
command's exit status, not parse human-readable text.

## 7. Release artifacts

GoReleaser must create one archive per operating-system and architecture pair.
Artifact names must be deterministic and include:

- Project name.
- Version.
- Operating system.
- Architecture.

The GoReleaser configuration is the source of truth for artifact naming. The
installers must implement exactly the same mapping and automated tests must
detect divergence between the configuration and scripts.

Each archive must contain only the platform executable and any release files
explicitly required for licensing. It must not contain source code, build
caches, the local memory database, or development configuration.

Every release must include one checksum manifest named `checksums.txt` with a
SHA-256 entry for every published archive.

## 8. GitHub Actions release workflow

The repository must contain a release workflow with these responsibilities:

1. Run only for eligible release tags and manual validation when appropriate.
2. Check out the complete repository history.
3. Install the Go version declared by the repository.
4. Run the relevant test and validation suite.
5. Run a pinned GoReleaser major version with `release --clean`.
6. Publish archives, checksums, and release notes to GitHub Releases.
7. Use the workflow-provided GitHub token with only the permissions required
   to publish release contents.

Pull requests must validate the GoReleaser configuration and perform a
snapshot build without publishing a release.

Third-party GitHub Actions must be pinned to an intentional version. The
workflow must not expose repository credentials to build scripts or untrusted
pull-request code.

## 9. Unix installer contract

The repository must provide `install.sh`, compatible with POSIX `sh`. The
documented default invocation is:

```sh
curl -fsSL https://raw.githubusercontent.com/jorgeluis594/happy-memory/main/install.sh | sh
```

The script must:

1. Fail on errors and reject incomplete downloads.
2. Detect Linux or macOS through `uname -s`.
3. Map `x86_64` to `amd64` and `arm64` or `aarch64` to `arm64`.
4. Reject unsupported platforms with a clear, actionable error.
5. Resolve the latest stable GitHub Release when no version is supplied.
6. Normalize an explicit version to the release tag contract without accepting
   arbitrary URLs or paths.
7. Download the platform archive and `checksums.txt` over HTTPS.
8. Verify the archive's SHA-256 checksum before extraction.
9. Extract only the expected executable into a temporary directory.
10. Install atomically into the target directory.
11. Ensure the installed Unix executable has execute permission.
12. Run the installed executable's `version` command and require success.
13. Remove temporary files on success and failure.
14. Print the installed version, path, and any required `PATH` guidance.

The default target directory is:

```text
$HOME/.local/bin
```

The script must create this directory when absent. It must not use `sudo`
automatically. If the user supplies a directory that is not writable, the
script must fail with instructions instead of escalating privileges.

Supported arguments:

```text
--version <vMAJOR.MINOR.PATCH>
--bin-dir <directory>
--help
```

Arguments for piped execution must work in this form:

```sh
curl -fsSL https://raw.githubusercontent.com/jorgeluis594/happy-memory/main/install.sh \
  | sh -s -- --version v0.2.0
```

The installer may use `curl` or `wget` for downloads, but must emit an
actionable error if neither is available. For checksum verification it must
support the standard utility available on the host, such as `sha256sum` on
Linux or `shasum -a 256` on macOS.

## 10. Windows installer contract

The repository must provide `install.ps1`, compatible with the supported
Windows PowerShell environment. The documented default invocation is:

```powershell
irm https://raw.githubusercontent.com/jorgeluis594/happy-memory/main/install.ps1 | iex
```

The script must:

1. Detect `amd64` or `arm64` without relying on Go.
2. Resolve the latest stable version when no version is supplied.
3. Download the matching `.zip` and `checksums.txt` over HTTPS.
4. Verify SHA-256 using native PowerShell capabilities.
5. Extract only `happy-memory.exe` into a temporary directory.
6. Install atomically into the target directory.
7. Run `happy-memory.exe version` and require success.
8. Clean up temporary files on success and failure.
9. Print the installed version, path, and `PATH` status.

The default target directory is:

```text
$HOME\AppData\Local\Programs\happy-memory\bin
```

The installer must not require administrator privileges. If the default or
selected directory is not already in the user's `PATH`, the installer must add
it once to the user-level `PATH`; it must never modify the machine-level
`PATH`. It must report that a new terminal is required for the change to take
effect. Reinstallation must not duplicate the entry.

Supported parameters:

```text
-Version <vMAJOR.MINOR.PATCH>
-BinDir <directory>
-Help
```

Because parameters are inconvenient through `iex`, explicit versions must be
documented using a downloaded script:

```powershell
irm https://raw.githubusercontent.com/jorgeluis594/happy-memory/main/install.ps1 -OutFile install.ps1
.\install.ps1 -Version v0.2.0
```

## 11. Upgrade and idempotency behavior

- Reinstalling the same version must succeed without producing duplicate
  files or repeated `PATH` entries.
- Installing a newer or explicitly requested older version must replace only
  the managed executable.
- The existing executable must remain usable until the new archive passes
  download, checksum, extraction, and basic execution validation.
- A failed installation must not leave a partial executable at the target
  path.
- The installer must not delete user data, repository configuration, or the
  SQLite database during installation, upgrade, or downgrade.

## 12. Security requirements

- All downloads must use HTTPS and fail on non-successful HTTP responses.
- Checksums must be downloaded from the same immutable GitHub Release as the
  archive.
- The expected checksum entry must match the exact artifact filename; partial
  or ambiguous filename matches are forbidden.
- Extraction must reject archives that do not contain the expected executable
  or that attempt path traversal.
- Temporary directories must be unique and restricted to the current user
  where the operating system permits it.
- User-supplied versions and paths must be handled as data and must not be
  evaluated as shell or PowerShell code.
- Logs must not print tokens, credentials, or sensitive environment values.
- Published instructions must offer a review-first alternative to direct
  remote execution.

Review-first Unix installation:

```sh
curl -fsSLO https://raw.githubusercontent.com/jorgeluis594/happy-memory/main/install.sh
less install.sh
sh install.sh
```

## 13. Errors and user experience

Installer output must be concise and identify the current phase: platform
detection, download, verification, extraction, installation, and validation.

Every failure must:

- Exit nonzero.
- State what failed.
- Avoid claiming that installation succeeded.
- Suggest a specific corrective action when possible.
- Preserve the previously installed executable.

Expected explicit errors include unsupported platform, unavailable release,
missing downloader, failed HTTP request, missing checksum, checksum mismatch,
malformed archive, unwritable target directory, and failed post-installation
validation.

## 14. Testing requirements

Automated validation must cover:

- GoReleaser configuration validation.
- Snapshot builds for every supported target.
- Archive and executable naming.
- Presence and correctness of checksum entries.
- Version metadata injection and development fallbacks.
- Unix OS and architecture mapping.
- Windows architecture mapping.
- Latest stable and explicit version resolution.
- Custom installation directories containing spaces.
- Missing download or checksum utilities.
- Checksum mismatch and malformed archives.
- Unsupported hosts.
- Idempotent reinstall and replacement of an existing binary.
- Cleanup and preservation of an existing installation after failure.
- User-level `PATH` handling without duplicate entries.

Installer tests must use temporary directories and controlled release fixtures;
they must not overwrite a developer's actual installation or depend on the
latest public GitHub Release.

Before publishing the first production release, manual smoke tests must be
completed on macOS ARM64, Linux AMD64, and Windows AMD64. Other published
targets require either native CI execution or an explicitly documented
equivalent validation.

## 15. Documentation requirements

The project README must document:

- The one-line installer for macOS/Linux.
- The one-line installer for Windows.
- The safer review-first installation flow.
- Supported platforms.
- Installing an explicit version.
- Selecting a custom installation directory.
- Adding the default directory to `PATH` when necessary.
- Verifying installation with `happy-memory version`.
- Manual archive installation as a fallback.
- Uninstallation by removing the executable and, on Windows, the optional
  user-level `PATH` entry.

Manual installation instructions must explain that users extract the archive
for their platform and copy the executable into a directory present in their
`PATH`. Go must not appear as an end-user prerequisite.

## 16. Acceptance criteria

The feature is complete when:

1. Pushing a valid version tag after successful checks creates a GitHub Release
   containing every supported archive and `checksums.txt`.
2. None of the published executables requires a local Go installation.
3. `curl ... | sh` installs the latest stable version on supported macOS and
   Linux hosts into `~/.local/bin` without `sudo`.
4. `irm ... | iex` installs the latest stable Windows version into the
   user-owned default directory without administrator privileges.
5. Explicit versions and custom installation directories work on both
   installer families.
6. Both installers reject a modified or mismatched artifact.
7. A failed upgrade preserves the existing executable.
8. The installed binary successfully runs `happy-memory version` and reports
   the release metadata.
9. Pull requests validate release configuration without publishing artifacts.
10. The README contains the supported installation and removal workflows.

## 17. Future extensions

After the base workflow is stable, separate changes may add:

- A custom short installation URL such as
  `https://happy-memory.dev/install.sh`.
- Homebrew and Scoop manifests generated from the release.
- WinGet, APT, RPM, or Arch Linux packaging.
- macOS and Windows code signing.
- Provenance attestations and cryptographic artifact signatures.
- A dedicated self-update command with an explicit user action.

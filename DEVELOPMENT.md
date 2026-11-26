# Developing ssm-helper

For using the app, see [README.md](README.md) ([Português](README.pt-BR.md)). This file is for whoever changes the code or publishes releases.

The app is written in Go with [Wails v2](https://wails.io), and the interface in React + TypeScript (`frontend/`).

## Setup

Requirements: Go 1.26+, Node 20+ and the Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0`).

```sh
make test   # go vet, go test, message translations and frontend type checking (same as CI)
make build  # builds build/bin/ssm-helper.exe (also from Linux/WSL)
make dev    # wails dev
```

To run the `.exe` with another folder for the environments, `ssm-helper.json` and `ssm-helper-data`, set `SSM_HELPER_DIR`. In WSL:

```sh
SSM_HELPER_DIR="$PWD" WSLENV="SSM_HELPER_DIR/p" ./build/bin/ssm-helper.exe
```

After changing a bound `App` method or its types, run `wails generate module` and commit `frontend/wailsjs`. It also flips `frontend/wailsjs/runtime/*` to mode 755; revert that with `git checkout frontend/wailsjs/runtime`.

## Code map

| Package | Responsibility |
|---|---|
| `app.go`, `update.go`, `troubleshooting.go` | Bindings used by the frontend |
| `about.go` | Data of the About screen (version from the embedded `wails.json`, commit from `-ldflags`) |
| `main.go` | Window setup, single instance lock, start after an update |
| `internal/env` | Reading, validating and writing environments, listing them and converting old files |
| `internal/settings` | `ssm-helper.json`: selected environment, language, screen options, update check and palettes |
| `internal/msg` | Messages to the user as codes (see [Internationalization](#internationalization)) |
| `internal/jsonfile` | Reading and writing JSON while keeping key order |
| `internal/sso` | SSO login (device authorization) and role credentials |
| `internal/tunnel` | SSM session. The [Session Manager Plugin](https://github.com/aws/session-manager-plugin) is compiled into the executable; each tunnel runs as a child process of the `.exe` itself (`--ssm-session-plugin`), because the plugin exits the process when the session ends |
| `internal/chrome` | Opening Chrome with `--host-rules`, tracking its window and clearing a profile |
| `internal/applog` | Daily log files, with each message's code, parameters and detail |
| `internal/troubleshoot` | The troubleshooting file, with the sensitive values masked |
| `internal/update` | Looking for a newer release, checking its signature and checksum, replacing the executable |
| `tools/updatekey` | Generating the update signing keys and signing `SHA256SUMS.txt` in the release workflow |

## Releases

`.github/workflows/release.yml` builds, signs and attaches the `.exe` when a release is published. Every version goes through a prerelease first, because promoting a release is what sends the update to every installed copy:

1. **Publish as a prerelease**: `gh release create vX.Y.Z --target main --prerelease --title vX.Y.Z --notes "..."`. The app ignores prereleases, so nobody gets it yet.
2. **Approve the signing**: the **Build for Windows** job runs the same checks as `make test` and builds. Then **Sign and attach** waits for approval (**Actions → the run → Review deployments**). Approve only runs you started, for the tag you expect.
3. **Test**: download `ssm-helper.exe` from the prerelease and check it.
4. **Promote**: `gh release edit vX.Y.Z --prerelease=false --latest`, or **Edit** on the release page. Copies on an earlier version see the update within 12 hours, or right away with **Check now**.

The version in the `.exe` properties and on the About screen comes from the tag (`v1.2.0` becomes `1.2.0`): the workflow writes it into `wails.json` before building. Local builds show the version in the file (`1.0.0`) and never look for updates; only release builds carry `-X main.channel=release`. `wails build` passes `-buildvcs=false`, so the commit goes in through `-X main.commit=...` (`Makefile` and the workflow).

The workflow can also be run by hand (**Actions → Release → Run workflow**) for an existing tag: the build is kept as a run artifact and, with **upload** checked, replaces the release files. A tag from before `tools/updatekey` existed fails at the signing step.

## Signing releases

The app only installs an update whose `SHA256SUMS.txt` is signed, together with the release tag, with one of the two ed25519 public keys in `internal/update/keys.go`: the **primary** and the **backup**. The tag in the signature keeps the files of an older release from being published again under a newer tag. `make test` fails while the keys are missing.

The build job has no secrets and only read access. **Sign and attach** runs in the `release` environment, which holds the `UPDATE_SIGNING_KEY` secret, accepts only `v*` tags and `main`, and requires a reviewer. It checks the build against its checksums, signs, and verifies the signature against `keys.go` before attaching, so a wrong secret fails the release instead of publishing an update nobody can install.

Setting up, once:

1. In your own terminal (not inside an AI assistant or anywhere the output is recorded), outside the repository:

   ```sh
   go run ./tools/updatekey generate -out ~/ssm-helper-primary
   go run ./tools/updatekey generate -out ~/ssm-helper-backup
   ```

2. Store both `.key` files in a password manager, in separate items. The backup lives **only** there.
3. **Settings → Environments → New environment** `release`: **Required reviewers** with yourself; **Deployment branches and tags** with a tag rule `v*` and a branch rule `main`.
4. `gh secret set UPDATE_SIGNING_KEY --env release --repo felipeolliveira/ssm-helper < ~/ssm-helper-primary.key`, then delete the `.key` files.
5. Put the two `.pub` contents in `internal/update/keys.go`.

To check a key before deleting it: `UPDATE_SIGNING_KEY="$(cat key)" go run ./tools/updatekey sign -version v0.0.0 file && go run ./tools/updatekey verify -version v0.0.0 file`.

| Situation | What to do |
|---|---|
| Primary lost | Set the backup as `UPDATE_SIGNING_KEY`, generate a new primary, put its public key first in `keys.go` and release. Installed copies accept it (signed with the backup) and get the new primary |
| Primary leaked | Same, removing the leaked public key from `keys.go`, right away: until each copy updates, it still accepts the leaked key |
| Both lost | Generate new keys. Installed copies reject the next releases, and everyone downloads one version by hand. Tell them through the usual channels: the app never tells users to download a version whose signature failed |

## Internationalization

The backend never sends text to the user. Everything it communicates (errors, tunnel states, Log lines, validation problems) is an `msg.Message`: a **code** (`TUNNEL_PORT_IN_USE`), **parameters** (`{"port": "8443"}`) and, when there is one, a raw technical **detail** such as the AWS error. Binding errors reach the frontend as that message's JSON.

The frontend translates with `frontend/src/i18n/locales/pt-BR.json` and `en.json`: `ui` holds the screen texts and `messages` one text per backend code, with parameters as `{name}`. `useI18n()` gives `t('targets.connect')` and `tm(message)`; the detail is appended unless the text places it with `{detail}`. Edit these files by hand: rewriting them with `JSON.stringify` changes the formatting.

- **New backend message**: constant in `internal/msg/codes.go`, `msg.New`/`msg.Err`/`msg.Wrap`, and a text under `messages` in **every** JSON. `make test` fails when a code has no translation.
- **New screen text**: key under `ui` in every JSON. Type checking fails when one misses it.
- **New language**: copy `pt-BR.json` under the language code, translate it, register it in `frontend/src/i18n/index.tsx` (`Lang`, `dictionaries`, `LANGUAGES`) and in `settings.Languages`, and add its SVG flag to `frontend/src/components/Flags.tsx` (Windows shows flag emojis as letters).

## Documentation

`README.md` and `README.pt-BR.md` keep the same sections in the same order; update both when behavior changes. Screenshots live in `docs/screenshots/<language>/`, taken with privacy mode on and the data from `env.json.example`.

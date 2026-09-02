# sv

SSH host manager for the terminal. One Go binary, hosts in a YAML file, the same hosts mirrored into `~/.ssh/config`.

![sv demo](assets/demo.gif)

I used Termius for years, then moved my whole workflow into the terminal and got tired of opening a desktop app to pick a server. `sv` keeps the part of Termius I used every day: a list of hosts with groups and tags, one command to connect, passwords in the keychain. Everything else is the system `ssh`.

- Hosts live in `~/.config/sv/hosts.yaml`. Edit it by hand or through the CLI, both work.
- Every host is written into a managed block of `~/.ssh/config`, so `ssh coolify` works in any tool that has never heard of `sv`.
- Key, ssh-agent and password auth. Passwords go to macOS Keychain or Secret Service, no `sshpass`.
- No SSH implementation of its own. `sv` builds the arguments and replaces itself with `ssh`, so signals, pty and resize are handled by ssh.
- `sv ls --json` for scripts and AI agents.

## Install

Homebrew:

```sh
brew install kianurivzzz/tap/sv
```

Go:

```sh
go install github.com/kianurivzzz/save-serve-cli/cmd/sv@latest
```

Or grab a binary from [Releases](https://github.com/kianurivzzz/save-serve-cli/releases). macOS and Linux only.

Password auth needs OpenSSH 8.4 or newer. `sv` checks `ssh -V` before connecting to a password host and tells you if it is too old.

## Quick start

```sh
sv import ssh-config                      # pick up what you already have in ~/.ssh/config
sv add coolify root@1.2.3.4 --group home --tag docker
sv add cashcow root@cashcow.example.com --password   # asks once, saves to the keychain
sv                                        # interactive list
sv coolify                                # connect
sv coolify docker ps                      # run a command and exit
```

## Commands

| Command | Notes |
|---|---|
| `sv` | Interactive list. Type to filter, `↑↓` or `ctrl+j`/`ctrl+k` to move, `tab` to cycle groups, `enter` to connect, `ctrl+e` to edit the YAML, `esc` to quit. |
| `sv <name> [command...]` | Connect. Fuzzy match on the name. Several matches open the list with the filter prefilled. Anything after the name runs remotely, like `ssh host cmd`. Use `--` if the command starts with a dash. |
| `sv add <name> [user@]host[:port]` | Flags: `--user`, `--port`, `--key <path>`, `--agent`, `--password`, `--group`, `--tag` (repeatable), `--jump <name>`, `--note`, `--store keychain\|file`. |
| `sv ls [--group g] [--tag t] [--json]` | Table sorted by group, then by last use. |
| `sv rm <name> [-y]` | Removes the host and its stored password. Refuses if another host uses it as a jump. |
| `sv edit [name]` | Opens `hosts.yaml` in `$EDITOR`, jumps to the host, validates and syncs on exit. |
| `sv group ls \| add \| rm \| mv` | Groups and colors. `sv group mv <host> <group>` moves a host. |
| `sv passwd <name>` | Stores or replaces a password and switches the host to `auth: password`. |
| `sv import ssh-config [path]` | Imports every `Host` without wildcards into group `imported`. |
| `sv import termius <file.csv>` | Imports a Termius CSV export, see below. |
| `sv sync` | Rebuilds the block in `~/.ssh/config`. Runs on its own after every change. |
| `sv completion <shell>` | Completion for bash, zsh, fish with host names. |

If a name is not an exact match, `sv` tries substring first, then subsequence: `sv cc` finds `CashCow` when nothing else fits.

## hosts.yaml

```yaml
version: 1
defaults:
  user: root
  port: 22
  key: ~/.ssh/id_ed25519

groups:
  - name: home
    color: green
  - name: work
    color: orange

hosts:
  - name: coolify
    host: 1.2.3.4
    auth: key
    group: home
    tags: [docker, prod]
    note: side projects

  - name: db-primary
    host: 10.1.0.5
    user: postgres
    port: 2222
    group: work
    jump: bastion

  - name: cashcow
    host: cashcow.example.com
    auth: password
    group: work
```

- `name` is case-insensitive, must not contain spaces, and doubles as the ssh alias.
- `auth` is `key`, `agent` or `password`. When omitted, `sv` uses `key` if `defaults.key` exists on disk, otherwise `agent`.
- `jump` names another host from the file and becomes `ProxyJump`.
- Group `color` is a name (`green`, `orange`, `purple`, ...), an ANSI code `0`–`255` or `#rrggbb`. Groups without a color get one from a small palette.
- The file is created with mode `0600`, the directory with `0700`. `SV_CONFIG` overrides the path.
- `state.json` next to it records when each host was last used. It is written before `ssh` starts, so a failed connection counts too.

## What lands in ~/.ssh/config

```
# >>> sv managed - do not edit, changes will be overwritten >>>
Host coolify
    HostName 1.2.3.4
    User root
    Port 22
    IdentityFile ~/.ssh/id_ed25519
    IdentitiesOnly yes

Host cashcow
    HostName cashcow.example.com
    User root
    Port 22
# <<< sv managed <<<
```

Only this block is touched, the rest of the file is left alone. The first write saves a copy as `~/.ssh/config.sv-backup`. Password hosts are listed too, so `ssh cashcow` works with a manual password prompt.

`sv` does not wrap ssh options. Port forwarding, `-o` and friends go through the alias: `ssh -L 8080:localhost:80 coolify`.

## Passwords

`sv` points `SSH_ASKPASS` at a two-line script that prints `$SV_PASS`, sets `SSH_ASKPASS_REQUIRE=force`, and execs `ssh`. The password exists only in the environment of that ssh process. The script has no secrets in it and lives at `~/.config/sv/askpass.sh`.

Storage is macOS Keychain or Secret Service on Linux, service `sv`, account `<host name>`. When neither is available, on a server or in WSL for example, passwords go to `~/.config/sv/secrets.yaml` with mode `0600`, and `sv` warns once that the file is plain text. Force a backend with `defaults.secret_store: keychain` or `file` in the YAML, or `--store` on `add`, `passwd` and `import termius`.

## Importing from Termius

The file needs the columns of the Termius CSV template:

```
Groups,Label,Tags,Hostname/IP,Protocol,Port,Username,Password
```

```sh
sv import termius ~/Downloads/hosts.csv
```

- Groups are kept as they are. A nested path like `work/db` becomes one group named `work/db`, rename it with `sv group` if you like. Hosts without a group go to `imported`.
- Rows with a password become `auth: password`, the password is stored in the keychain.
- Labels with spaces or `* ? !` are cleaned up: `db primary` turns into `db-primary`. Duplicate labels get `-2`, `-3` suffixes.
- Rows with a protocol other than `ssh` are skipped with a warning.
- Existing hosts are skipped unless you pass `--overwrite`.

If your Termius build has no CSV export, [termius-data-exporter](https://github.com/LXiuu/termius-data-exporter) writes the same columns from the local database.

## Scripts and agents

```sh
sv ls --json | jq -r '.[] | select(.group == "work") | .name'
sv coolify -- df -h
```

The JSON carries resolved values, so `user`, `port`, `auth` and `key` already include the defaults. `last_used` is RFC 3339 or `null`.

```json
[
  {
    "name": "coolify",
    "host": "1.2.3.4",
    "user": "root",
    "port": 22,
    "auth": "key",
    "key": "~/.ssh/id_ed25519",
    "group": "home",
    "tags": ["docker", "prod"],
    "jump": "",
    "note": "side projects",
    "last_used": "2026-09-02T10:14:03Z"
  }
]
```

When stdout is not a terminal, plain `sv` prints the table instead of opening the list.

## Environment

| Variable | Effect |
|---|---|
| `SV_CONFIG` | Path to `hosts.yaml` |
| `EDITOR`, `VISUAL` | Editor for `sv edit` and `ctrl+e` |
| `NO_COLOR` | Disables colors and bold everywhere |

## Development

```sh
go build -o sv ./cmd/sv
go test ./...
```

Re-record the demo with [vhs](https://github.com/charmbracelet/vhs):

```sh
PATH="$PWD:$PATH" vhs assets/demo.tape
```

Releases are cut by pushing a tag: `git tag v0.1.0 && git push origin v0.1.0`. GitHub Actions runs goreleaser, which builds darwin/linux binaries, publishes them to Releases and updates the cask in `kianurivzzz/homebrew-tap`. The workflow needs a `HOMEBREW_TAP_TOKEN` secret with write access to the tap repository.

## License

MIT

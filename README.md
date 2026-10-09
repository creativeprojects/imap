# imap tools

[![Build](https://github.com/creativeprojects/imap/actions/workflows/build.yaml/badge.svg)](https://github.com/creativeprojects/imap/actions/workflows/build.yaml)
[![codecov](https://codecov.io/gh/creativeprojects/imap/branch/main/graph/badge.svg?token=3LGb0PvATl)](https://codecov.io/gh/creativeprojects/imap)

Backup, copy and move your emails between IMAP servers, or between an IMAP server and your local machine.

A single `imap` binary, with no runtime dependencies, that can:

* copy every mailbox of an account to another account, **incrementally**: rerunning the same copy only transfers new messages
* back up an IMAP account to a local Maildir or to a single compressed database file
* restore a local backup to an IMAP server
* list mailboxes and message counts
* find duplicate messages in an account
* update itself from GitHub releases

## Table of contents

- [Supported backends](#supported-backends)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Configuration file](#configuration-file)
- [Commands](#commands)
- [How the incremental copy works](#how-the-incremental-copy-works)
- [Running in Docker](#running-in-docker)
- [Development](#development)

## Supported backends

Every account in the configuration file has a `type`:

| Type      | Description | Notes |
|-----------|-------------|-------|
| `imap`    | Remote IMAP server over TLS | Uses the `UIDPLUS` extension when the server supports it |
| `maildir` | [Maildir](https://en.wikipedia.org/wiki/Maildir) folder on disk | **Not supported on Windows** |
| `local`   | Single database file of compressed emails ([bbolt](https://github.com/etcd-io/bbolt)) | Works on all platforms |

Any backend can be used as a source or a destination, so you can copy IMAP to IMAP, IMAP to local, local to IMAP, Maildir to local, and so on.

## Installation

### Pre-built binaries

Download the archive for your platform from the [releases page](https://github.com/creativeprojects/imap/releases) and put the `imap` binary somewhere in your `PATH`.

Binaries are built for Linux, macOS, Windows and FreeBSD on `amd64`, `386`, `arm` and `arm64` (not every combination is available, see the release assets).

Once installed, you can keep it up to date with:

```shell
imap selfupdate
```

### Docker

Images are published for `linux/amd64` and `linux/arm64` on both Docker Hub and the GitHub Container Registry:

* `creativeprojects/imap`
* `ghcr.io/creativeprojects/imap`

See [Running in Docker](#running-in-docker) for usage.

### From source

Requires Go 1.27 or later.

```shell
go install github.com/creativeprojects/imap@latest
```

Or clone the repository and build with `make`:

```shell
git clone https://github.com/creativeprojects/imap.git
cd imap
make build
```

## Quick start

1. Create a file named `imap.yaml` in the current directory describing your accounts (see [Configuration file](#configuration-file)):

   ```yaml
   accounts:
     work:
       type: imap
       serverURL: imap.example.com:993
       username: me@example.com
       password: secret

     backup:
       type: local
       file: ./work-backup.db
   ```

2. Check that the connection works and see what is in the account:

   ```shell
   imap list work
   ```

3. Copy everything from the IMAP account to the local backup:

   ```shell
   imap copy work backup
   ```

4. Run the same command again later: only messages received since the last copy are transferred.

   ```shell
   imap copy work backup
   ```

## Configuration file

The configuration is a YAML file. By default `imap` looks for `imap.yaml` in the current directory; use `--config` (or `-c`) to point to another file.

```yaml
---
accounts:

  imap-user:
    type: imap
    serverURL: localhost:993
    username: user@example.com
    password: pass
    skipTLSverification: true

  maildir-test:
    type: maildir
    root: ./maildir-test

  local-test:
    type: local
    file: ./local/test.db
```

Each key under `accounts` is the name you will use on the command line.

### `imap` account

| Field | Required | Description |
|-------|----------|-------------|
| `serverURL` | yes | Host and port of the server, for example `imap.example.com:993`. The connection always uses TLS (minimum TLS 1.2), so use the implicit TLS port. |
| `username` | yes | Login name |
| `password` | yes | Password |
| `skipTLSverification` | no | Set to `true` to accept a self-signed or otherwise invalid certificate. Defaults to `false`. |

### `maildir` account

| Field | Required | Description |
|-------|----------|-------------|
| `root` | yes | Directory containing the Maildir. It is created if it does not exist. |

### `local` account

| Field | Required | Description |
|-------|----------|-------------|
| `file` | yes | Path to the database file. It is created if it does not exist. |

## Commands

```
imap [command] [flags]
```

### Global flags

| Flag | Default | Description |
|------|---------|-------------|
| `-c`, `--config` | `imap.yaml` | Configuration file |
| `-q`, `--quiet` | | Only display warnings and errors |
| `-v`, `--verbose` | | Display debugging information, including the IMAP conversation |

### `list`

Display the mailboxes of an account and the number of messages in each.

```shell
imap list <account>
```

### `copy`

Copy all mailboxes and messages from one account to another. Mailboxes are created on the destination if they do not exist. The copy is incremental: see [How the incremental copy works](#how-the-incremental-copy-works).

```shell
imap copy <source account> <destination account>
```

Message flags (seen, flagged, answered, etc.) and the internal date of each message are preserved.

### `history`

Display the history of copy operations recorded on an account, mailbox by mailbox.

```shell
imap history <account>
```

With `--verbose`, it also shows the date from which the next copy will start for each source account.

### `duplicates`

Read every mailbox of an account and report messages that appear more than once (across all mailboxes). Messages are compared by a hash of their content.

```shell
imap duplicates <account>
```

This command only reports duplicates, it does not delete anything. On an `imap` account, every message is downloaded to compute its hash, so it can take a while on a large account.

### `selfupdate`

Download the latest release from GitHub and replace the current binary.

```shell
imap selfupdate
```

## How the incremental copy works

### History

Each `copy` records which messages were copied in a **history** saved on the **destination** account. The history associates the message IDs of the source with the message IDs created on the destination.

On the next run, the copy:

1. only fetches messages from the source with an internal date later than the latest message in the history
2. skips any message whose source ID is already in the history

Where the history is stored depends on the destination backend:

| Destination | History location |
|-------------|------------------|
| `local` | Inside the database file |
| `maildir` | A file `<mailbox name>.history.json` in the Maildir root |
| `imap` | A folder `.cache/<account ID>/` in the current working directory, one `<mailbox name>.history.json` file per mailbox |

**If you delete the history, the next copy starts from scratch and all messages are copied again.**

Note that when the destination is an IMAP server, the history lives on the machine running `imap`, not on the server. Run the copy from the same working directory every time to keep it incremental.

### Copying from multiple sources

Each account is given an account ID, which is recorded in the history so that several sources can be copied into the same destination. How the ID is generated depends on the backend:

| Backend | Account ID |
|---------|------------|
| `local` | Randomly generated when the database is created and stored inside it |
| `maildir` | Randomly generated when the Maildir is created and stored in `.account.metadata.json` in the Maildir root |
| `imap` | Derived from the server URL and the login name (not stored anywhere) |

### Resuming after an error

If the connection to an IMAP server is lost in the middle of a copy, the messages copied so far are saved in the history. Rerun the `copy` command and it resumes from where it stopped.

## Running in Docker

The image has `imap` as its entrypoint and uses `/imap` as its working directory. Mount a local directory there containing your `imap.yaml`; the same directory will receive local backups, Maildirs and the `.cache` folder for IMAP history.

```shell
docker run --rm -v "$PWD:/imap" creativeprojects/imap list work
docker run --rm -v "$PWD:/imap" creativeprojects/imap copy work backup
```

Any path in the configuration file should be relative to `/imap` (or simply relative, like `./backup.db`).

## Development

### Build and test

```shell
make build      # build the imap binary
make test       # run the test suite
make coverage   # run the tests and open the coverage report
make lint       # run golangci-lint for darwin, linux and windows
```

### Local IMAP server for testing

A [Dovecot](https://www.dovecot.org/) image is provided for testing against a real IMAP server:

```shell
make dovecot
```

This builds the image and starts a container listening on ports `143` and `993`. Any username is accepted with the password `pass`, and the server uses a self-signed certificate, so set `skipTLSverification: true` in your configuration:

```yaml
accounts:
  dovecot:
    type: imap
    serverURL: localhost:993
    username: test
    password: pass
    skipTLSverification: true
```

### Project layout

| Directory | Content |
|-----------|---------|
| `cmd/` | Command-line interface (one file per command) |
| `cfg/` | Configuration file loading |
| `storage/` | The `Backend` interface and its implementations: `remote` (IMAP), `mdir` (Maildir), `local` (bbolt) and `mem` (in-memory, for tests) |
| `mailbox/` | Shared types: messages, properties, history |
| `lib/` | Helpers: account tags, flags, UID handling, test email generator |
| `dovecot/` | Dockerfile for the test IMAP server |

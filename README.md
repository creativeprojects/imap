# imap tools

[![codecov](https://codecov.io/gh/creativeprojects/imap/branch/main/graph/badge.svg?token=3LGb0PvATl)](https://codecov.io/gh/creativeprojects/imap)


Backup, copy, move your emails from and to IMAP servers. It can also load and save your emails locally.

## backend supported:

* IMAP
* [Maildir](https://en.wikipedia.org/wiki/Maildir) (**not** for Windows)
* Local database of compressed emails (boltDB)

## commands implemented:

* `list`: list mailboxes from the account
* `copy`: copy all messages from one account to another one (incremental copy)
* `history`: see an history of the actions on the account (only `copy` for now)
* `selfupdate`: update automatically to the newest version from Github releases

## keeping history for the incremental copy

The copy command will save a history of messages copied from the source. The history is saved on the destination backend. This is needed to associate the messages IDs of the source with the destination.

The way the history is saved is different for each backend:
* local: the history is saved in the database file
* Maildir: the history is saved in a file `<mailbox name>.history.json`
* imap: the history is saved in a folder `.cache`

The incremental copy will break if you delete the history: all messages will be copied again.

## copying from multiple sources while keeping history

Each account is given an account ID so we can reference it in the history. The way this ID is generated depends on the backend:
* local: randomly generated at creation and saved in the database file
* Maildir: randomly generated at creation and saved in a file `.account.metadata.json`
* imap: generated from the server URL and login name (not saved anywhere)

## restart after error

After a connection to an IMAP server is lost, the current copy is saved in the history. It should restart from where it stopped if you rerun the `copy` command.

## configuration file

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

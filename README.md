# imap tools

[![codecov](https://codecov.io/gh/creativeprojects/imap/branch/main/graph/badge.svg?token=3LGb0PvATl)](https://codecov.io/gh/creativeprojects/imap)


Backup, copy, move your emails from and to IMAP servers. It can also load and save your emails locally.

## backend supported:

* IMAP
* [Maildir](https://en.wikipedia.org/wiki/Maildir) (**not** for Windows)
* Local database of compressed emails (boltDB)

## commands implemented:

* list (mailboxes)
* copy all message from one backend to another one

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

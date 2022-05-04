package cmd

import (
	"fmt"

	"github.com/creativeprojects/imap/cfg"
	"github.com/creativeprojects/imap/lib"
	"github.com/creativeprojects/imap/storage"
	"github.com/creativeprojects/imap/storage/local"
	"github.com/creativeprojects/imap/storage/mdir"
	"github.com/creativeprojects/imap/storage/remote"
)

func NewBackend(config cfg.Account, logger lib.Logger) (storage.Backend, error) {
	switch config.Type {
	case cfg.IMAP:
		return remote.NewImap(remote.Config{
			ServerURL:           config.ServerURL,
			Username:            config.Username,
			Password:            config.Password,
			SkipTLSVerification: config.SkipTLSVerification,
		})
	case cfg.LOCAL:
		return local.NewBoltStoreWithLogger(config.File, logger)
	case cfg.MAILDIR:
		return mdir.NewWithLogger(config.Root, logger)
	default:
		return nil, fmt.Errorf("unsupported account type %q", config.Type)
	}
}

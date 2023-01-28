package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/creativeprojects/go-selfupdate"
	"github.com/spf13/cobra"
)

var selfUpdateCmd = &cobra.Command{
	Use:   "selfupdate",
	Short: "Download newest release from Github and update",
	RunE:  runSelfUpdate,
}

var (
	appVersion = ""
	appCommit  = ""
	appDate    = ""
	appBuiltBy = ""
)

func init() {
	rootCmd.AddCommand(selfUpdateCmd)
}

func setApp(version, commit, date, builtBy string) {
	appVersion = version
	appCommit = commit
	appDate = date
	appBuiltBy = builtBy
}

func runSelfUpdate(cmd *cobra.Command, args []string) error {
	if global.verbose {
		selfupdate.SetLogger(log.Default())
	}
	// only filters return an error
	updater, _ := selfupdate.NewUpdater(selfupdate.Config{
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
	})

	latest, found, err := detectLatest(updater)

	if err != nil {
		return fmt.Errorf("unable to detect latest version: %w", err)
	}
	if !found {
		return fmt.Errorf("latest version for %s/%s could not be found from github repository", runtime.GOOS, runtime.GOARCH)
	}
	if latest.LessOrEqual(appVersion) {
		fmt.Printf("Current version (%s) is the latest\n", appVersion)
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return errors.New("could not locate executable path")
	}
	if err := updater.UpdateTo(context.Background(), latest, exe); err != nil {
		return fmt.Errorf("unable to update binary: %w", err)
	}
	fmt.Printf("Successfully updated to version %s", latest.Version())
	return nil
}

func detectLatest(updater *selfupdate.Updater) (*selfupdate.Release, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return updater.DetectLatest(ctx, selfupdate.NewRepositorySlug("creativeprojects", "imap"))
}

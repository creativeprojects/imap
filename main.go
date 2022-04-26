package main

import (
	"log"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

func main() {
	cfg, err := LoadFileConfig("imap.yaml")
	if err != nil {
		log.Fatal(err)
	}

	for _, account := range cfg.Accounts {
		err := listMailboxes(account.ServerURL, account.Username, account.Password)
		if err != nil {
			log.Print(err)
		}
	}
}

func listMailboxes(serverURL, username, password string) error {
	log.Printf("Connecting to server %s...", serverURL)
	imapClient, err := client.DialTLS(serverURL, nil)
	if err != nil {
		return err
	}
	defer imapClient.Logout()
	log.Print("Connected")

	if err := imapClient.Login(username, password); err != nil {
		return err
	}
	log.Printf("Logged in as %s", username)

	capabilities, err := imapClient.Capability()
	if err != nil {
		return err
	}
	log.Printf("%v", capabilities)

	// List mailboxes
	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- imapClient.List("", "*", mailboxes)
	}()

	log.Println("Mailboxes:")
	for m := range mailboxes {
		log.Println("* " + m.Name)
	}

	if err := <-done; err != nil {
		return err
	}

	// Select INBOX
	mbox, err := imapClient.Select("INBOX", false)
	if err != nil {
		return err
	}
	log.Println("Flags for INBOX:", mbox.Flags)
	return nil
}

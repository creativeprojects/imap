package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-maildir"
)

func main() {
	wd, err := os.Getwd()
	if err == nil {
		wd = "./"
	}

	cfg, err := LoadFileConfig("imap.yaml")
	if err != nil {
		log.Fatal(err)
	}

	for _, account := range cfg.Accounts {
		// if account.Type == IMAP {
		// 	err := listMailboxes(account.ServerURL, account.Username, account.Password)
		// 	if err != nil {
		// 		log.Print(err)
		// 	}
		// }
		if account.Type == MAILDIR {
			box := maildir.Dir(filepath.Join(wd, account.Root))
			err := box.Init()
			if err != nil {
				log.Print(err)
			}
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

	// Get the last message
	// from := uint32(1)
	// to := mbox.Messages
	// if mbox.Messages > 3 {
	// 	// We're using unsigned integers here, only subtract if the result is > 0
	// 	from = mbox.Messages - 3
	// }
	seqset := new(imap.SeqSet)
	// seqset.AddRange(from, to)
	seqset.AddNum(mbox.Messages)

	messages := make(chan *imap.Message, 10)
	done = make(chan error, 1)
	go func() {
		done <- imapClient.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope}, messages)
	}()

	log.Println("Last message:")
	for msg := range messages {
		log.Println("* " + msg.Envelope.Subject)
	}

	if err := <-done; err != nil {
		log.Fatal(err)
	}

	return nil
}

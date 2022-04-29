package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/creativeprojects/clog"
	"github.com/creativeprojects/imap/store"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

func main() {
	log := clog.NewFilteredConsoleLogger(clog.LevelDebug)
	wd, err := os.Getwd()
	if err == nil {
		wd = "./"
	}

	cfg, err := LoadFileConfig("imap.yaml")
	if err != nil {
		log.Errorf("cannot open or read configuration file: %w", err)
		os.Exit(1)
	}

	for _, account := range cfg.Accounts {
		if account.Type == IMAP {
			// server, err := remote.NewImap(remote.Config{
			// 	Logger:    *log,
			// 	ServerURL: account.ServerURL,
			// 	Username:  account.Username,
			// 	Password:  account.Password,
			// })
			// if err != nil {
			// 	log.Error(err)
			// }
			// defer server.Close()
			// mailboxes, err := server.List()
			// if err != nil {
			// 	log.Error(err)
			// }
			// err = os.MkdirAll(filepath.Join(wd, ".cache"), 0755)
			// if err != nil {
			// 	log.Error(err)
			// 	break
			// }
			// db := store.NewBoltStore(filepath.Join(wd, ".cache", account.Username))
			// defer db.Close()
			// err = db.Init()
			// if err != nil {
			// 	log.Error(err)
			// 	break
			// }
			// for _, mailbox := range mailboxes {
			// 	err := db.CreateMailbox(mailbox)
			// 	if err != nil {
			// 		log.Error(err)
			// 		break
			// 	}
			// }
			log.Debugf("Mailboxes in account %q:", account.Username)
			db := store.NewBoltStore(filepath.Join(wd, ".cache", account.Username))
			defer db.Close()
			list, err := db.List()
			if err != nil {
				log.Error(err)
				break
			}
			for _, m := range list {
				log.Debugf("* %q: %+v (delimiter = %q)", m.Name, m.Attributes, m.Delimiter)
			}
		}

		// if account.Type == MAILDIR {
		// 	dir := filepath.Join(wd, account.Root, "testbox")
		// 	err := os.MkdirAll(dir, 0755)
		// 	if err != nil {
		// 		log.Print(err)
		// 		continue
		// 	}
		// 	box := maildir.Dir(dir)
		// 	err = box.Init()
		// 	if err != nil {
		// 		log.Print(err)
		// 		continue
		// 	}
		// 	key, w, err := box.Create([]maildir.Flag{maildir.FlagSeen})
		// 	if err != nil {
		// 		log.Print(err)
		// 		continue
		// 	}
		// 	log.Printf("new message key = %s", key)
		// 	w.Write([]byte("From: toto\nTo: toto\nSubject: message\n\nCoucou!\n"))
		// 	err = w.Close()
		// 	if err != nil {
		// 		log.Print(err)
		// 	}
		// 	err = box.SetInfo(key, "UID")
		// 	if err != nil {
		// 		log.Print(err)
		// 	}
		// }
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

	section := &imap.BodySectionName{Peek: true}
	items := []imap.FetchItem{section.FetchItem(), imap.FetchFlags, imap.FetchUid}

	messages := make(chan *imap.Message, 10)
	done = make(chan error, 1)
	go func() {
		done <- imapClient.Fetch(seqset, items, messages)
	}()

	log.Println("Last message:")
	for msg := range messages {
		// log.Println("* " + msg.Envelope.Subject)
		log.Println("-------------- Message Info --------------------")
		log.Printf("Flags: %+v, Seqnum: %d, Uid: %d", msg.Flags, msg.SeqNum, msg.Uid)
		// log.Println("-------------- Start of message --------------------")
		// r := msg.GetBody(section)
		// io.Copy(os.Stdout, r)
		// log.Println("----------- End of message -----------------------")
	}

	if err := <-done; err != nil {
		log.Fatal(err)
	}

	return nil
}

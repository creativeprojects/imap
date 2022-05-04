package lib

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

const (
	charset = "abcdefghijklmnopqrstuvwxyz \n" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ 0123456789 \n" +
		",./;'\\ \" []{}<>?:|!@£$%^&*()_+-= \n"

	template = "From: %s\n" +
		"To: %s\n" +
		"Subject: A little message, just for you\n" +
		"Date: Wed, 11 May 2016 14:31:59 +0000\n" +
		"Message-ID: <%d@localhost/>\n" +
		"Content-Type: text/plain\n" +
		"\n%s"
)

var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixMilli()))

func stringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func GenerateEmail(from, to string, uid uint32, minSize, maxSize int) []byte {
	length := seededRand.Intn(maxSize-minSize) + minSize
	msg := fmt.Sprintf(template, from, to, uid, stringWithCharset(length, charset))
	// emails are using CRLF line endings
	msg = strings.ReplaceAll(msg, "\n", "\r\n")
	return []byte(msg)
}

package lib

import (
	"fmt"
	"math/rand"
	"time"
)

const charset = "abcdefghijklmnopqrstuvwxyz " +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 " +
	",./;'\\ \" []{}<>?:|!@£$%^&*()_+-= " +
	"\r\n\r\n\r\n "

const template = "From: %s\r\n" +
	"To: %s\r\n" +
	"Subject: A little message, just for you\r\n" +
	"Date: Wed, 11 May 2016 14:31:59 +0000\r\n" +
	"Message-ID: <%d@localhost/>\r\n" +
	"Content-Type: text/plain\r\n" +
	"\r\n%s"

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixMilli()))

func stringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func GenerateEmail(from, to string, uid uint32) []byte {
	length := seededRand.Intn(300000)
	msg := fmt.Sprintf(template, from, to, uid, stringWithCharset(length, charset))
	return []byte(msg)
}

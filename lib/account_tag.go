package lib

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

func AccountTag(serverURL, username string) string {
	hasher := sha256.New()
	hasher.Write([]byte(username))
	hasher.Write([]byte(":"))
	hasher.Write([]byte(serverURL))
	hasher.Write([]byte("\n"))
	return hex.EncodeToString(hasher.Sum(nil))
}

func RandomTag(salt string) string {
	data := make([]byte, 30)
	_, _ = rand.Read(data)
	temp := sha256.Sum256(data)
	return hex.EncodeToString(temp[:])
}

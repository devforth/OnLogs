package userdb

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strconv"
	"strings"
)

const (
	hashScheme     = "pbkdf2_sha256"
	hashIterations = 600000
	hashKeyLength  = 32
	hashSaltLength = 16
)

func HashPassword(password string) string {
	salt := make([]byte, hashSaltLength)
	if _, err := rand.Read(salt); err != nil {
		panic(err)
	}
	return encodeHash(password, salt, hashIterations)
}

func encodeHash(password string, salt []byte, iterations int) string {
	key, err := pbkdf2.Key(sha256.New, password, salt, iterations, hashKeyLength)
	if err != nil {
		panic(err)
	}
	return strings.Join([]string{
		hashScheme,
		strconv.Itoa(iterations),
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	}, "$")
}

func verifyHash(stored string, password string) (ok bool, wasHashed bool) {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != hashScheme {
		return false, false
	}

	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false, true
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false, true
	}

	candidate := encodeHash(password, salt, iterations)
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(stored)) == 1, true
}

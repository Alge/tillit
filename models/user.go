package models

import (
	"bytes"
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"strings"

	"github.com/google/uuid"
)

// User represents a user with an ID, username, public key, and creation timestamp.

type User struct {
	ID          string       `json:"id"`
	Username    string       `json:"username"`
	PubKey      string       `json:"public_key"`
	Connections []Connection `json:"connections,omitempty"`
	IsAdmin     bool         `json:"is_admin,omitempty"`
}

func NewUser(username string, publicKey string) (*User, error) {

	u := &User{}
	u.Username = username
	u.PubKey = publicKey

	if id, err := uuid.NewRandom(); err != nil {
		return u, err
	} else {
		u.ID = id.String()
	}

	return u, nil
}

func (u User) String() string {
	return fmt.Sprintf("User Object. ID: %s, Username: %s, Connections: %d", u.ID, u.Username, len(u.Connections))
}

func (u *User) Connect(other *User, public bool, trust bool, trustExtends int) (c *Connection, err error) {
	c, err = NewConnection()
	if err != nil {
		return
	}

	c.Owner = u.ID
	c.OtherID = other.ID
	c.Public = public
	c.Trust = trust
	c.TrustExtends = trustExtends

	return
}

func (u *User) GetPublicKey() (*rsa.PublicKey, error) {

	parts := strings.Split(u.PubKey, " ")
	if len(parts) < 2 {
		return nil, errors.New("Invalid pubkey string")
	}

	log.Println("Decoding public key")
	log.Printf("Key type: '%s'", parts[0])
	log.Printf("Key Content: '%s'", parts[1])

	switch parts[0] {
	case "ssh-rsa":
		log.Println("Found RSA key")
	default:
		return nil, fmt.Errorf("Unknown key type: %s", parts[0])
	}

	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("Failed to b64-decode string: %w", err)
	}

	reader := bytes.NewReader(decoded)

	// Helper function to read MPInt
	readMPInt := func() (*big.Int, error) {
		var length uint32
		if err := binary.Read(reader, binary.BigEndian, &length); err != nil {
			return nil, err
		}
		intBytes := make([]byte, length)
		if _, err := io.ReadFull(reader, intBytes); err != nil {
			return nil, err
		}
		return new(big.Int).SetBytes(intBytes), nil
	}

	// Read exponent
	eBig, err := readMPInt()
	if err != nil {
		return nil, err
	}
	e := int(eBig.Int64())

	// Read modulus
	n, err := readMPInt()
	if err != nil {
		return nil, err
	}

	return &rsa.PublicKey{
		N: n,
		E: e,
	}, nil
}

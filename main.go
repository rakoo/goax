package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/openpgp/armor"

	"github.com/crowsonkb/base58"
	"github.com/pkg/errors"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Need an action: one of mykey, send or receive")
		os.Exit(1)
	}

	ensureIdentityKey()

	switch os.Args[1] {
	case "mykey":
		printPublicKey()
	case "send":
		if len(os.Args) < 3 {
			fmt.Println("Need email adress of recipient")
			os.Exit(1)
		}
		send(os.Args[2])
	case "receive":
		if len(os.Args) < 3 {
			fmt.Println("Need email adress of sender")
			os.Exit(1)
		}
		receive(os.Args[2])
	default:
		fmt.Println("Unrecognized action:", os.Args[1])
		fmt.Println("Need one of generate, send or receive")
		os.Exit(1)
	}
}

func ensureIdentityKey() {
	_, err := os.Open("key")
	if err != nil {
		f, err := os.Create("key")
		if err != nil {
			log.Fatal(errors.Wrap(err, "Couldn't create private identity key"))
		}
		encoder, err := armor.Encode(f, "GOAX PRIVATE KEY", nil)
		if err != nil {
			log.Fatal(errors.Wrap(err, "Couldn't create armored writer"))
		}
		_, err = io.CopyN(encoder, rand.Reader, 32)
		if err != nil {
			log.Fatal(errors.Wrap(err, "Couldn't write private key to file"))
		}
		err = encoder.Close()
		if err != nil {
			log.Fatal(errors.Wrap(err, "Couldn't close encoder"))
		}
		err = f.Close()
		if err != nil {
			log.Fatal(errors.Wrap(err, "Couldn't close file"))
		}
	}
}

func printPublicKey() {
	var myPublicKey [32]byte
	var myPrivateKey [32]byte
	copy(myPrivateKey[:], getPrivateKey())
	curve25519.ScalarBaseMult(&myPublicKey, &myPrivateKey)
	fmt.Println(base58.Encode(myPublicKey[:]))
}

func getPrivateKey() (pkey []byte) {
	f, err := os.Open("key")
	if err != nil {
		log.Fatal(errors.Wrap(err, "Error opening private key"))
	}
	defer f.Close()

	block, err := armor.Decode(f)
	if err != nil {
		log.Fatal(errors.Wrap(err, "Error decoding private key"))
	}
	private, err := ioutil.ReadAll(block.Body)
	if err != nil {
		log.Fatal(errors.Wrap(err, "Error decoding private key"))
	}
	return private
}

const (
	ENCRYPTED_MESSAGE_TYPE string = "GOAX ENCRYPTED MESSAGE"
	KEY_EXCHANGE_TYPE             = "KEY EXCHANGE MATERIAL"
)

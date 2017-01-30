package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"golang.org/x/crypto/openpgp/armor"

	"github.com/pkg/errors"
	"github.com/rakoo/goax"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Need an action: one of generate, send or receive")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "generate":
		ensureIdentityKey()
		printRatchetMaterial()
	case "send":
	case "receive":
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

func printRatchetMaterial() {
	f, err := os.Open("key")
	if err != nil {
		log.Fatal(errors.Wrap(err, "Error opening private key"))
	}
	block, err := armor.Decode(f)
	if err != nil {
		log.Fatal(errors.Wrap(err, "Error decoding private key"))
	}
	private, err := ioutil.ReadAll(block.Body)
	if err != nil {
		log.Fatal(errors.Wrap(err, "Error decoding private key"))
	}

	var privateArray [32]byte
	copy(privateArray[:], private)
	ratchet := goax.New(rand.Reader, privateArray)
	kx, err := ratchet.GetKeyExchangeMaterial()
	if err != nil {
		log.Fatal(errors.Wrap(err, "Couldn't derive key exchange material"))
	}

	fmt.Println("")

	encoder, err := armor.Encode(os.Stdout, "GOAX KEY EXCHANGE MATERIAL", nil)
	if err != nil {
		log.Fatal(errors.Wrap(err, "Couldn't open encoder"))
	}
	err = json.NewEncoder(encoder).Encode(kx)
	if err != nil {
		log.Fatal(errors.Wrap(err, "Couldn't encode key exchange material"))
	}
	err = encoder.Close()
	if err != nil {
		log.Fatal(errors.Wrap(err, "Couldn't close armor encoder"))
	}

	fmt.Println("")
}

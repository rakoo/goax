package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"

	"golang.org/x/crypto/openpgp/armor"
)

func send(peer string) {
	r, err := openRatchet(peer)
	if err != nil {
		if err == errNoRatchet {
			fmt.Printf("No ratchet for %s, please send this to the peer and \"receive\" what they send you back", peer)
			fmt.Println("\n")
			sendRatchet(peer)
			fmt.Println("")
			os.Exit(0)
		} else {
			log.Fatal(err)
		}
	}

	msg, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal("Couldn't read all stdin")
	}
	cipherText := r.Encrypt(msg)
	if err := saveRatchet(r, peer); err != nil {
		log.Println("Couldn't save ratchet:", err)
		os.Remove(path.Join("ratchets", hex.EncodeToString([]byte(peer))))
		os.Exit(1)
	}

	encoder, err := armor.Encode(os.Stdout, ENCRYPTED_MESSAGE_TYPE, nil)
	if err != nil {
		log.Fatal("Couldn't create armor encoder: ", err)
	}

	io.Copy(encoder, bytes.NewReader(cipherText))
	encoder.Close()
	fmt.Println("")
}

func sendRatchet(peer string) {
	r, err := createRatchet(peer)
	if err != nil {
		log.Fatalf("Couldn't create ratchet for %s: %s", peer, err)
	}
	err = saveRatchet(r, peer)
	if err != nil {
		log.Fatal("Couldn't save ratchet, will have to try another time", err)
	}
	kx, err := r.GetKeyExchangeMaterial()
	if err != nil {
		log.Fatal("Couldn't get key exchange material", err)
	}
	encoder, err := armor.Encode(os.Stdout, KEY_EXCHANGE_TYPE, nil)
	if err != nil {
		log.Fatal("Couldn't get armor encoder")
	}

	json.NewEncoder(encoder).Encode(kx)
	encoder.Close()
	fmt.Println("")
}

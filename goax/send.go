package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"

	"golang.org/x/crypto/openpgp/armor"

	"github.com/pkg/errors"
	"github.com/rakoo/goax"
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

	log.Println(len(cipherText))
}

var errNoRatchet = errors.New("No ratchet")

var errInvalidRatchet = errors.New("Invalid ratchet")

func openRatchet(peer string) (r *goax.Ratchet, err error) {
	f, err := os.Open(path.Join("ratchets", hex.EncodeToString([]byte(peer))))
	if err != nil {
		return nil, errNoRatchet
	}
	defer f.Close()

	r = new(goax.Ratchet)
	armorDecoder, err := armor.Decode(f)
	if err != nil {
		return nil, errors.Wrap(err, "Error opening decoder")
	}
	err = json.NewDecoder(armorDecoder.Body).Decode(r)
	if err != nil {
		return nil, errInvalidRatchet
	}

	return r, nil
}

func createRatchet(peer string) (r *goax.Ratchet, err error) {
	myIdentityKeyPrivate := getPrivateKey()
	var asArray [32]byte
	copy(asArray[:], myIdentityKeyPrivate)
	r = goax.New(rand.Reader, asArray)
	err = saveRatchet(r, peer)
	return r, err
}

func saveRatchet(r *goax.Ratchet, peer string) error {
	os.MkdirAll("ratchets", 0755)
	f, err := os.Create(path.Join("ratchets", hex.EncodeToString([]byte(peer))))
	if err != nil {
		return errors.Wrap(err, "Couldn't create ratchet file")
	}
	defer f.Close()

	armorEncoder, err := armor.Encode(f, "GOAX RATCHET", nil)
	if err != nil {
		return errors.Wrap(err, "Couldn't create armor encoder")
	}
	err = json.NewEncoder(armorEncoder).Encode(r)
	if err != nil {
		return errors.Wrap(err, "Couldn't marshall ratchet")
	}
	err = armorEncoder.Close()
	if err != nil {
		return errors.Wrap(err, "Couldn't close armor encoder")
	}
	return nil
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
	encoder, err := armor.Encode(os.Stdout, "KEY EXCHANGE MATERIAL", nil)
	if err != nil {
		log.Fatal("Couldn't get armor encoder")
	}

	json.NewEncoder(encoder).Encode(kx)
	encoder.Close()
}

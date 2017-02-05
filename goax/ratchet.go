package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path"

	"github.com/pkg/errors"
	"github.com/rakoo/goax"
	"golang.org/x/crypto/openpgp/armor"
)

var errNoRatchet = errors.New("No ratchet")

var errInvalidRatchet = errors.New("Invalid ratchet")

func openRatchet(peer string) (r *goax.Ratchet, err error) {
	f, err := os.Open(path.Join("ratchets", hex.EncodeToString([]byte(peer))))
	if err != nil {
		return nil, errNoRatchet
	}
	defer f.Close()

	myIdentityKeyPrivate := getPrivateKey()
	var asArray [32]byte
	copy(asArray[:], myIdentityKeyPrivate)
	r = goax.New(rand.Reader, asArray)

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

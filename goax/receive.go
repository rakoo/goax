package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/rakoo/goax"
	"golang.org/x/crypto/openpgp/armor"
)

func receive(peer string) {
	getRatchet := func(peer string) (r *goax.Ratchet) {
		r, err := openRatchet(peer)
		if err != nil {
			if err == errNoRatchet {
				fmt.Fprintf(os.Stderr, "No ratchet for %s, creating one.\n", peer)
				r, err = createRatchet(peer)
				if err != nil {
					log.Fatal("Couldn't create ratchet:", err)
				}
			} else {
				log.Fatal(err)
			}
		}
		return r

	}

	stat, err := os.Stdin.Stat()
	if err != nil {
		log.Fatal("Couldn't stat stdin")
	}
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		// stdin is from a terminal, not from a pipe
		fmt.Fprintln(os.Stderr, "Please paste in the message; when done, hit Ctrl-D\n")
	}
	stdin, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal("Couldn't read from stdin: ", err)
	}

	blockScanner := newBlockSplitter(stdin)
	var scannedSomething bool
	for blockScanner.Scan() {
		armorDecoder, err := armor.Decode(strings.NewReader(blockScanner.Text()))
		if err != nil {
			log.Fatal("Couldn't read message from stdin: ", err)
		}
		switch armorDecoder.Type {
		case ENCRYPTED_MESSAGE_TYPE:
			msg, err := ioutil.ReadAll(armorDecoder.Body)
			if err != nil {
				log.Fatal("Couldn't read message: ", err)
			}
			r := getRatchet(peer)
			plaintext, err := r.Decrypt(msg)
			if err != nil {
				log.Fatal("Couldn't decrypt message: ", err)
			}
			fmt.Println("")
			io.Copy(os.Stdout, bytes.NewReader(plaintext))
			deleteNew(peer)
			scannedSomething = true
		case KEY_EXCHANGE_TYPE:
			r := getRatchet(peer)
			var kx goax.KeyExchange
			json.NewDecoder(armorDecoder.Body).Decode(&kx)
			err = r.CompleteKeyExchange(kx)
			if err != nil && err != goax.ErrHandshakeComplete {
				log.Fatal("Invalid key exchange material: ", err)
			}
			saveRatchet(r, peer)
			scannedSomething = true
		default:
			log.Println("Unknown block type: ", armorDecoder.Type)
		}
	}
	if err := blockScanner.Err(); err != nil {
		log.Fatal("Error scanning blocks: ", err)
	}
	if !scannedSomething {
		fmt.Fprintln(os.Stderr, "The input you provided is invalid")
		os.Exit(1)
	}
}

// A blockSplitter is a bufio.Scanner that splits the input into
// multiple armored blocks
type blockSplitter struct {
	*bufio.Scanner
}

func newBlockSplitter(input []byte) blockSplitter {
	scanner := bufio.NewScanner(bytes.NewReader(input))
	split := func(data []byte, atEof bool) (advance int, token []byte, err error) {
		kxType := fmt.Sprintf("-----END %s-----", KEY_EXCHANGE_TYPE)
		kxTypeIdx := bytes.Index(data, []byte(kxType))

		encryptedType := fmt.Sprintf("-----END %s-----", ENCRYPTED_MESSAGE_TYPE)
		encryptedTypeIdx := bytes.Index(data, []byte(encryptedType))
		if kxTypeIdx != -1 {
			advance := kxTypeIdx + len(kxType)
			return advance, data[:advance], nil
		}
		if encryptedTypeIdx != -1 {
			advance := encryptedTypeIdx + len(encryptedType)
			return advance, data[:advance], nil
		}

		// No end of armored block, read more
		return 0, nil, nil
	}
	scanner.Split(split)
	return blockSplitter{scanner}
}

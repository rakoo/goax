package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/rakoo/goax"
)

var lastFetchTime = time.Unix(0, 0)

func main() {

	// Create ratchet and kx material
	var priv [32]byte
	io.ReadFull(rand.Reader, priv[:])
	ratchet := goax.New(rand.Reader, &priv)
	kx, err := ratchet.GetKeyExchangeMaterial()
	if err != nil {
		log.Fatal(err)
	}

	marshalled, err := json.Marshal(kx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Our kx, to be entered by remote:\n\n%s\n\n", marshalled)

	// Input remote kx material
	fmt.Println("Please enter the remote json key exchange:\n")
	line, err := bufio.NewReader(os.Stdin).ReadBytes(byte('\n'))
	if err != nil {
		log.Fatal(err)
	}
	var remoteKx goax.KeyExchange
	err = json.Unmarshal(line, &remoteKx)
	if err != nil {
		log.Fatal(err)
	}
	err = ratchet.CompleteKeyExchange(remoteKx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println()
	fmt.Printf(
		`Great! You are now connected to %x. Type 

		> m my message

to send messages to your contact, and

		> g

to retrieve all messages for you. Not that once retrieved, they can't be retrieved again.

Type

		> q

to quit.`, remoteKx.IdentityPublic)

	fmt.Println()

	for {
		fmt.Print("\n> ")
		cmd, msg := scan()

		if cmd == "g" {
			err := get(hex.EncodeToString(kx.IdentityPublic[:]), ratchet)
			if err != nil {
				fmt.Printf("! Error with retrieving message: %s\n", err)
			}
		} else if cmd == "m" {
			if msg == "" {
				fmt.Print("! format is 'm <message>'")
				continue
			}
			encrypted := hex.EncodeToString(ratchet.Encrypt([]byte(msg)))
			err := send(hex.EncodeToString(remoteKx.IdentityPublic[:]), encrypted)
			if err != nil {
				fmt.Printf("! Error with sending the message: %s\n", err)
			}
		} else if cmd == "q" {
			return
		} else {
			fmt.Println("! cmd is not understood, please enter (g)et or (m)essage or (q)uit")
		}
	}
}

func scan() (cmd, msg string) {
	line, err := bufio.NewReader(os.Stdin).ReadString(byte('\n'))
	if err != nil {
		log.Fatal(err)
	}

	spl := strings.SplitN(line[:len(line)-1], " ", 2)
	if len(spl) > 0 {
		cmd = spl[0]
	}
	if len(spl) > 1 {
		msg = spl[1]
	}
	return
}

func send(to, message string) error {
	reqUrl :=
		fmt.Sprintf("http://localhost:8090/message/new/?to=%s&message=%s",
			url.QueryEscape(to), url.QueryEscape(message))
	req, err := http.NewRequest("PUT", reqUrl, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return errors.New("Error sending message")
	}
	return nil
}

func get(to string, ratchet *goax.Ratchet) error {
	since := lastFetchTime.Format(time.RFC3339Nano)

	reqUrl :=
		fmt.Sprintf("http://localhost:8090/message/since/?to=%s&since=%s",
			url.QueryEscape(to), url.QueryEscape(since))
	resp, err := http.Get(reqUrl)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return errors.New("Error getting messages")
	}
	defer resp.Body.Close()

	var messages []timestampedMessage
	err = json.NewDecoder(resp.Body).Decode(&messages)
	if err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	for _, tmessage := range messages {
		raw, err := hex.DecodeString(tmessage.Message)
		if err != nil {
			fmt.Println("! Error when de-hexifying: ", err)
			continue
		}
		decoded, err := ratchet.Decrypt(raw)
		if err != nil {
			fmt.Println("! Error when decrypting: ", err)
			continue
		}
		fmt.Printf("< [%s] %s\n", tmessage.Timestamp.Format(time.RFC3339), decoded)
		if tmessage.Timestamp.After(lastFetchTime) {
			lastFetchTime = tmessage.Timestamp
		}
	}

	return nil
}

type timestampedMessage struct {
	Message   string
	Timestamp time.Time
}

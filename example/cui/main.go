package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/agl/xmpp"
	"github.com/jroimartin/gocui"
	"github.com/rakoo/goax"
)

var (
	ratchet *goax.Ratchet

	// The Key Exchange material marshalled as json
	kx string
)

type config struct {
	Jid                     string `json:"jid"`
	Password                string `json:"password"`
	ServerCertificateSHA256 string
}

func main() {
	// The xmpp connection
	configFile, err := os.Open(filepath.Join(os.Getenv("HOME"), ".config", "goax", "config.json"))
	if err != nil {
		log.Fatal("Couldn't open config file: ", err)
	}

	var conf config
	err = json.NewDecoder(configFile).Decode(&conf)
	if err != nil {
		log.Fatal("Couldn't decode json config: ", err)
	}

	parts := strings.SplitN(conf.Jid, "@", 2)
	if len(parts) != 2 {
		log.Fatal(errors.New("xmpp: invalid username (want user@domain): " + conf.Jid))
	}
	user := parts[0]
	domain := parts[1]

	host, port, err := xmpp.Resolve(domain)
	if err != nil {
		log.Fatalf("Failed to resolve xmpp host for domain %s: %s\n", domain, err)
	}
	addr := fmt.Sprintf("%s:%d", host, port)

	rawCert, err := hex.DecodeString(conf.ServerCertificateSHA256)
	if err != nil {
		log.Fatal("Bad server certificate : ", err)
	}

	cfg := &xmpp.Config{
		ServerCertificateSHA256: rawCert,
	}

	xmppClient, err := xmpp.Dial(addr, user, domain, conf.Password, cfg)
	if err != nil {
		log.Fatal("Couldn't connect to server: ", err)
	}
	xmppClient.SignalPresence("alive")

	// Create ratchet and kx material
	var priv [32]byte
	io.ReadFull(rand.Reader, priv[:])
	ratchet = goax.New(rand.Reader, &priv)
	kxraw, err := ratchet.GetKeyExchangeMaterial()
	if err != nil {
		log.Fatal(err)
	}

	marshalled, err := json.Marshal(kxraw)
	if err != nil {
		log.Fatal(err)
	}
	kx = string(marshalled)

	// The ui
	g := gocui.NewGui()
	if err := g.Init(); err != nil {
		log.Panicln(err)
	}
	defer g.Close()
	g.SetLayout(layout)
	if err := keybindings(g); err != nil {
		log.Panicln(err)
	}
	g.SelBgColor = gocui.ColorGreen
	g.SelFgColor = gocui.ColorBlack
	g.ShowCursor = true
	err = g.MainLoop()
	if err != nil && err != gocui.ErrorQuit {
		log.Panicln(err)
	}
}

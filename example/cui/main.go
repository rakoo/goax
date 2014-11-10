package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
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

	xmppClient *xmpp.Conn

	// contact type indexed by jid
	contacts map[string]contact
)

type contact struct {
	jid    string
	status string
}

// Convert a xmpp status ("away", "dnd") into a status, defaulting to
// "available"
func statusFromStatus(xstatus string) string {
	if xstatus == "" {
		return "available"
	}
	return xstatus
}

func main() {
	var err error
	var ourJid string
	xmppClient, ourJid, err = getXmppClient()
	if err != nil {
		log.Fatal(err)
	}
	resp, _, err := xmppClient.RequestRoster()
	if err != nil {
		log.Fatal(err)
	}

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

	go func() {
		roster := <-resp
		switch v := roster.Value.(type) {
		case *xmpp.ClientIQ:
			var roster xmpp.Roster
			err := xml.NewDecoder(bytes.NewReader(v.Query)).Decode(&roster)
			if err != nil {
				debugf(g, "Couldn't decode into a roster: %s\n", err)
			}
			contacts = make(map[string]contact)
			for _, entry := range roster.Item {
				contacts[entry.Jid] = contact{entry.Jid, "unknown"}
			}
			setContacts(g, contacts)
		}
	}()

	go func() {
		for {
			st, err := xmppClient.Next()
			if err != nil {
				debugf(g, "! Error at next stanza: %s\n", err)
				continue
			}

			switch v := st.Value.(type) {
			case *xmpp.ClientPresence:
				bare := xmpp.RemoveResourceFromJid(v.From)
				if bare == ourJid {
					continue
				}
				if len(contacts) == 0 {
					contacts = make(map[string]contact)
				}
				c, ok := contacts[v.From]
				if !ok {
					contacts[v.From] = contact{v.From, statusFromStatus(v.Status)}
					setContacts(g, contacts)
				} else {
					if v.Type == "error" {
						delete(contacts, v.From)
						setContacts(g, contacts)
					} else if c.status != statusFromStatus(v.Status) {
						c.status = statusFromStatus(v.Status)
						setContacts(g, contacts)
					}
				}
			default:
				debugf(g, "! Got stanza: %v\n", st.Name)
			}
		}
	}()

	err = g.MainLoop()
	if err != nil && err != gocui.ErrorQuit {
		log.Panicln(err)
	}
}

type config struct {
	Jid                     string `json:"jid"`
	Password                string `json:"password"`
	ServerCertificateSHA256 string
}

func getXmppClient() (*xmpp.Conn, string, error) {
	// The xmpp connection
	configFile, err := os.Open(filepath.Join(os.Getenv("HOME"), ".config", "goax", "config.json"))
	if err != nil {
		return nil, "", fmt.Errorf("Couldn't open config file: %s", err)
	}

	var conf config
	err = json.NewDecoder(configFile).Decode(&conf)
	if err != nil {
		return nil, "", fmt.Errorf("Couldn't decode json config: %s", err)
	}

	parts := strings.SplitN(conf.Jid, "@", 2)
	if len(parts) != 2 {
		return nil, "", fmt.Errorf("xmpp: invalid username (want user@domain): %s" + conf.Jid)
	}
	user := parts[0]
	domain := parts[1]

	host, port, err := xmpp.Resolve(domain)
	if err != nil {
		return nil, "", fmt.Errorf("Failed to resolve xmpp host for domain %s: %s", domain, err)
	}
	addr := fmt.Sprintf("%s:%d", host, port)

	rawCert, err := hex.DecodeString(conf.ServerCertificateSHA256)
	if err != nil {
		return nil, "", fmt.Errorf("Bad server certificate : %s", err)
	}

	logfile, err := os.Create("log")
	cfg := &xmpp.Config{
		InLog: logfile,
		ServerCertificateSHA256: rawCert,
	}

	xmppClient, err := xmpp.Dial(addr, user, domain, conf.Password, cfg)
	if err != nil {
		return nil, "", fmt.Errorf("Couldn't connect to server: %s", err)
	}
	xmppClient.SignalPresence("alive")

	return xmppClient, conf.Jid, nil
}

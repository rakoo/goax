package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"flag"
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
	privIdentity [32]byte

	xmppClient *xmpp.Conn

	// contact type indexed by jid
	contacts map[string]*contact
)

var (
	configPath = flag.String("config", filepath.Join(os.Getenv("HOME"), ".config", "goax", "config.json"), "The path to config file")
)

type axoParams struct {
	Identity []byte
	Dh       []byte
	Dh1      []byte
}

type contact struct {
	ratchet *goax.Ratchet
	jid     string
	status  string
}

func (c *contact) HasAxo() bool {
	if c.ratchet == nil {
		return false
	}

	_, err := c.ratchet.GetKeyExchangeMaterial()
	// if err != nil, ratchet is ready
	return err != nil
}

func (c *contact) String() string {
	return c.jid
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
	flag.Parse()
	var err error
	xmppClient, err = getXmppClient(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	io.ReadFull(rand.Reader, privIdentity[:])

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
		for {
			st, err := xmppClient.Next()
			if err != nil {
				debugf(g, "! Error at next stanza: %s\n", err)
				continue
			}

			switch v := st.Value.(type) {
			case *xmpp.ClientPresence:
				if len(contacts) == 0 {
					contacts = make(map[string]*contact)
				}
				c, ok := contacts[v.From]
				if !ok {
					contacts[v.From] = &contact{
						jid:    v.From,
						status: statusFromStatus(v.Status),
					}
				} else if c.status != statusFromStatus(v.Status) {
					c.status = statusFromStatus(v.Status)
				}
				go queryAxo(g, v.From)
				setContacts(g, contacts)
			case *xmpp.ClientIQ:
				var q axoQuery
				err := xml.Unmarshal(v.Query, &q)
				if err != nil {
					debugf(g, "! Not an axolotl query: %s\n", string(v.Query))
					continue
				}

				c, ok := contacts[v.From]
				if !ok {
					continue
				}
				if c.ratchet == nil {
					c.ratchet = goax.New(rand.Reader, privIdentity)
				}
				kx, err := c.ratchet.GetKeyExchangeMaterial()
				if err != nil {
					continue
				}
				resp := axoQuery{
					Identity: hex.EncodeToString(kx.IdentityPublic[:]),
					Dh:       hex.EncodeToString(kx.Dh[:]),
					Dh1:      hex.EncodeToString(kx.Dh1[:]),
				}
				xmppClient.SendIQReply(v.From, "result", v.Id, resp)
			case *xmpp.ClientMessage:
				c, ok := contacts[v.From]
				if !ok || !c.HasAxo() {
					continue
				}

				raw, err := base64.StdEncoding.DecodeString(v.Body)
				if err != nil {
					debugf(g, "! Couldn't base64-decode: %s\n", err)
					continue
				}
				decrypted, err := c.ratchet.Decrypt(raw)
				if err != nil {
					debugf(g, "! Couldn't decrypt message: %s\n", err)
					continue
				}
				displayTimestamped(g, v.From, string(decrypted))
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

func sendMessage(to, msg string) error {
	contact, ok := contacts[to]
	if !ok {
		return nil
	}
	encrypted := contact.ratchet.Encrypt([]byte(msg))
	based := base64.StdEncoding.EncodeToString(encrypted)
	xmppClient.Send(to, based)

	return nil
}

type axoQuery struct {
	XMLName  xml.Name `xml:"axolotl"`
	Identity string   `xml:"identity,omitempty"`
	Dh       string   `xml:"dh,omitempty"`
	Dh1      string   `xml:"dh1,omitempty"`
}

func queryAxo(g *gocui.Gui, to string) error {
	c, ok := contacts[to]
	if !ok {
		return nil
	}
	c.ratchet = goax.New(rand.Reader, privIdentity)

	resp, _, err := xmppClient.SendIQ(to, "get", axoQuery{})
	if err != nil {
		debugf(g, "! Couldn't query axolotl parameters for %s: %s", to, err)
	}
	response := <-resp
	switch v := response.Value.(type) {
	case *xmpp.ClientIQ:
		if v.Error.Type == "cancel" {
			return nil
		}

		c, ok := contacts[v.From]
		if !ok {
			return nil
		}

		var q axoQuery
		err := xml.Unmarshal(v.Query, &q)
		if err != nil {
			debugf(g, "! Not an axolotl query: %s\n", string(v.Query))
			return nil
		}

		id, err := hex.DecodeString(q.Identity)
		if err != nil {
			return err
		}
		dh, err := hex.DecodeString(q.Dh)
		if err != nil {
			return err
		}
		dh1, err := hex.DecodeString(q.Dh1)
		if err != nil {
			return err
		}

		remoteKx := &goax.KeyExchange{}
		copy(remoteKx.IdentityPublic[:], id)
		copy(remoteKx.Dh[:], dh)
		copy(remoteKx.Dh1[:], dh1)

		err = c.ratchet.CompleteKeyExchange(*remoteKx)
		if err != nil {
			debug(g, err.Error())
			return nil
		}
		setContacts(g, contacts)
	}
	return nil
}

type config struct {
	Jid                     string `json:"jid"`
	Password                string `json:"password"`
	ServerCertificateSHA256 string
}

func getXmppClient(configPath string) (*xmpp.Conn, error) {
	// The xmpp connection
	configFile, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("Couldn't open config file: %s", err)
	}

	var conf config
	err = json.NewDecoder(configFile).Decode(&conf)
	if err != nil {
		return nil, fmt.Errorf("Couldn't decode json config: %s", err)
	}

	parts := strings.SplitN(conf.Jid, "@", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("xmpp: invalid username (want user@domain): %s" + conf.Jid)
	}
	user := parts[0]
	domain := parts[1]

	host, port, err := xmpp.Resolve(domain)
	if err != nil {
		return nil, fmt.Errorf("Failed to resolve xmpp host for domain %s: %s", domain, err)
	}
	addr := fmt.Sprintf("%s:%d", host, port)

	rawCert, err := hex.DecodeString(conf.ServerCertificateSHA256)
	if err != nil {
		return nil, fmt.Errorf("Bad server certificate : %s", err)
	}

	logfile, err := os.Create("log")
	cfg := &xmpp.Config{
		InLog: logfile,
		ServerCertificateSHA256: rawCert,
	}

	xmppClient, err := xmpp.Dial(addr, user, domain, conf.Password, cfg)
	if err != nil {
		return nil, fmt.Errorf("Couldn't connect to server: %s", err)
	}
	xmppClient.SignalPresence("alive")

	return xmppClient, nil
}

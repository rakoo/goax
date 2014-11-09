package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/jroimartin/gocui"
	"github.com/rakoo/goax"
)

var (
	ratchet *goax.Ratchet

	// The Key Exchange material marshalled as json
	kx string
)

}

func main() {
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

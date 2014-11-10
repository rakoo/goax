package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jroimartin/gocui"
)

var (
	errBadInit = errors.New("Bad initialization")
)

func nextView(g *gocui.Gui, v *gocui.View) error {
	currentView := g.CurrentView()
	if currentView == nil || currentView.Name() == "contacts" {
		return g.SetCurrentView("input")
	}
	return g.SetCurrentView("contacts")
}
func cursorDown(g *gocui.Gui, v *gocui.View) error {
	if v != nil {
		cx, cy := v.Cursor()
		if err := v.SetCursor(cx, cy+1); err != nil {
			ox, oy := v.Origin()
			if err := v.SetOrigin(ox, oy+1); err != nil {
				return err
			}
		}
	}
	return nil
}
func cursorUp(g *gocui.Gui, v *gocui.View) error {
	if v != nil {
		ox, oy := v.Origin()
		cx, cy := v.Cursor()
		if err := v.SetCursor(cx, cy-1); err != nil && oy > 0 {
			if err := v.SetOrigin(ox, oy-1); err != nil {
				return err
			}
		}
	}
	return nil
}
func cursorLeft(g *gocui.Gui, v *gocui.View) error {
	if v != nil {
		ox, oy := v.Origin()
		cx, cy := v.Cursor()
		if err := v.SetCursor(cx-1, cy); err != nil && ox > 0 {
			if err := v.SetOrigin(ox-1, oy); err != nil {
				return err
			}
		}
	}
	return nil
}
func cursorRight(g *gocui.Gui, v *gocui.View) error {
	if v != nil {
		cx, cy := v.Cursor()
		if err := v.SetCursor(cx+1, cy); err != nil {
			ox, oy := v.Origin()
			if err := v.SetOrigin(ox+1, oy); err != nil {
				return err
			}
		}
	}
	return nil
}
func setContact(g *gocui.Gui, v *gocui.View) error {
	var l string
	var err error
	_, cy := v.Cursor()
	if l, err = v.Line(cy); err != nil {
		l = ""
	}
	g.View("header").Clear()
	fmt.Fprintf(g.View("header"), "Now discussing with %s", l)
	g.SetCurrentView("input")
	return nil
}

func setContacts(g *gocui.Gui, contacts map[string]*contact) error {
	g.View("contacts").Clear()
	asList := make([]string, 0)
	for _, c := range contacts {
		if c.ratchet == nil {
			continue
		}
		asList = append(asList, c.String())
	}
	sort.Strings(asList)
	for _, c := range asList {
		fmt.Fprintln(g.View("contacts"), c)
	}
	g.Flush()
	return nil
}

func keybindings(g *gocui.Gui) error {
	if err := g.SetKeybinding("", gocui.KeyCtrlSpace, 0, nextView); err != nil {
		return err
	}
	if err := g.SetKeybinding("contacts", gocui.KeyArrowDown, 0, cursorDown); err != nil {
		return err
	}
	if err := g.SetKeybinding("contacts", gocui.KeyArrowUp, 0, cursorUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("input", gocui.KeyArrowLeft, 0, cursorLeft); err != nil {
		return err
	}
	if err := g.SetKeybinding("input", gocui.KeyArrowRight, 0, cursorRight); err != nil {
		return err
	}
	if err := g.SetKeybinding("contacts", gocui.KeyEnter, 0, setContact); err != nil {
		return err
	}
	if err := g.SetKeybinding("input", gocui.KeyEnter, 0, send); err != nil {
		return err
	}
	return nil
}
func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if v, err := g.SetView("header", 60, 1, maxX, 3); err != nil {
		if err != gocui.ErrorUnkView {
			return err
		}
		fmt.Fprint(v, "Type '/connect jsonstring' to exchange messages with someone")
		v.Editable = true
	}
	if v, err := g.SetView("contacts", -1, 1, 60, maxY); err != nil {
		if err != gocui.ErrorUnkView {
			return err
		}
		v.Highlight = true
	}
	if _, err := g.SetView("main", 60, 3, maxX, maxY-1); err != nil {
		if err != gocui.ErrorUnkView {
			return err
		}
	}
	if v, err := g.SetView("input", 60, maxY-2, maxX, maxY); err != nil {
		if err != gocui.ErrorUnkView {
			return err
		}
		v.Editable = true
		if err := g.SetCurrentView("input"); err != nil {
			return err
		}
	}
	return nil
}

func send(g *gocui.Gui, v *gocui.View) error {
	v.SetOrigin(0, 0)
	message, err := v.Line(0)
	if err != nil {
		// We pressed enter and there's no text. Do nothing
		return nil
	}
	v.Clear()
	message = strings.Replace(message, string(0x00), "", -1) // Remove trailing 0x00
	if strings.TrimSpace(message) == "" {
		return nil
	}

	if message[0] == '/' {
		spl := strings.Split(message[1:], " ")
		if len(spl) == 0 {
			return nil
		}
		switch spl[0] {
		case "connect":
			g.View("header").Clear()
			fmt.Fprint(g.View("header"), message)
		case "q":
			fallthrough
		case "quit":
			return gocui.ErrorQuit
		default:
			fmt.Fprintf(g.View("main"), "! Unknown command: %#v", message)
		}
	} else {
		contacts := g.View("contacts")
		_, cy := contacts.Cursor()
		contact, err := contacts.Line(cy)
		if err != nil {
			return err
		}
		fmt.Fprintf(g.View("main"), "[%s] > %s\n", time.Now().UTC().Format(time.RFC3339), message)
		sendMessage(contact, message)
	}

	return nil
}

func debugf(g *gocui.Gui, format string, args ...interface{}) error {
	fmt.Fprintf(g.View("main"), format, args)
	g.Flush()
	return nil
}
func debug(g *gocui.Gui, str string) error {
	fmt.Fprintln(g.View("main"), str)
	g.Flush()
	return nil
}

// Copyright (c) 2013 Adam Langley. All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//    * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//    * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//    * Neither the name Pond nor the names of its contributors may be
// used to endorse or promote products derived from this software without
// specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package goax

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"io"
	"testing"
)

func pairedRatchet() (a, b *Ratchet) {
	var privA, privB [32]byte
	io.ReadFull(rand.Reader, privA[:])
	io.ReadFull(rand.Reader, privB[:])

	a, b = New(rand.Reader, privA), New(rand.Reader, privB)

	kxA, err := a.GetKeyExchangeMaterial()
	if err != nil {
		panic(err)
	}
	kxB, err := b.GetKeyExchangeMaterial()
	if err != nil {
		panic(err)
	}
	if err := a.CompleteKeyExchange(kxB); err != nil {
		panic(err)
	}
	if err := b.CompleteKeyExchange(kxA); err != nil {
		panic(err)
	}

	return
}

func TestExchange(t *testing.T) {
	a, b := pairedRatchet()

	msg := []byte("test message")
	encrypted := a.Encrypt(msg)
	result, err := b.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(msg, result) {
		t.Fatalf("result doesn't match: %x vs %x", msg, result)
	}
}

type scriptAction struct {
	// object is one of sendA, sendB or sendDelayed. The first two options
	// cause a message to be sent from one party to the other. The latter
	// causes a previously delayed message, identified by id, to be
	// delivered.
	object int
	// result is one of deliver, drop or delay. If delay, then the message
	// is stored using the value in id. This value can be repeated later
	// with a sendDelayed.
	result int
	id     int
}

const (
	sendA = iota
	sendB
	sendDelayed
	deliver
	drop
	delay
)

func testScript(t *testing.T, script []scriptAction) {
	type delayedMessage struct {
		msg       []byte
		encrypted []byte
		fromA     bool
	}
	delayedMessages := make(map[int]delayedMessage)
	a, b := pairedRatchet()

	for i, action := range script {
		switch action.object {
		case sendA, sendB:
			sender, receiver := a, b
			if action.object == sendB {
				sender, receiver = receiver, sender
			}

			var msg [20]byte
			rand.Reader.Read(msg[:])
			encrypted := sender.Encrypt(msg[:])

			switch action.result {
			case deliver:
				result, err := receiver.Decrypt(encrypted)
				if err != nil {
					t.Fatalf("#%d: receiver returned error: %s", i, err)
				}
				if !bytes.Equal(result, msg[:]) {
					t.Fatalf("#%d: bad message: got %x, not %x", i, result, msg[:])
				}
			case delay:
				if _, ok := delayedMessages[action.id]; ok {
					t.Fatalf("#%d: already have delayed message with id %d", i, action.id)
				}
				delayedMessages[action.id] = delayedMessage{msg[:], encrypted, sender == a}
			case drop:
			}
		case sendDelayed:
			delayed, ok := delayedMessages[action.id]
			if !ok {
				t.Fatalf("#%d: no such delayed message id: %d", i, action.id)
			}

			receiver := a
			if delayed.fromA {
				receiver = b
			}

			result, err := receiver.Decrypt(delayed.encrypted)
			if err != nil {
				t.Fatalf("#%d: receiver returned error: %s", i, err)
			}
			if !bytes.Equal(result, delayed.msg) {
				t.Fatalf("#%d: bad message: got %x, not %x", i, result, delayed.msg)
			}
		}

	}
}

func TestBackAndForth(t *testing.T) {
	testScript(t, []scriptAction{
		{sendA, deliver, -1},
		{sendB, deliver, -1},
		{sendA, deliver, -1},
		{sendB, deliver, -1},
		{sendA, deliver, -1},
		{sendB, deliver, -1},
	})
}

func TestReorder(t *testing.T) {
	testScript(t, []scriptAction{
		{sendA, deliver, -1},
		{sendA, delay, 0},
		{sendA, deliver, -1},
		{sendDelayed, deliver, 0},
	})
}

func TestReorderAfterRatchet(t *testing.T) {
	testScript(t, []scriptAction{
		{sendA, deliver, -1},
		{sendA, delay, 0},
		{sendB, deliver, -1},
		{sendA, deliver, -1},
		{sendB, deliver, -1},
		{sendDelayed, deliver, 0},
	})
}

func TestDrop(t *testing.T) {
	testScript(t, []scriptAction{
		{sendA, drop, -1},
		{sendA, drop, -1},
		{sendA, drop, -1},
		{sendA, drop, -1},
		{sendA, deliver, -1},
		{sendB, deliver, -1},
	})
}

func TestMarshal(t *testing.T) {
	var priv [32]byte
	io.ReadFull(rand.Reader, priv[:])
	ratchet := New(rand.Reader, priv)
	kx, err := ratchet.GetKeyExchangeMaterial()
	if err != nil {
		t.Fatal(err)
	}

	marshalled, err := json.Marshal(kx)
	if err != nil {
		t.Fatal(err)
	}

	var kxActual KeyExchange
	err = json.Unmarshal(marshalled, &kxActual)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(kxActual.Dh[:], kx.Dh[:]) {
		t.Fatalf("Dh doesn't match; expected %x, got %x\n", kx.Dh, kxActual.Dh)
	}

	if !bytes.Equal(kxActual.Dh1[:], kx.Dh1[:]) {
		t.Fatalf("Dh1 doesn't match; expected %x, got %x\n", kx.Dh1, kxActual.Dh1)
	}

	if !bytes.Equal(kxActual.IdentityPublic[:], kx.IdentityPublic[:]) {
		t.Fatalf("IdentityPublic doesn't match; expected %x, got %x\n", kx.IdentityPublic, kxActual.IdentityPublic)
	}
}

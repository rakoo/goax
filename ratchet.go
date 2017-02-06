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

// Package ratchet implements the axolotl ratchet, by Trevor Perrin. See
// https://github.com/trevp/axolotl/wiki.
package goax

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash"
	"io"
	"time"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/nacl/secretbox"
)

const (
	// headerSize is the size, in bytes, of a header's plaintext contents.
	headerSize = 4 /* uint32 message count */ +
		4 /* uint32 previous message count */ +
		32 /* curve25519 ratchet public */ +
		24 /* nonce for message */
	// sealedHeader is the size, in bytes, of an encrypted header.
	sealedHeaderSize = 24 /* nonce */ + headerSize + secretbox.Overhead
	// nonceInHeaderOffset is the offset of the message nonce in the
	// header's plaintext.
	nonceInHeaderOffset = 4 + 4 + 32
	// maxMissingMessages is the maximum number of missing messages that
	// we'll keep track of.
	maxMissingMessages = 8
)

type KeyExchange struct {
	IdentityPublic [32]byte `bencode:"identity"`
	Dh             [32]byte `bencode:"dh"`
	Dh1            [32]byte `bencode:"dh1"`
}

// MarshalJSON makes the KeyExchange a json.Marshaler by hex-ing fields
// before putting them in the json
func (k KeyExchange) MarshalJSON() ([]byte, error) {
	hexified := struct {
		IdentityPublic string `json:"idpub"`
		Dh             string `json:"dh"`
		Dh1            string `json:"dh1"`
	}{
		IdentityPublic: hex.EncodeToString(k.IdentityPublic[:]),
		Dh:             hex.EncodeToString(k.Dh[:]),
		Dh1:            hex.EncodeToString(k.Dh1[:]),
	}

	return json.Marshal(hexified)
}

// UnmarshalJSON makes the *KeyExchange a json.Unmarshaler by un-hex-ing
// fields after taking them from the json
func (k *KeyExchange) UnmarshalJSON(in []byte) error {
	type hexified struct {
		IdentityPublic string `json:"idpub"`
		Dh             string `json:"dh"`
		Dh1            string `json:"dh1"`
	}
	var h hexified
	err := json.Unmarshal(in, &h)
	if err != nil {
		return err
	}

	idpub, err := hex.DecodeString(h.IdentityPublic)
	if err != nil {
		return err
	}
	dh, err := hex.DecodeString(h.Dh)
	if err != nil {
		return err
	}
	dh1, err := hex.DecodeString(h.Dh1)
	if err != nil {
		return err
	}

	copy(k.IdentityPublic[:], idpub)
	copy(k.Dh[:], dh)
	copy(k.Dh1[:], dh1)

	return nil
}

// Ratchet contains the per-contact, crypto state.
type Ratchet struct {
	// myIdentityPrivate and TheirIdentityPublic contain the primary,
	// curve25519 identity keys.
	myIdentityPrivate, theirIdentityPublic [32]byte

	// rootKey gets updated by the DH ratchet.
	rootKey [32]byte
	// Header keys are used to encrypt message headers.
	sendHeaderKey, recvHeaderKey         [32]byte
	nextSendHeaderKey, nextRecvHeaderKey [32]byte
	// Chain keys are used for forward secrecy updating.
	sendChainKey, recvChainKey            [32]byte
	sendRatchetPrivate, recvRatchetPublic [32]byte
	sendCount, recvCount                  uint32
	prevSendCount                         uint32
	// ratchet is true if we will send a new ratchet value in the next message.
	ratchet bool

	// saved is a map from a header key to a map from sequence number to
	// message key.
	saved map[[32]byte]map[uint32]savedKey

	// kxPrivate0 and kxPrivate1 contain curve25519 private values during
	// the key exchange phase.
	kxPrivate0, kxPrivate1 *[32]byte

	// isHandshakeComplete tells if the key exchange was completed in
	// both directions
	isHandshakeComplete bool

	rand io.Reader
}

// MyPriv returns the hex-encoded private DH key
func (r *Ratchet) MyPriv() string {
	return hex.EncodeToString(r.myIdentityPrivate[:])
}

// savedKey contains a message key and timestamp for a message which has not
// been received. The timestamp comes from the message by which we learn of the
// missing message.
type savedKey struct {
	key       [32]byte
	timestamp time.Time
}

func (r *Ratchet) randBytes(buf []byte) {
	if _, err := io.ReadFull(r.rand, buf); err != nil {
		panic(err)
	}
}

func New(rand io.Reader, myPriv [32]byte) *Ratchet {
	r := &Ratchet{
		rand:              rand,
		kxPrivate0:        new([32]byte),
		kxPrivate1:        new([32]byte),
		saved:             make(map[[32]byte]map[uint32]savedKey),
		myIdentityPrivate: myPriv,
	}

	r.randBytes(r.kxPrivate0[:])
	r.randBytes(r.kxPrivate1[:])

	return r
}

// GetKeyExchangeMaterial returns key exchange information from the
// ratchet.
func (r *Ratchet) GetKeyExchangeMaterial() (kx KeyExchange, err error) {

	var public0, public1, myIdentity [32]byte
	curve25519.ScalarBaseMult(&public0, r.kxPrivate0)
	curve25519.ScalarBaseMult(&public1, r.kxPrivate1)
	curve25519.ScalarBaseMult(&myIdentity, &r.myIdentityPrivate)

	kx = KeyExchange{
		IdentityPublic: myIdentity,
		Dh:             public0,
		Dh1:            public1,
	}

	return
}

// deriveKey takes an HMAC object and a label and calculates out = HMAC(k, label).
func deriveKey(out *[32]byte, label []byte, h hash.Hash) {
	h.Reset()
	h.Write(label)
	n := h.Sum(out[:0])
	if &n[0] != &out[0] {
		panic("hash function too large")
	}
}

// These constants are used as the label argument to deriveKey to derive
// independent keys from a master key.
var (
	chainKeyLabel          = []byte("chain key")
	headerKeyLabel         = []byte("header key")
	nextRecvHeaderKeyLabel = []byte("next receive header key")
	rootKeyLabel           = []byte("root key")
	rootKeyUpdateLabel     = []byte("root key update")
	sendHeaderKeyLabel     = []byte("next send header key")
	messageKeyLabel        = []byte("message key")
	chainKeyStepLabel      = []byte("chain key step")
)

var ErrHandshakeComplete = errors.New("ratchet: handshake already complete")

// CompleteKeyExchange takes a KeyExchange message from the other party and
// establishes the ratchet.
func (r *Ratchet) CompleteKeyExchange(kx KeyExchange) error {
	if r.isHandshakeComplete {
		return ErrHandshakeComplete
	}

	var public0 [32]byte
	curve25519.ScalarBaseMult(&public0, r.kxPrivate0)

	if len(kx.Dh) != len(public0) {
		return errors.New("ratchet: peer's key exchange is invalid")
	}
	if len(kx.Dh1) != len(public0) {
		return errors.New("ratchet: peer using old-form key exchange")
	}

	var myIdentity [32]byte
	curve25519.ScalarBaseMult(&myIdentity, &r.myIdentityPrivate)

	if len(kx.IdentityPublic) != len(myIdentity) {
		return errors.New("Invalid identity length")
	}
	copy(r.theirIdentityPublic[:], kx.IdentityPublic[:])

	var amAlice bool
	switch bytes.Compare(public0[:], []byte(kx.Dh[:])) {
	case -1:
		amAlice = true
	case 1:
		amAlice = false
	case 0:
		return errors.New("ratchet: peer echoed our own DH values back")
	}

	var theirDH [32]byte
	copy(theirDH[:], kx.Dh[:])

	keyMaterial := make([]byte, 0, 32*5)
	var sharedKey [32]byte
	curve25519.ScalarMult(&sharedKey, r.kxPrivate0, &theirDH)
	keyMaterial = append(keyMaterial, sharedKey[:]...)

	if amAlice {
		curve25519.ScalarMult(&sharedKey, &r.myIdentityPrivate, &theirDH)
		keyMaterial = append(keyMaterial, sharedKey[:]...)
		curve25519.ScalarMult(&sharedKey, r.kxPrivate0, &r.theirIdentityPublic)
		keyMaterial = append(keyMaterial, sharedKey[:]...)
	} else {
		curve25519.ScalarMult(&sharedKey, r.kxPrivate0, &r.theirIdentityPublic)
		keyMaterial = append(keyMaterial, sharedKey[:]...)
		curve25519.ScalarMult(&sharedKey, &r.myIdentityPrivate, &theirDH)
		keyMaterial = append(keyMaterial, sharedKey[:]...)
	}

	h := hmac.New(sha256.New, keyMaterial)
	deriveKey(&r.rootKey, rootKeyLabel, h)
	if amAlice {
		deriveKey(&r.recvHeaderKey, headerKeyLabel, h)
		deriveKey(&r.nextSendHeaderKey, sendHeaderKeyLabel, h)
		deriveKey(&r.nextRecvHeaderKey, nextRecvHeaderKeyLabel, h)
		deriveKey(&r.recvChainKey, chainKeyLabel, h)
		copy(r.recvRatchetPublic[:], kx.Dh1[:])
	} else {
		deriveKey(&r.sendHeaderKey, headerKeyLabel, h)
		deriveKey(&r.nextRecvHeaderKey, sendHeaderKeyLabel, h)
		deriveKey(&r.nextSendHeaderKey, nextRecvHeaderKeyLabel, h)
		deriveKey(&r.sendChainKey, chainKeyLabel, h)
		copy(r.sendRatchetPrivate[:], r.kxPrivate1[:])
	}

	r.ratchet = amAlice
	r.isHandshakeComplete = true

	return nil
}

// Encrypt acts like append() but appends an encrypted version of msg to out.
func (r *Ratchet) Encrypt(msg []byte) []byte {
	if r.ratchet {
		r.randBytes(r.sendRatchetPrivate[:])
		copy(r.sendHeaderKey[:], r.nextSendHeaderKey[:])

		var sharedKey, keyMaterial [32]byte
		curve25519.ScalarMult(&sharedKey, &r.sendRatchetPrivate, &r.recvRatchetPublic)
		sha := sha256.New()
		sha.Write(rootKeyUpdateLabel)
		sha.Write(r.rootKey[:])
		sha.Write(sharedKey[:])

		sha.Sum(keyMaterial[:0])
		h := hmac.New(sha256.New, keyMaterial[:])
		deriveKey(&r.rootKey, rootKeyLabel, h)
		deriveKey(&r.nextSendHeaderKey, sendHeaderKeyLabel, h)
		deriveKey(&r.sendChainKey, chainKeyLabel, h)
		r.prevSendCount, r.sendCount = r.sendCount, 0
		r.ratchet = false
	}

	h := hmac.New(sha256.New, r.sendChainKey[:])
	var messageKey [32]byte
	deriveKey(&messageKey, messageKeyLabel, h)
	deriveKey(&r.sendChainKey, chainKeyStepLabel, h)

	var sendRatchetPublic [32]byte
	curve25519.ScalarBaseMult(&sendRatchetPublic, &r.sendRatchetPrivate)
	var header [headerSize]byte
	var headerNonce, messageNonce [24]byte
	r.randBytes(headerNonce[:])
	r.randBytes(messageNonce[:])

	binary.LittleEndian.PutUint32(header[0:4], r.sendCount)
	binary.LittleEndian.PutUint32(header[4:8], r.prevSendCount)
	copy(header[8:], sendRatchetPublic[:])
	copy(header[nonceInHeaderOffset:], messageNonce[:])
	out := make([]byte, len(headerNonce))
	copy(out, headerNonce[:])
	out = secretbox.Seal(out, header[:], &headerNonce, &r.sendHeaderKey)
	r.sendCount++
	return secretbox.Seal(out, msg, &messageNonce, &messageKey)
}

// trySavedKeys tries to decrypt ciphertext using keys saved for missing messages.
func (r *Ratchet) trySavedKeys(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < sealedHeaderSize {
		return nil, errors.New("ratchet: header too small to be valid")
	}

	sealedHeader := ciphertext[:sealedHeaderSize]
	var nonce [24]byte
	copy(nonce[:], sealedHeader)
	sealedHeader = sealedHeader[len(nonce):]

	for headerKey, messageKeys := range r.saved {
		header, ok := secretbox.Open(nil, sealedHeader, &nonce, &headerKey)
		if !ok {
			continue
		}
		if len(header) != headerSize {
			continue
		}
		msgNum := binary.LittleEndian.Uint32(header[:4])
		msgKey, ok := messageKeys[msgNum]
		if !ok {
			// This is a fairly common case: the message key might
			// not have been saved because it's the next message
			// key.
			return nil, nil
		}

		sealedMessage := ciphertext[sealedHeaderSize:]
		copy(nonce[:], header[nonceInHeaderOffset:])
		msg, ok := secretbox.Open(nil, sealedMessage, &nonce, &msgKey.key)
		if !ok {
			return nil, errors.New("ratchet: corrupt message")
		}
		delete(messageKeys, msgNum)
		if len(messageKeys) == 0 {
			delete(r.saved, headerKey)
		}
		return msg, nil
	}

	return nil, nil
}

// saveKeys takes a header key, the current chain key, a received message
// number and the expected message number and advances the chain key as needed.
// It returns the message key for given given message number and the new chain
// key. If any messages have been skipped over, it also returns savedKeys, a
// map suitable for merging with r.saved, that contains the message keys for
// the missing messages.
func (r *Ratchet) saveKeys(headerKey, recvChainKey *[32]byte, messageNum, receivedCount uint32) (provisionalChainKey, messageKey [32]byte, savedKeys map[[32]byte]map[uint32]savedKey, err error) {
	if messageNum < receivedCount {
		// This is a message from the past, but we didn't have a saved
		// key for it, which means that it's a duplicate message or we
		// expired the save key.
		err = errors.New("ratchet: duplicate message or message delayed longer than tolerance")
		return
	}

	missingMessages := messageNum - receivedCount
	if missingMessages > maxMissingMessages {
		err = errors.New("ratchet: message exceeds reordering limit")
		return
	}

	// messageKeys maps from message number to message key.
	var messageKeys map[uint32]savedKey
	now := time.Now()
	if missingMessages > 0 {
		messageKeys = make(map[uint32]savedKey)
	}

	copy(provisionalChainKey[:], recvChainKey[:])

	for n := receivedCount; n <= messageNum; n++ {
		h := hmac.New(sha256.New, provisionalChainKey[:])
		deriveKey(&messageKey, messageKeyLabel, h)
		deriveKey(&provisionalChainKey, chainKeyStepLabel, h)
		if n < messageNum {
			messageKeys[n] = savedKey{messageKey, now}
		}
	}

	if messageKeys != nil {
		savedKeys = make(map[[32]byte]map[uint32]savedKey)
		savedKeys[*headerKey] = messageKeys
	}

	return
}

// mergeSavedKeys takes a map of saved message keys from saveKeys and merges it
// into r.saved.
func (r *Ratchet) mergeSavedKeys(newKeys map[[32]byte]map[uint32]savedKey) {
	for headerKey, newMessageKeys := range newKeys {
		messageKeys, ok := r.saved[headerKey]
		if !ok {
			r.saved[headerKey] = newMessageKeys
			continue
		}

		for n, messageKey := range newMessageKeys {
			messageKeys[n] = messageKey
		}
	}
}

// isZeroKey returns true if key is all zeros.
func isZeroKey(key *[32]byte) bool {
	var x uint8
	for _, v := range key {
		x |= v
	}

	return x == 0
}

func (r *Ratchet) Decrypt(ciphertext []byte) ([]byte, error) {
	if !r.isHandshakeComplete {
		return nil, errors.New("handshake not complete yet")
	}

	msg, err := r.trySavedKeys(ciphertext)
	if err != nil || msg != nil {
		return msg, err
	}

	sealedHeader := ciphertext[:sealedHeaderSize]
	sealedMessage := ciphertext[sealedHeaderSize:]
	var nonce [24]byte
	copy(nonce[:], sealedHeader)
	sealedHeader = sealedHeader[len(nonce):]

	header, ok := secretbox.Open(nil, sealedHeader, &nonce, &r.recvHeaderKey)
	ok = ok && !isZeroKey(&r.recvHeaderKey)
	if ok {
		if len(header) != headerSize {
			return nil, errors.New("ratchet: incorrect header size")
		}
		messageNum := binary.LittleEndian.Uint32(header[:4])
		provisionalChainKey, messageKey, savedKeys, err := r.saveKeys(&r.recvHeaderKey, &r.recvChainKey, messageNum, r.recvCount)
		if err != nil {
			return nil, err
		}

		copy(nonce[:], header[nonceInHeaderOffset:])
		msg, ok := secretbox.Open(nil, sealedMessage, &nonce, &messageKey)
		if !ok {
			return nil, errors.New("ratchet: corrupt message")
		}

		copy(r.recvChainKey[:], provisionalChainKey[:])
		r.mergeSavedKeys(savedKeys)
		r.recvCount = messageNum + 1
		return msg, nil
	}

	header, ok = secretbox.Open(nil, sealedHeader, &nonce, &r.nextRecvHeaderKey)
	if !ok {
		return nil, errors.New("ratchet: cannot decrypt")
	}
	if len(header) != headerSize {
		return nil, errors.New("ratchet: incorrect header size")
	}

	if r.ratchet {
		return nil, errors.New("ratchet: received message encrypted to next header key without ratchet flag set")
	}

	messageNum := binary.LittleEndian.Uint32(header[:4])
	prevMessageCount := binary.LittleEndian.Uint32(header[4:8])

	_, _, oldSavedKeys, err := r.saveKeys(&r.recvHeaderKey, &r.recvChainKey, prevMessageCount, r.recvCount)
	if err != nil {
		return nil, err
	}

	var dhPublic, sharedKey, rootKey, chainKey, keyMaterial [32]byte
	copy(dhPublic[:], header[8:])

	curve25519.ScalarMult(&sharedKey, &r.sendRatchetPrivate, &dhPublic)

	sha := sha256.New()
	sha.Write(rootKeyUpdateLabel)
	sha.Write(r.rootKey[:])
	sha.Write(sharedKey[:])

	var rootKeyHMAC hash.Hash

	sha.Sum(keyMaterial[:0])
	rootKeyHMAC = hmac.New(sha256.New, keyMaterial[:])
	deriveKey(&rootKey, rootKeyLabel, rootKeyHMAC)
	deriveKey(&chainKey, chainKeyLabel, rootKeyHMAC)

	provisionalChainKey, messageKey, savedKeys, err := r.saveKeys(&r.nextRecvHeaderKey, &chainKey, messageNum, 0)
	if err != nil {
		return nil, err
	}

	copy(nonce[:], header[nonceInHeaderOffset:])
	msg, ok = secretbox.Open(nil, sealedMessage, &nonce, &messageKey)
	if !ok {
		return nil, errors.New("ratchet: corrupt message")
	}

	copy(r.rootKey[:], rootKey[:])
	copy(r.recvChainKey[:], provisionalChainKey[:])
	copy(r.recvHeaderKey[:], r.nextRecvHeaderKey[:])
	deriveKey(&r.nextRecvHeaderKey, sendHeaderKeyLabel, rootKeyHMAC)
	for i := range r.sendRatchetPrivate {
		r.sendRatchetPrivate[i] = 0
	}
	copy(r.recvRatchetPublic[:], dhPublic[:])

	r.recvCount = messageNum + 1
	r.mergeSavedKeys(oldSavedKeys)
	r.mergeSavedKeys(savedKeys)
	r.ratchet = true

	return msg, nil
}

func dup(key *[32]byte) []byte {
	if key == nil {
		return nil
	}

	ret := make([]byte, 32)
	copy(ret, key[:])
	return ret
}

type ratchetState struct {
	RootKey             []byte                   `json:"root_key,omitempty"`
	SendHeaderKey       []byte                   `json:"send_header_key,omitempty"`
	RecvHeaderKey       []byte                   `json:"recv_header_key,omitempty"`
	NextSendHeaderKey   []byte                   `json:"next_send_header_key,omitempty"`
	NextRecvHeaderKey   []byte                   `json:"next_recv_header_key,omitempty"`
	SendChainKey        []byte                   `json:"send_chain_key,omitempty"`
	RecvChainKey        []byte                   `json:"recv_chain_key,omitempty"`
	SendRatchetPrivate  []byte                   `json:"send_ratchet_private,omitempty"`
	RecvRatchetPublic   []byte                   `json:"recv_ratchet_public,omitempty"`
	SendCount           uint32                   `json:"send_count,omitempty"`
	RecvCount           uint32                   `json:"recv_count,omitempty"`
	PrevSendCount       uint32                   `json:"prev_send_count,omitempty"`
	Ratchet             bool                     `json:"ratchet,omitempty"`
	V2                  bool                     `json:"v2,omitempty"`
	Private0            []byte                   `json:"private0,omitempty"`
	Private1            []byte                   `json:"private1,omitempty"`
	IsHandshakeComplete bool                     `json:"isHandshakeComplete,omitempty"`
	SavedKeys           []ratchetState_SavedKeys `json:"saved_keys,omitempty"`
	XXX_unrecognized    []byte                   `json:"-"`
}

type ratchetState_SavedKeys struct {
	HeaderKey        []byte                              `json:"header_key,omitempty"`
	MessageKeys      []ratchetState_SavedKeys_MessageKey `json:"message_keys,omitempty"`
	XXX_unrecognized []byte                              `json:"-"`
}

type ratchetState_SavedKeys_MessageKey struct {
	Num              uint32 `json:"num,omitempty"`
	Key              []byte `json:"key,omitempty"`
	CreationTime     int64  `json:"creation_time,omitempty"`
	XXX_unrecognized []byte `json:"-"`
}

func (r *Ratchet) MarshalJSON() ([]byte, error) {
	s := ratchetState{
		RootKey:             dup(&r.rootKey),
		SendHeaderKey:       dup(&r.sendHeaderKey),
		RecvHeaderKey:       dup(&r.recvHeaderKey),
		NextSendHeaderKey:   dup(&r.nextSendHeaderKey),
		NextRecvHeaderKey:   dup(&r.nextRecvHeaderKey),
		SendChainKey:        dup(&r.sendChainKey),
		RecvChainKey:        dup(&r.recvChainKey),
		SendRatchetPrivate:  dup(&r.sendRatchetPrivate),
		RecvRatchetPublic:   dup(&r.recvRatchetPublic),
		SendCount:           r.sendCount,
		RecvCount:           r.recvCount,
		PrevSendCount:       r.prevSendCount,
		Ratchet:             r.ratchet,
		Private0:            dup(r.kxPrivate0),
		Private1:            dup(r.kxPrivate1),
		IsHandshakeComplete: r.isHandshakeComplete,
	}

	for headerKey, messageKeys := range r.saved {
		keys := make([]ratchetState_SavedKeys_MessageKey, 0, len(messageKeys))
		for messageNum, savedKey := range messageKeys {
			keys = append(keys, ratchetState_SavedKeys_MessageKey{
				Num:          messageNum,
				Key:          dup(&savedKey.key),
				CreationTime: savedKey.timestamp.Unix(),
			})
		}
		s.SavedKeys = append(s.SavedKeys, ratchetState_SavedKeys{
			HeaderKey:   dup(&headerKey),
			MessageKeys: keys,
		})
	}

	return json.Marshal(s)
}

func unmarshalKey(dst *[32]byte, src []byte) bool {
	if len(src) != 32 {
		return false
	}
	copy(dst[:], src)
	return true
}

var badSerialisedKeyLengthErr = errors.New("ratchet: bad serialised key length")

func (r *Ratchet) UnmarshalJSON(in []byte) error {
	var s ratchetState
	err := json.Unmarshal(in, &s)
	if err != nil {
		return err
	}

	if !unmarshalKey(&r.rootKey, s.RootKey) ||
		!unmarshalKey(&r.sendHeaderKey, s.SendHeaderKey) ||
		!unmarshalKey(&r.recvHeaderKey, s.RecvHeaderKey) ||
		!unmarshalKey(&r.nextSendHeaderKey, s.NextSendHeaderKey) ||
		!unmarshalKey(&r.nextRecvHeaderKey, s.NextRecvHeaderKey) ||
		!unmarshalKey(&r.sendChainKey, s.SendChainKey) ||
		!unmarshalKey(&r.recvChainKey, s.RecvChainKey) ||
		!unmarshalKey(&r.sendRatchetPrivate, s.SendRatchetPrivate) ||
		!unmarshalKey(&r.recvRatchetPublic, s.RecvRatchetPublic) {
		return badSerialisedKeyLengthErr
	}

	r.sendCount = s.SendCount
	r.recvCount = s.RecvCount
	r.prevSendCount = s.PrevSendCount
	r.ratchet = s.Ratchet
	r.isHandshakeComplete = s.IsHandshakeComplete

	if len(s.Private0) > 0 {
		if !unmarshalKey(r.kxPrivate0, s.Private0) ||
			!unmarshalKey(r.kxPrivate1, s.Private1) {
			return badSerialisedKeyLengthErr
		}
	} else {
		r.kxPrivate0 = nil
		r.kxPrivate1 = nil
	}

	for _, saved := range s.SavedKeys {
		var headerKey [32]byte
		if !unmarshalKey(&headerKey, saved.HeaderKey) {
			return badSerialisedKeyLengthErr
		}
		messageKeys := make(map[uint32]savedKey)
		for _, messageKey := range saved.MessageKeys {
			var savedKey savedKey
			if !unmarshalKey(&savedKey.key, messageKey.Key) {
				return badSerialisedKeyLengthErr
			}
			savedKey.timestamp = time.Unix(messageKey.CreationTime, 0)
			messageKeys[messageKey.Num] = savedKey
		}

		r.saved[headerKey] = messageKeys
	}

	return nil
}

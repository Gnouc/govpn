/*
GoVPN -- simple secure free software virtual private network daemon
Copyright (C) 2014-2015 Sergey Matveev <stargrave@stargrave.org>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package govpn

import (
	"io"

	"golang.org/x/crypto/poly1305"
)

const (
	EtherSize = 14
)

type TAP struct {
	Name   string
	dev    io.ReadWriter
	buf    []byte
	sink   chan []byte
	ready  chan struct{}
	synced bool
}

// Return maximal acceptable TAP interface MTU. This is daemon's MTU
// minus nonce, MAC, packet size mark and Ethernet header sizes.
func TAPMaxMTU() int {
	return MTU - poly1305.TagSize - NonceSize - PktSizeSize - EtherSize
}

func NewTAP(ifaceName string) (*TAP, error) {
	maxIfacePktSize := TAPMaxMTU() + EtherSize
	tapRaw, err := newTAPer(ifaceName)
	if err != nil {
		return nil, err
	}
	tap := TAP{
		Name:   ifaceName,
		dev:    tapRaw,
		buf:    make([]byte, maxIfacePktSize),
		sink:   make(chan []byte),
		ready:  make(chan struct{}),
		synced: false,
	}
	go func() {
		var n int
		var err error
		for {
			<-tap.ready
			n, err = tap.dev.Read(tap.buf)
			if err != nil {
				panic("Reading TAP:" + err.Error())
			}
			tap.sink <- tap.buf[:n]
		}
	}()
	return &tap, nil
}

func (t *TAP) Write(data []byte) (n int, err error) {
	return t.dev.Write(data)
}

/*
GoVPN -- simple secure free software virtual private network daemon
Copyright (C) 2014-2016 Sergey Matveev <stargrave@stargrave.org>

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

package main

import (
	"log"
	"net"
	"sync/atomic"
	"time"

	"cypherpunks.ru/govpn"
)

func startUDP(timeouted, rehandshaking, termination chan struct{}) {
	remote, err := net.ResolveUDPAddr("udp", *remoteAddr)
	if err != nil {
		log.Fatalln("Can not resolve remote address:", err)
	}
	conn, err := net.DialUDP("udp", nil, remote)
	if err != nil {
		log.Fatalln("Can not listen on UDP:", err)
	}
	govpn.Printf(`[connected remote="%s"]`, *remoteAddr)

	hs := govpn.HandshakeStart(*remoteAddr, conn, conf)
	buf := make([]byte, *mtu*2)
	var n int
	var timeouts int
	var peer *govpn.Peer
	var terminator chan struct{}
MainCycle:
	for {
		select {
		case <-termination:
			break MainCycle
		default:
		}

		conn.SetReadDeadline(time.Now().Add(time.Second))
		n, err = conn.Read(buf)
		if timeouts == timeout {
			govpn.Printf(`[connection-timeouted remote="%s"]`, *remoteAddr)
			timeouted <- struct{}{}
			break
		}
		if err != nil {
			timeouts++
			continue
		}
		if peer != nil {
			if peer.PktProcess(buf[:n], tap, true) {
				timeouts = 0
			} else {
				govpn.Printf(`[packet-unauthenticated remote="%s"]`, *remoteAddr)
				timeouts++
			}
			if atomic.LoadUint64(&peer.BytesIn)+atomic.LoadUint64(&peer.BytesOut) > govpn.MaxBytesPerKey {
				govpn.Printf(`[rehandshake-required remote="%s"]`, *remoteAddr)
				rehandshaking <- struct{}{}
				break MainCycle
			}
			continue
		}
		if idsCache.Find(buf[:n]) == nil {
			govpn.Printf(`[identity-invalid remote="%s"]`, *remoteAddr)
			continue
		}
		timeouts = 0
		peer = hs.Client(buf[:n])
		if peer == nil {
			continue
		}
		govpn.Printf(`[handshake-completed remote="%s"]`, *remoteAddr)
		knownPeers = govpn.KnownPeers(map[string]**govpn.Peer{*remoteAddr: &peer})
		if firstUpCall {
			go govpn.ScriptCall(*upPath, *ifaceName, *remoteAddr)
			firstUpCall = false
		}
		hs.Zero()
		terminator = make(chan struct{})
		go govpn.PeerTapProcessor(peer, tap, terminator)
	}
	if terminator != nil {
		terminator <- struct{}{}
	}
	if hs != nil {
		hs.Zero()
	}
	conn.Close()
}

package main

import (
	"log"
	"net"
	"time"

	"github.com/dr-useless/gobkv/protocol"
	"github.com/dr-useless/gobkv/store"
)

const BACKOFF = 10        // ms
const BACKOFF_LIMIT = 100 // ms

// Listens for requests
// & sends response
func serveConn(conn net.Conn, store *store.Store, authSecret string) {
	backoff := BACKOFF // ms
	authed := authSecret == ""
loop:
	for {
		msg, err := protocol.ReadMsgFrom(conn)
		if err != nil {
			if backoff > BACKOFF_LIMIT {
				respondWithStatus(conn, protocol.StatusError)
				log.Println("conn timed out:", err)
				break
			}
			time.Sleep(time.Duration(backoff) * time.Millisecond)
			backoff *= 2
			continue
		}

		switch msg.Op {
		case protocol.OpPing:
			handlePing(conn)
			continue
		case protocol.OpAuth:
			authed = handleAuth(conn, msg, authSecret)
			continue
		default:
			if !authed {
				respondWithStatus(conn, protocol.StatusUnauthorized)
				break loop
			}
		}

		// requires auth
		switch msg.Op {
		case protocol.OpGet:
			handleGet(conn, msg, store)
		case protocol.OpSet:
			handleSet(conn, msg, store)
		case protocol.OpSetAck:
			handleSet(conn, msg, store)
		case protocol.OpDel:
			handleDel(conn, msg, store)
		case protocol.OpDelAck:
			handleDel(conn, msg, store)
		case protocol.OpList:
			handleList(conn, msg, store)
		case protocol.OpClose:
			break loop
		}
	}

	conn.Close()
}

func handlePing(conn net.Conn) {
	resp := protocol.Msg{
		Op:     protocol.OpPong,
		Status: protocol.StatusOk,
	}
	resp.WriteTo(conn)
}

func handleAuth(conn net.Conn, msg *protocol.Msg, secret string) bool {
	given := string(msg.Body)
	authed := given == secret
	if authed {
		respondWithStatus(conn, protocol.StatusOk)
	} else {
		respondWithStatus(conn, protocol.StatusUnauthorized)
	}
	return authed
}

func handleGet(conn net.Conn, msg *protocol.Msg, s *store.Store) {
	req, err := protocol.DecodeData(msg.Body)
	if err != nil {
		respondWithStatus(conn, protocol.StatusError)
		return
	}
	slot := s.Get(req.Key)
	if slot == nil {
		respondWithStatus(conn, protocol.StatusNotFound)
		return
	}
	dataResp := protocol.Data{
		Key:     req.Key,
		Value:   slot.Value,
		Expires: slot.Expires,
	}
	body, _ := dataResp.Encode()
	resp := protocol.Msg{
		Op:     protocol.OpGet,
		Status: protocol.StatusOk,
		Body:   body,
	}
	resp.WriteTo(conn)
}

func handleSet(conn net.Conn, msg *protocol.Msg, s *store.Store) {
	d, err := protocol.DecodeData(msg.Body)
	if err != nil {
		respondWithStatus(conn, protocol.StatusError)
		return
	}
	slot := store.Slot{
		Value:   d.Value,
		Expires: d.Expires,
	}
	s.Set(d.Key, &slot)
	if msg.Op == protocol.OpSetAck {
		respondWithStatus(conn, protocol.StatusOk)
	}
}

func handleDel(conn net.Conn, msg *protocol.Msg, s *store.Store) {
	d, err := protocol.DecodeData(msg.Body)
	if err != nil {
		respondWithStatus(conn, protocol.StatusError)
		return
	}
	s.Del(d.Key)
	if msg.Op == protocol.OpDelAck {
		respondWithStatus(conn, protocol.StatusOk)
	}
}

// TODO: add ability to stream unknown length,
// then stream keys as they are found (buffered)
func handleList(conn net.Conn, msg *protocol.Msg, s *store.Store) {
	d, err := protocol.DecodeData(msg.Body)
	if err != nil {
		respondWithStatus(conn, protocol.StatusError)
		return
	}
	dResp := protocol.Data{
		Keys: s.List(d.Key),
	}
	dRespEnc, err := dResp.Encode()
	if err != nil {
		respondWithStatus(conn, protocol.StatusError)
		return
	}
	resp := protocol.Msg{
		Op:     protocol.OpList,
		Status: protocol.StatusOk,
		Body:   dRespEnc,
	}
	resp.WriteTo(conn)
}

func respondWithStatus(conn net.Conn, status byte) {
	resp := protocol.Msg{
		Status: status,
	}
	resp.WriteTo(conn)
}
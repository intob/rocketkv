package main

import (
	"net"

	"github.com/intob/chamux"
	"github.com/intob/gobkv/protocol"
	"github.com/intob/gobkv/store"
)

const MSG = "msg"

func serveConn(conn net.Conn, st *store.Store, authSecret string) {
	authed := authSecret == ""
	mc := chamux.NewMConn(conn, chamux.Gob{}, 2048)
	msgTopic := chamux.NewTopic(MSG)
	msgSub := msgTopic.Subscribe()
	mc.AddTopic(&msgTopic)
loop:
	for mBytes := range msgSub {
		msg := &protocol.Msg{}
		err := msg.DecodeFrom(mBytes)
		if err != nil {
			panic(err)
		}

		switch msg.Op {
		case protocol.OpPing:
			handlePing(&mc)
			continue
		case protocol.OpAuth:
			authed = handleAuth(&mc, msg, authSecret)
			continue
		default:
			if !authed {
				respond(&mc, protocol.Msg{
					Status: protocol.StatusError,
				})
				break loop
			}
		}

		// requires auth
		switch msg.Op {
		case protocol.OpGet:
			handleGet(&mc, msg, st)
		case protocol.OpSet:
			handleSet(&mc, msg, st)
		case protocol.OpSetAck:
			handleSet(&mc, msg, st)
		case protocol.OpDel:
			handleDel(&mc, msg, st)
		case protocol.OpDelAck:
			handleDel(&mc, msg, st)
		case protocol.OpList:
			handleList(&mc, msg, st)
		case protocol.OpClose:
			break loop
		}
	}

	conn.Close()
}

func handlePing(mc *chamux.MConn) {
	respond(mc, protocol.Msg{
		Op:     protocol.OpPong,
		Status: protocol.StatusOk,
	})
}

func handleAuth(mc *chamux.MConn, msg *protocol.Msg, secret string) bool {
	authed := msg.Key == secret
	if authed {
		respondWithStatus(mc, protocol.StatusOk)
	} else {
		respondWithStatus(mc, protocol.StatusError)
	}
	return authed
}

func handleGet(mc *chamux.MConn, msg *protocol.Msg, st *store.Store) {
	slot := st.Get(msg.Key)
	if slot == nil {
		respondWithStatus(mc, protocol.StatusError)
		return
	}
	respond(mc, protocol.Msg{
		Key:     msg.Key,
		Value:   slot.Value,
		Expires: slot.Expires,
	})
}

func handleSet(mc *chamux.MConn, msg *protocol.Msg, st *store.Store) {
	slot := store.Slot{
		Value:   msg.Value,
		Expires: msg.Expires,
	}
	st.Set(msg.Key, &slot)
	if msg.Op == protocol.OpSetAck {
		respondWithStatus(mc, protocol.StatusOk)
	}
}

func handleDel(mc *chamux.MConn, msg *protocol.Msg, st *store.Store) {
	st.Del(msg.Key)
	if msg.Op == protocol.OpDelAck {
		respondWithStatus(mc, protocol.StatusOk)
	}
}

func handleList(mc *chamux.MConn, msg *protocol.Msg, st *store.Store) {
	respond(mc, protocol.Msg{
		Keys: st.List(msg.Key),
	})
}

func respond(mc *chamux.MConn, resp protocol.Msg) {
	respEnc, err := resp.Encode()
	if err != nil {
		panic(err)
	}
	mc.Publish(chamux.NewFrame(respEnc, MSG))
}

func respondWithStatus(mc *chamux.MConn, status byte) {
	respond(mc, protocol.Msg{
		Status: status,
	})
}

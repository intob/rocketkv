package client

import (
	"bufio"
	"errors"
	"net"

	"github.com/intob/rocketkv/protocol"
)

const errEmptySecret = "secret is empty"
const errNegativeExpiry = "expires should be 0 or positive"
const errEmptyKey = "key must not be empty"

// Client provides connection & command helpers
type Client struct {
	conn net.Conn
	Msgs chan protocol.Msg
}

// NewClient returns a pointer to a new Client
//
// Messages can be receieved on Msgs chan.
func NewClient(conn net.Conn) *Client {
	c := &Client{
		conn: conn,
		Msgs: make(chan protocol.Msg),
	}
	go c.pumpMsgs()
	return c
}

// pumpMsgs reads from conn, decodes & writes
// messages to Msgs chan
func (c *Client) pumpMsgs() {
	scan := bufio.NewScanner(c.conn)
	scan.Split(protocol.SplitPlusEnd)
	for scan.Scan() {
		mBytes := scan.Bytes()
		msg, err := protocol.DecodeMsg(mBytes)
		if err != nil {
			panic(err)
		}
		c.Msgs <- *msg
	}
}

// Send encodes & publishes the given message
func (c *Client) Send(msg *protocol.Msg) error {
	msgEnc, err := protocol.EncodeMsg(msg)
	if err != nil {
		return err
	}
	_, err = c.conn.Write(msgEnc)
	return err
}

// Close sends a close message,
// and closes the connection
func (c *Client) Close() error {
	defer c.conn.Close()
	return c.Send(&protocol.Msg{
		Op: protocol.OpClose,
	})
}

// Ping sends a ping message
//
// A status message will follow
func (c *Client) Ping() error {
	return c.Send(&protocol.Msg{
		Op: protocol.OpPing,
	})
}

// Auth sends an auth message using the given secret
//
// A status message will follow
func (c *Client) Auth(secret string) error {
	if secret == "" {
		return errors.New(errEmptySecret)
	}
	msg := &protocol.Msg{
		Op:  protocol.OpAuth,
		Key: secret,
	}
	return c.Send(msg)
}

// Set the value & expires properties of the key
//
// If expires is 0, the key will not expire
// If ack is true, a response will follow
func (c *Client) Set(key string, value []byte, expires int64, ack bool) error {
	if expires < 0 {
		return errors.New(errNegativeExpiry)
	}
	if key == "" {
		return errors.New(errEmptyKey)
	}
	msg := &protocol.Msg{
		Op:    protocol.OpSet,
		Key:   key,
		Value: value,
	}
	if ack {
		msg.Op = protocol.OpSetAck
	}
	return c.Send(msg)
}

// Get the value & expires time for a key
//
// The response will follow on the MsgChan
func (c *Client) Get(key string) error {
	msg := &protocol.Msg{
		Op:  protocol.OpGet,
		Key: key,
	}
	return c.Send(msg)
}

// Del deletes a key
//
// If ack is true, a status response will follow
func (c *Client) Del(key string, ack bool) error {
	msg := &protocol.Msg{
		Op:  protocol.OpDel,
		Key: key,
	}
	if ack {
		msg.Op = protocol.OpDelAck
	}
	return c.Send(msg)
}

// List all keys with the given prefix
func (c *Client) List(keyPrefix string) error {
	msg := &protocol.Msg{
		Op:  protocol.OpList,
		Key: keyPrefix,
	}
	return c.Send(msg)
}

// Count keys with the given prefix
func (c *Client) Count(keyPrefix string) error {
	msg := &protocol.Msg{
		Op:  protocol.OpCount,
		Key: keyPrefix,
	}
	return c.Send(msg)
}

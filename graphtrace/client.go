package graphtrace

import (
	"context"
	"encoding/binary"
	"errors"
	"log"
	"net"
	"sync/atomic"
	"time"
)

// net.UDPConn or net.TCPConn
type tcpOrUdp interface {
}

type Client interface {
	Trace(m []byte)
	Ping(previousMessageSenderTime uint64)
}

type client struct {
	addr string
	conn *net.TCPConn
	ctx  context.Context
	cf   func()

	// use atomics
	closed uint32
}

func NewTcpClient(addr string) (c Client, err error) {
	d := net.Dialer{
		Timeout: 5 * time.Second,
	}
	/*
		ta, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			return nil, err
		}
		conn, err := net.DialTCP("tcp", nil, ta)
		if err != nil {
			return nil, err
		}
	*/
	ctx, cf := context.WithCancel(context.Background())
	gconn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		cf()
		return nil, err
	}
	conn := gconn.(*net.TCPConn)
	err = conn.SetNoDelay(true)
	if err != nil {
		cf()
		conn.Close()
		return nil, err
	}
	out := client{
		addr: addr,
		conn: conn,
		ctx:  ctx,
		cf:   cf,
	}
	go out.ReadThread()
	return &out, nil
}

var Epoch time.Time

func init() {
	Epoch = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
}

const (
	MessagePing  = 1
	MessageTrace = 2

	MaxMessageLength = 100
	MaxRecordLength  = 1 + binary.MaxVarintLen64 + binary.MaxVarintLen64 + MaxMessageLength
)

func (c *client) Trace(m []byte) {
	if c == nil {
		return
	}
	if c.conn == nil {
		// TODO: try to reconnect?
		return
	}
	msg := BuildTrace(EpochMicroseconds(), m)
	_, err := c.conn.Write(msg)
	if err != nil {
		// TODO: log error
		c.Close()
	}
}

func EpochMicroseconds() uint64 {
	dt := time.Now().Sub(Epoch).Microseconds()
	if dt < 0 {
		panic("negative time since epoch")
	}
	return uint64(dt)
}

func BuildPing(now, otherTime uint64) []byte {
	msg := make([]byte, 1+binary.MaxVarintLen64+binary.MaxVarintLen64)
	msg[0] = MessagePing
	pos := 1
	pos += binary.PutUvarint(msg[pos:], now)
	pos += binary.PutUvarint(msg[pos:], otherTime)
	return msg[:pos]
}

var (
	ErrBadPingTheirTime  = errors.New("bad ping record in their time")
	ErrBadPingMyTime     = errors.New("bad ping record in my time")
	ErrBadTraceTheirTime = errors.New("bad trace record in their time")
	ErrBadTraceLength    = errors.New("bad trace record in length")
)

func ParsePing(rb []byte) (theirTime, myTime uint64, err error) {
	pos := 1
	var blen int
	theirTime, blen = binary.Uvarint(rb[pos:])
	if blen <= 0 {
		err = ErrBadPingTheirTime
		return
	}
	pos += blen
	myTime, blen = binary.Uvarint(rb[pos:])
	if blen <= 0 {
		err = ErrBadPingMyTime
		return
	}
	return
}

func BuildTrace(now uint64, m []byte) []byte {
	msg := make([]byte, 1+binary.MaxVarintLen64+binary.MaxVarintLen64+len(m))
	msg[0] = MessageTrace
	pos := 1
	pos += binary.PutUvarint(msg[pos:], now)
	pos += binary.PutUvarint(msg[pos:], uint64(len(m)))
	copy(msg[pos:], m)
	pos += len(m)
	return msg[:pos]
}

func ParseTrace(rb []byte) (theirTime uint64, m []byte, err error) {
	pos := 1
	var blen int
	theirTime, blen = binary.Uvarint(rb[pos:])
	if blen <= 0 {
		err = ErrBadTraceTheirTime
		return
	}
	pos += blen
	var mlen uint64
	mlen, blen = binary.Uvarint(rb[pos:])
	if blen <= 0 {
		err = ErrBadTraceLength
		return
	}
	pos += blen
	m = rb[pos : pos+int(mlen)]
	return
}

func (c *client) Ping(otherTime uint64) {
	if c == nil {
		return
	}
	if c.conn == nil {
		// TODO: try to reconnect?
		return
	}
	msg := BuildPing(EpochMicroseconds(), otherTime)
	_, err := c.conn.Write(msg)
	if err != nil {
		// TODO: log error
		c.Close()
	}
}

func (c *client) ReadThread() {
	buf := make([]byte, MaxRecordLength)
	for {
		if c == nil {
			return
		}
		if c.conn == nil {
			return
		}
		rlen, err := c.conn.Read(buf)
		if err != nil {
			xc := atomic.LoadUint32(&c.closed)
			if xc == 0 {
				log.Printf("%s: read %s", c.addr, err)
			}
			return
		}
		rb := buf[:rlen]
		switch rb[0] {
		case MessagePing:
			myTime, theirTime, err := ParsePing(rb)
			if err != nil {
				log.Print(err)
				c.Close()
				return
			}
			// TODO: log.debug of round trip time
			log.Printf("round trip time %d microseconds", EpochMicroseconds()-myTime)
			// reply:
			c.Ping(theirTime)
		default:
			// TODO: log unknown message
			c.Close()
		}
	}
}

func (c *client) Close() {
	if c == nil || c.conn == nil {
		return
	}
	if atomic.CompareAndSwapUint32(&c.closed, 0, 1) {
		c.conn.Close()
		c.conn = nil
	}
}

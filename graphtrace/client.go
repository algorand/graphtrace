package graphtrace

import (
	"context"
	"encoding/binary"
	"errors"
	"log"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// net.UDPConn or net.TCPConn
type tcpOrUdp interface {
}

// Client connects to a graph trace server and sends it messages.
type Client interface {
	Trace(m []byte) error
	Ping(previousMessageSenderTime uint64) error
}

type client struct {
	addr string
	conn *net.TCPConn
	ctx  context.Context
	cf   func()

	// use atomics
	closed uint32

	lock sync.Mutex
}

// NewTcpClient oppens a connection to a trace server.
//
// A thread is started to handle ping protocol messages so that this
// client and the server can detect clock difference and round trip
// time.
func NewTcpClient(addr string) (c Client, err error) {
	ctx, cf := context.WithCancel(context.Background())
	out := client{
		addr: addr,
		conn: nil,
		ctx:  ctx,
		cf:   cf,
	}
	go out.readLoop()
	return &out, nil
}

// Epoch is the zero time of the system.
// Times are measured as microseconds from Epoch.
// Epoch is currently 2020-10-01 00:00:00 UTC
// Major versions will probably update Epoch to just before the release of the version.
var Epoch time.Time

func init() {
	Epoch = time.Date(2020, 10, 1, 0, 0, 0, 0, time.UTC)
}

const (
	MessagePing  = 1
	MessageTrace = 2

	MaxMessageLength = 100
	MaxRecordLength  = 1 + binary.MaxVarintLen64 + binary.MaxVarintLen64 + MaxMessageLength
)

func retryTime() time.Duration {
	return time.Duration(4000+rand.Intn(2000)) * time.Millisecond
}

var errDone = errors.New("Done")

var clientDialer = net.Dialer{
	Timeout: 5 * time.Second,
}

func (c *client) connect() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.conn != nil {
		return nil
	}
	gconn, err := clientDialer.DialContext(c.ctx, "tcp", c.addr)
	if err != nil {
		return err
	}
	conn := gconn.(*net.TCPConn)
	err = conn.SetNoDelay(true)
	if err != nil {
		conn.Close()
		return err
	}
	atomic.StoreUint32(&c.closed, 0)
	c.conn = conn
	return nil
}

func (c *client) write(msg []byte) error {
	if c.conn == nil {
		return ErrNotConnected
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.conn == nil {
		return ErrNotConnected
	}
	_, err := c.conn.Write(msg)
	if err != nil {
		c.conn.Close()
		atomic.StoreUint32(&c.closed, 1)
		c.conn = nil
	}
	return err
}

func (c *client) Trace(m []byte) error {
	if c.conn == nil {
		return nil
	}
	msg := BuildTrace(EpochMicroseconds(), m)
	return c.write(msg)
}

// EpochMicroseconds is the microseconds since Epoch.
// If the value would be negative it will panic().
func EpochMicroseconds() uint64 {
	dt := time.Now().Sub(Epoch).Microseconds()
	if dt < 0 {
		panic("negative time since epoch")
	}
	return uint64(dt)
}

// BuildPing packs bytes for a ping message.
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
	ErrNotConnected      = errors.New("not connected")
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

func (c *client) Ping(otherTime uint64) error {
	if c.conn == nil {
		return nil
	}
	msg := BuildPing(EpochMicroseconds(), otherTime)
	return c.write(msg)
}

func (c *client) readLoop() {
	buf := make([]byte, MaxRecordLength)
	// startup pause to dither the stampeding herd
	time.Sleep(time.Duration(10+rand.Intn(50)) * time.Millisecond)
	for {
		// connect phase
		for {
			select {
			case <-c.ctx.Done():
				return
			default:
			}
			err := c.connect()
			if err == nil {
				break
			}
			time.Sleep(retryTime())
		}
		// read loop
		for {
			select {
			case <-c.ctx.Done():
				return
			default:
			}
			c.lock.Lock()
			rc := c.conn
			if rc == nil {
				// disconnected due to protocol hiccup
				break
			}
			c.lock.Unlock()
			rlen, err := rc.Read(buf)
			if err != nil {
				xc := atomic.LoadUint32(&c.closed)
				if xc == 0 {
					// if not closing, log the error
					log.Printf("%s: read %s", c.addr, err)
				}
				break
			}
			rb := buf[:rlen]
			switch rb[0] {
			case MessagePing:
				theirTime, myTime, err := ParsePing(rb)
				if err != nil {
					log.Print(err)
					c.Close()
					break
				}
				// TODO: log.debug of round trip time
				if myTime != 0 {
					log.Printf("round trip time %d microseconds", EpochMicroseconds()-myTime)
				}
				// reply:
				err = c.Ping(theirTime)
				if err != nil {
					// already closed inside Ping()
					break
				}
			default:
				// TODO: log unknown message
				c.Close()
			}
		}
	}
}

func (c *client) Close() {
	if c.conn == nil {
		return
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.conn == nil {
		return
	}
	if atomic.CompareAndSwapUint32(&c.closed, 0, 1) {
		c.conn.Close()
		c.conn = nil
	}
}

type NopClient struct {
}

func (nop *NopClient) Trace(m []byte) error {
	return nil
}
func (nop *NopClient) Ping(previousMessageSenderTime uint64) error {
	return nil
}

var NopClientSingleton NopClient

// NewNopClient returns a Client that will very quickly do nothing on Trace() or Ping()
func NewNopClient() Client {
	return &NopClientSingleton
}

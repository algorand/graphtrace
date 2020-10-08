package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/algorand/graphtrace/graphtrace"
)

type server struct {
	addr string
	ln   net.Listener
	ctx  context.Context
	cf   func()
}

func (s *server) Run() {
	defer s.cf()
	lc := net.ListenConfig{}
	var err error
	s.ln, err = lc.Listen(s.ctx, "tcp", s.addr)
	if err != nil {
		log.Printf("%s: listen, %s", s.addr, err)
		return
	}
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			log.Printf("%s: accept, %s", s.addr, err)
			return
		}
		cl := &client{conn.(*net.TCPConn), s.ctx}
		go cl.Run()
	}
}

type client struct {
	conn *net.TCPConn
	ctx  context.Context
}

func (c *client) Run() {
	buf := make([]byte, graphtrace.MaxRecordLength)
	for {
		select {
		case <-c.ctx.Done():
			c.Close()
			return
		default:
		}
		blen, err := c.conn.Read(buf)
		if err != nil {
			log.Printf("%s: read %s", c.conn.RemoteAddr(), err)
			c.Close()
			return
		}
		rb := buf[:blen]
		switch rb[0] {
		case graphtrace.MessagePing:
		case graphtrace.MessageTrace:
		default:
			log.Printf("%s: bad msg 0x%x", c.conn.RemoteAddr(), rb[0])
			c.Close()
			return
		}
	}
}
func (c *client) Close() {
	if c.conn == nil {
		return
	}
	c.conn.Close()
	c.conn = nil
}

func main() {
	ctx, cf := context.WithCancel(context.Background())
	s := server{
		addr: ":3372",
		ctx:  ctx,
		cf:   cf,
	}
	s.Run()
}

func maybeFail(err error, errfmt string, params ...interface{}) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, errfmt, params...)
	os.Exit(1)
}

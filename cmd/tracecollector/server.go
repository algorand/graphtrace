package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/algorand/graphtrace/graphtrace"
)

var verbose bool = false

func debug(format string, args ...interface{}) {
	if verbose == false {
		return
	}
	log.Printf(format, args...)
}

type server struct {
	addr string
	ln   net.Listener
	ctx  context.Context
	cf   func()

	out  io.Writer
	lock sync.Mutex
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
		cl := &client{
			conn: conn.(*net.TCPConn),
			ctx:  s.ctx,
			s:    s,
		}
		go cl.Run()
	}
}

func (s *server) trace(addr *net.TCPAddr, t int64, m []byte) {
	msg := fmt.Sprintf("%d\t%s\t%d\t%s\n", t, addr.IP.String(), addr.Port, base64.StdEncoding.EncodeToString(m))
	s.lock.Lock()
	defer s.lock.Unlock()
	_, err := s.out.Write([]byte(msg))
	if err != nil {
		log.Printf("record log err: %s", err)
		s.cf()
	}
}

type client struct {
	conn *net.TCPConn
	ctx  context.Context

	// their time + offset == our time
	// offset = (our time) - (their time)
	offset int64

	s *server
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
			if err != io.EOF {
				log.Printf("%s: read %s", c.conn.RemoteAddr(), err)
			}
			c.Close()
			return
		}
		rb := buf[:blen]
		switch rb[0] {
		case graphtrace.MessagePing:
			err = c.handlePing(rb)
			if err != nil {
				log.Printf("%s: bad ping %s", c.conn.RemoteAddr(), err)
				c.Close()
				return
			}
		case graphtrace.MessageTrace:
			err = c.handleTrace(rb)
			if err != nil {
				log.Printf("%s: bad trace %s", c.conn.RemoteAddr(), err)
				c.Close()
				return
			}
		default:
			log.Printf("%s: bad msg 0x%x", c.conn.RemoteAddr(), rb[0])
			c.Close()
			return
		}
	}
}
func (c *client) handlePing(rb []byte) error {
	now := graphtrace.EpochMicroseconds()
	theirTime, myTime, err := graphtrace.ParsePing(rb)
	if err != nil {
		return err
	}
	if myTime == 0 {
		c.offset = int64(now) - int64(theirTime)
		msg := graphtrace.BuildPing(now, theirTime)
		_, err = c.conn.Write(msg)
		if err != nil {
			return err
		}
	} else {
		roundTripMicros := now - myTime
		c.offset = int64(now) - int64(theirTime+(roundTripMicros/2))
		debug("%s rtt=%d µs, offset=%d µs", c.conn.RemoteAddr(), roundTripMicros, c.offset)
	}
	return nil
}
func (c *client) handleTrace(rb []byte) error {
	now := int64(graphtrace.EpochMicroseconds())
	theirTime, msg, err := graphtrace.ParseTrace(rb)
	if err != nil {
		return err
	}
	adjTime := int64(theirTime) + c.offset
	if adjTime < now {
		adjTime = now
	}
	c.s.trace(c.conn.RemoteAddr().(*net.TCPAddr), adjTime, msg)
	return nil
}
func (c *client) Close() {
	if c.conn == nil {
		return
	}
	c.conn.Close()
	c.conn = nil
}

// math package only has min/max values for explicitly sized int
var m1 = -1
var maxInt = int(uint64(m1) >> 1)

func main() {
	var serveAddr string
	var dataPath string
	var rotateSize int
	var archivePath string
	flag.StringVar(&serveAddr, "addr", ":6525", ":port or host:port to serve on")
	flag.StringVar(&dataPath, "out", "-", "path to write data to, \"-\" for stdout (default)")
	flag.IntVar(&rotateSize, "rotateSize", maxInt, "number of bytes of main output file after which to rotate the output file")
	flag.StringVar(&archivePath, "outArchive", "", "path to move old files to. {{s}} for unix seconds")
	flag.BoolVar(&verbose, "verbose", false, "verbose logging")
	flag.Parse()

	var out io.Writer
	if dataPath == "-" {
		out = os.Stdout
	} else {
		lw := &LogWriter{
			BreakSize:     rotateSize,
			OutPath:       dataPath,
			ArchiveFormat: archivePath,
		}
		if strings.HasSuffix(archivePath, ".gz") {
			lw.Gzip = true
		}
		out = lw
	}
	ctx, cf := context.WithCancel(context.Background())
	s := server{
		addr: serveAddr,
		ctx:  ctx,
		cf:   cf,
		out:  out,
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

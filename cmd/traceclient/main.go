package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/algorand/graphtrace/graphtrace"
)

func main() {
	var serverAddr string
	var message string
	flag.StringVar(&serverAddr, "addr", "localhost:6525", "host:port of tracecollector server")
	flag.StringVar(&message, "m", "hello world", "string payload message to send")
	flag.Parse()
	c, err := graphtrace.NewTcpClient(serverAddr)
	maybeFail(err, "%s: could not connect, %s", serverAddr, err)
	err = c.Ping(0)
	maybeFail(err, "%s: ping, %s", serverAddr, err)
	time.Sleep(500 * time.Millisecond)

	c.Trace([]byte(message))
}

func maybeFail(err error, errfmt string, params ...interface{}) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, errfmt, params...)
	os.Exit(1)
}

package main

import (
	"fmt"
	"os"

	"github.com/algorand/graphtrace/graphtrace"
)

func main() {
	addr := "localhost:3372"
	c, err := graphtrace.NewTcpClient(addr)
	maybeFail(err, "%s: could not connect, %s", addr, err)
	c.Trace([]byte("hello world"))
}

func maybeFail(err error, errfmt string, params ...interface{}) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, errfmt, params...)
	os.Exit(1)
}

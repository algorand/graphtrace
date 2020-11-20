package main

import (
	"compress/gzip"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// LogWriter implements io.Writer but rotates away old chunks of log
type LogWriter struct {
	// BreakSize is the number of bytes after which to start a new log file
	BreakSize int

	// OutPath is where to write the current log file
	// `{{s}}` will be replaced by unix-time seconds since 1970-01-01 00:00:00 UTC
	OutPath string

	// ArchiveFormat is what log files will be renamed to after closing
	// `{{s}}` will be replaced by unix-time seconds since 1970-01-01 00:00:00 UTC
	ArchiveFormat string

	// Gzip directs renamed files to be compressed with gzip
	Gzip bool

	outname  string
	out      io.WriteCloser
	thisSize int
}

// Write implements io.Writer
func (lw *LogWriter) Write(data []byte) (n int, err error) {
	if lw.out == nil {
		lw.outname = formatOutputPath(lw.OutPath)
		lw.out, err = os.Create(lw.outname)
		if err != nil {
			lw.out = nil
			return
		}
	}
	n, err = lw.out.Write(data)
	if err != nil {
		return
	}
	lw.thisSize += n
	if lw.thisSize > lw.BreakSize {
		err = lw.Break()
	}
	return
}

func formatOutputPath(format string) string {
	out := format
	if strings.Contains(out, "{{s}}") {
		out = strings.ReplaceAll(out, "{{s}}", strconv.FormatInt(time.Now().Unix(), 10))
	}
	return out
}

// Break closes the current log output, moves it aside, and starts a new log output
func (lw *LogWriter) Break() (err error) {
	err = lw.out.Close()
	lw.thisSize = 0
	lw.out = nil
	if err != nil {
		return
	}
	if lw.ArchiveFormat == "" && lw.Gzip == false {
		return
	}
	var outpath string
	if lw.ArchiveFormat == "" {
		outpath = lw.outname
	} else {
		outpath = formatOutputPath(lw.ArchiveFormat)
	}
	outpathRaw := outpath
	if lw.Gzip {
		outpathRaw = outpath + ".uncompressed"
	}
	err = os.Rename(lw.outname, outpathRaw)
	if err != nil {
		return
	}
	if lw.Gzip {
		go dogzip(outpathRaw, outpath)
	}
	return
}

// TODO: log errors
func dogzip(from, to string) {
	out, err := os.Create(to)
	cleanup := func() {
		if out != nil {
			out.Close()
		}
		os.Remove(to)
	}
	if err != nil {
		cleanup()
		return
	}
	gzo := gzip.NewWriter(out)
	fin, err := os.Open(from)
	if err != nil {
		cleanup()
		return
	}
	_, err = io.Copy(gzo, fin)
	if err != nil {
		cleanup()
		return
	}
	err = gzo.Close()
	if err != nil {
		cleanup()
		return
	}
	err = out.Close()
	if err != nil {
		cleanup()
		return
	}
	os.Remove(from)
}

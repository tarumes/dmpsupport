package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

var bindir string

func init() {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	bindir = filepath.Dir(ex) + string(os.PathSeparator)

	// log handler to file and console out
	logFile, err := os.OpenFile(bindir+"logfile.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile) // | log.Lmicroseconds  )
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
}

package main

import (
	"bytes"
)

func lastLine(s []byte) []byte {
	s = bytes.TrimRight(s, "\n")
	pieces := bytes.Split(s, []byte("\n"))
	return pieces[len(pieces)-1]
}

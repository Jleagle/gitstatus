package main

import (
	"bytes"
)

func boolP(b bool) *bool {
	return &b
}

func stringP(b string) *string {
	return &b
}

func firstLine(s []byte) []byte {
	pieces := bytes.Split(s, []byte("\n"))
	return pieces[0]
}

func lastLine(s []byte) []byte {
	s = bytes.TrimRight(s, "\n")
	pieces := bytes.Split(s, []byte("\n"))
	return pieces[len(pieces)-1]
}

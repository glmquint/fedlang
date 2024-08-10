package main

import (
	"os"
	"strings"
)

func main() {
	str := "Hello world " + strings.Join(os.Args, ", ")
	fo, err := os.Create("output.txt")
	if err != nil {
		panic(err)
	}
	// close fo on exit and check for its returned error
	defer func() {
		if err := fo.Close(); err != nil {
			panic(err)
		}
	}()

	// make a buffer to keep chunks that are read
	// write a chunk
	if _, err := fo.WriteString(str); err != nil {
		panic(err)
	}
}

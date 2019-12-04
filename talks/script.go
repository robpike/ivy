package main // import "robpike.io/ivy/talks"

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
)

func main() {
	flag.Parse()
	if flag.NArg() != 2 {
		log.Fatal("Usage: script program filename")
	}
	text, err := ioutil.ReadFile(flag.Arg(1))
	ck(err)
	cmd := exec.Command(flag.Arg(0))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	input, err := cmd.StdinPipe()
	ck(err)
	err = cmd.Start()
	ck(err)
	scan := bufio.NewScanner(os.Stdin)
	for scan.Scan() {
		// User typed something; step back across the newline.
		if len(scan.Bytes()) > 0 {
			// User typed a non-empty line of text; send that.
			line := []byte(fmt.Sprintf("%s\n", scan.Bytes()))
			_, err = input.Write(line)
		} else {
			// User typed newline; send next line of file's text.
			if len(text) == 0 {
				break
			}
			for i := 0; i < len(text); i++ {
				if text[i] == '\n' {
					os.Stdout.Write(text[:i+1])
					_, err = input.Write(text[:i+1])
					text = text[i+1:]
					break
				}
			}
		}
		ck(err)
	}
	ck(scan.Err())
}

func ck(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

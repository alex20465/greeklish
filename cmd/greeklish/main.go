// Command greeklish converts Greeklish to Greek, fully offline.
//
//	greeklish gia sou ime o alexandros
//	echo "ti kaneis?" | greeklish
//
// Every conversion is appended to a local history file, see historyPath.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/alex20465/greeklish/model"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println(version)
		return
	}
	m, err := model.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	hist, err := openHistory()
	if err != nil {
		fmt.Fprintln(os.Stderr, "greeklish: history disabled:", err)
	} else {
		defer hist.Close()
	}
	convert := func(in string) string {
		out := m.Convert(in)
		if hist != nil {
			if err := hist.add(in, out); err != nil {
				fmt.Fprintln(os.Stderr, "greeklish: history disabled:", err)
				hist = nil
			}
		}
		return out
	}

	if len(os.Args) > 1 {
		fmt.Println(convert(strings.Join(os.Args[1:], " ")))
		return
	}
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<20)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for in.Scan() {
		fmt.Fprintln(out, convert(in.Text()))
		out.Flush()
	}
	if err := in.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

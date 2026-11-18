// Command train builds the embedded model from Greek corpora.
//
//	train -o model.bin -en en_50k.txt -heldout testdata/heldout.txt -train-only subs.txt corpus...
//
// Corpus files hold one sentence per line, optionally prefixed with
// "id<TAB>" as in the Leipzig corpora. Every heldoutEvery-th clean sentence
// is written to the held-out file instead of being trained on, except from
// -train-only files, whose license does not allow publishing their text.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"iter"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/alex20465/greeklish"
)

const (
	heldoutEvery   = 500
	heldoutPerFile = 1000
)

func main() {
	out := flag.String("o", "model.bin", "output model file")
	enPath := flag.String("en", "", "English word frequency list: word count per line")
	heldoutPath := flag.String("heldout", "", "file receiving held-out test sentences")
	trainOnly := flag.String("train-only", "", "comma separated corpus files never used for held-out sentences")
	flag.Parse()
	noHeldout := map[string]bool{}
	for _, f := range strings.Split(*trainOnly, ",") {
		noHeldout[f] = true
	}
	files := flag.Args()
	if len(files) == 0 || *enPath == "" {
		flag.Usage()
		os.Exit(2)
	}

	english, err := readEnglish(*enPath)
	if err != nil {
		log.Fatal(err)
	}
	var heldout *bufio.Writer
	if *heldoutPath != "" {
		f, err := os.Create(*heldoutPath)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		heldout = bufio.NewWriter(f)
		defer heldout.Flush()
	}

	start := time.Now()
	pass := 0
	corpus := func() iter.Seq[string] {
		pass++
		log.Printf("pass %d", pass)
		w := heldout
		if pass > 1 {
			w = nil
		}
		return sentences(files, noHeldout, w)
	}
	m := greeklish.Train(corpus, english)
	data, err := m.MarshalBinary()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s: %.1f MB in %s", *out, float64(len(data))/1e6, time.Since(start).Round(time.Second))
}

func sentences(files []string, noHeldout map[string]bool, heldout *bufio.Writer) iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, path := range files {
			f, err := os.Open(path)
			if err != nil {
				log.Fatal(err)
			}
			sc := bufio.NewScanner(f)
			sc.Buffer(make([]byte, 1<<20), 1<<20)
			line, kept := 0, 0
			for sc.Scan() {
				s := clean(sc.Text())
				line++
				if !noHeldout[path] && line%heldoutEvery == 0 && kept < heldoutPerFile && testable(s) {
					kept++
					if heldout != nil {
						fmt.Fprintln(heldout, s)
					}
					continue
				}
				if !yield(s) {
					f.Close()
					return
				}
			}
			f.Close()
			if err := sc.Err(); err != nil {
				log.Fatal(err)
			}
		}
	}
}

func clean(s string) string {
	if tab := strings.IndexByte(s, '\t'); tab >= 0 {
		if _, err := strconv.Atoi(s[:tab]); err == nil {
			s = s[tab+1:]
		}
	}
	return strings.TrimLeft(strings.TrimSpace(s), "-– ")
}

// testable keeps plain Greek sentences of a conversational length.
func testable(s string) bool {
	words := strings.Fields(s)
	if len(words) < 3 || len(words) > 20 {
		return false
	}
	for _, r := range s {
		if unicode.IsLetter(r) && !unicode.Is(unicode.Greek, r) || unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func readEnglish(path string) (map[string]int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	english := map[string]int{}
	for line := range strings.Lines(string(data)) {
		word, count, ok := strings.Cut(strings.TrimSpace(line), " ")
		n, err := strconv.Atoi(count)
		if ok && err == nil && isASCIIWord(word) {
			english[word] += n
		}
	}
	return english, nil
}

func isASCIIWord(s string) bool {
	for _, r := range s {
		if (r < 'a' || r > 'z') && r != '\'' {
			return false
		}
	}
	return s != ""
}

package greeklish

import (
	"bufio"
	"fmt"
	"math/rand/v2"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// A style is one way people type Greeklish. Each Greek unit maps to the
// spellings that style uses; one is picked at random per occurrence.
type style struct {
	name    string
	units   map[string][]string
	typos   float64 // probability of a typo in a word of 5+ letters
	english bool    // insert English terms that must survive
}

var common = map[string][]string{
	"α": {"a"}, "β": {"v"}, "γ": {"g"}, "δ": {"d"}, "ε": {"e"}, "ζ": {"z"}, "κ": {"k"}, "λ": {"l"},
	"μ": {"m"}, "ν": {"n"}, "π": {"p"}, "ρ": {"r"}, "σ": {"s"}, "τ": {"t"}, "φ": {"f"}, "ψ": {"ps"},
	"μπ": {"b", "mp"}, "ντ": {"d", "nt"}, "γκ": {"g", "gk"}, "γγ": {"gg", "g"}, "'": {"'"},
}

func with(extra map[string][]string) map[string][]string {
	out := map[string][]string{}
	for k, v := range common {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

var styles = []style{
	{name: "phonetic", units: with(map[string][]string{
		"η": {"i"}, "ι": {"i"}, "υ": {"i", "y"}, "ω": {"o"}, "ο": {"o"}, "ει": {"i"}, "οι": {"i"}, "αι": {"e"},
		"ου": {"ou", "u"}, "θ": {"th"}, "χ": {"x", "ch", "h"}, "ξ": {"ks", "x"}, "αυ": {"av", "af"}, "ευ": {"ev", "ef"},
	})},
	{name: "orthographic", units: with(map[string][]string{
		"η": {"h", "i"}, "ι": {"i"}, "υ": {"y", "u"}, "ω": {"w"}, "ο": {"o"}, "ει": {"ei"}, "οι": {"oi"}, "αι": {"ai"},
		"ου": {"ou"}, "θ": {"th"}, "χ": {"x", "ch"}, "ξ": {"ks", "x"}, "αυ": {"au", "av", "af"}, "ευ": {"eu", "ev", "ef"},
	})},
	{name: "lazy", units: with(map[string][]string{
		"η": {"h", "i"}, "ι": {"i"}, "υ": {"u", "i"}, "ω": {"w", "o"}, "ο": {"o"}, "ει": {"ei", "i"}, "οι": {"oi", "i"}, "αι": {"ai", "e"},
		"ου": {"ou"}, "θ": {"8", "th"}, "χ": {"x"}, "ξ": {"3", "ks"}, "αυ": {"au", "af"}, "ευ": {"eu", "ef"},
	})},
}

// englishTerms are words people keep in English inside Greek messages.
// Words that are also Greeklish ("file" for φίλε) are left out on purpose.
var englishTerms = strings.Fields(`deploy build merge release commit branch server meeting email link laptop
update backup bug feature ticket call deadline online offline password login cloud database client project
team manager report test review sprint staging production cache frontend backend API docker pipeline`)

// codeSwitch inserts English terms between the words of a Greek sentence.
func codeSwitch(s string, rng *rand.Rand) string {
	fields := strings.Fields(s)
	for range 1 + rng.IntN(2) {
		i := rng.IntN(len(fields) + 1)
		fields = append(fields[:i], append([]string{englishTerms[rng.IntN(len(englishTerms))]}, fields[i:]...)...)
	}
	return strings.Join(fields, " ")
}

// greeklishify writes a Greek sentence the way the style would type it.
func greeklishify(s string, st style, rng *rand.Rand) string {
	var b strings.Builder
	for _, t := range tokenize(s) {
		if t.space {
			b.WriteByte(' ')
		}
		if t.kind != kindWord || latinWord(t.text) {
			b.WriteString(strings.ReplaceAll(t.text, ";", "?"))
			continue
		}
		w := romanizeWith(stripGreek(t.text), st, rng)
		if st.typos > 0 && len(w) >= 5 && rng.Float64() < st.typos {
			w = typo(w, rng)
		}
		b.WriteString(w)
	}
	return b.String()
}

func romanizeWith(word string, st style, rng *rand.Rand) string {
	var b strings.Builder
	rs := []rune(word)
	for i := 0; i < len(rs); i++ {
		if i+1 < len(rs) {
			if opts, ok := st.units[string(rs[i:i+2])]; ok {
				b.WriteString(opts[rng.IntN(len(opts))])
				i++
				continue
			}
		}
		if opts, ok := st.units[string(rs[i])]; ok {
			b.WriteString(opts[rng.IntN(len(opts))])
		} else {
			b.WriteRune(rs[i])
		}
	}
	return b.String()
}

func typo(w string, rng *rand.Rand) string {
	i := 1 + rng.IntN(len(w)-2)
	switch rng.IntN(3) {
	case 0:
		return w[:i] + w[i+1:]
	case 1:
		return w[:i] + w[i+1:i+2] + w[i:i+1] + w[i+2:]
	default:
		return w[:i] + w[i:i+1] + w[i:]
	}
}

func readHeldout(t testing.TB, limit int) []string {
	f, err := os.Open("testdata/heldout.txt")
	if err != nil {
		t.Fatalf("held-out sentences missing, run make model: %v", err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() && len(out) < limit {
		if s := sc.Text(); wellSpelled(s) {
			out = append(out, s)
		}
	}
	return out
}

// variants are spellings the dictionary accepts both ways.
var variants = map[string]string{"μία": "μια", "δύο": "δυο"}

// wellSpelled rejects reference sentences that are themselves misspelled:
// a word of two or more syllables without an accent, an accented
// monosyllable other than ή, πού, πώς, or a broken final ν rule.
func wellSpelled(s string) bool {
	ws := words(s)
	for i, w := range ws {
		polysyllabic := syllables(stripGreek(w)) >= 2
		if !hasAccent(w) && polysyllabic || hasAccent(w) && !polysyllabic && w != "ή" && w != "πού" && w != "πώς" {
			return false
		}
		if i+1 < len(ws) && finalNu(w, ws[i+1]) != w {
			return false
		}
	}
	return true
}

func words(s string) []string {
	var out []string
	for _, t := range tokenize(s) {
		if t.kind == kindWord {
			out = append(out, lowerGreek(t.text))
		}
	}
	return out
}

type score struct {
	words, correct, unaccented, sentences, exact int
	errors                                       map[string]int
}

func (s *score) add(got, want string) {
	g, w := words(got), words(want)
	s.sentences++
	if len(g) != len(w) {
		s.words += len(w)
		return
	}
	exact := true
	for i := range w {
		s.words++
		if g[i] == w[i] || variants[g[i]] == w[i] || variants[w[i]] == g[i] {
			s.correct++
			s.unaccented++
			continue
		}
		exact = false
		if stripGreek(g[i]) == stripGreek(w[i]) {
			s.unaccented++
		}
		s.errors[w[i]+" → "+g[i]]++
	}
	if exact {
		s.exact++
	}
}

func (s *score) wordAccuracy() float64 { return float64(s.correct) / float64(s.words) }

func (s *score) report(t *testing.T, name string) {
	t.Logf("%-14s words %.2f%%  ignoring accents %.2f%%  exact sentences %.2f%%  (%d sentences)",
		name, 100*s.wordAccuracy(), 100*float64(s.unaccented)/float64(s.words),
		100*float64(s.exact)/float64(s.sentences), s.sentences)
	type kv struct {
		k string
		v int
	}
	var top []kv
	for k, v := range s.errors {
		top = append(top, kv{k, v})
	}
	sort.Slice(top, func(i, j int) bool { return top[i].v > top[j].v })
	var b strings.Builder
	for _, e := range top[:min(15, len(top))] {
		fmt.Fprintf(&b, "  %dx %s\n", e.v, e.k)
	}
	t.Logf("most frequent errors:\n%s", b.String())
}

// TestStress measures word accuracy on sentences the model never saw, typed
// in every style. The thresholds are regression guards set below the
// measured accuracy, see the logged numbers with go test -v.
func TestStress(t *testing.T) {
	m := loadModel(t)
	sentences := readHeldout(t, 4000)
	typoStyle := styles[0]
	typoStyle.name, typoStyle.typos = "phonetic+typos", 0.15
	switchStyle := styles[1]
	switchStyle.name, switchStyle.english = "code-switch", true
	thresholds := map[string]float64{"phonetic": 0.96, "orthographic": 0.965, "lazy": 0.963, "phonetic+typos": 0.95, "code-switch": 0.96}

	for i, st := range append(styles, typoStyle, switchStyle) {
		t.Run(st.name, func(t *testing.T) {
			t.Parallel()
			rng := rand.New(rand.NewPCG(uint64(i), 42))
			sc := &score{errors: map[string]int{}}
			for _, s := range sentences {
				if st.english {
					s = codeSwitch(s, rng)
				}
				sc.add(m.Convert(greeklishify(s, st, rng)), s)
			}
			sc.report(t, st.name)
			if acc := sc.wordAccuracy(); acc < thresholds[st.name] {
				t.Errorf("word accuracy %.2f%% below %.1f%%", 100*acc, 100*thresholds[st.name])
			}
		})
	}
}

// TestStressGreek checks accent restoration and spelling correction on
// Greek typed without accents, with ι/η/ο/ω slips, and already correct.
func TestStressGreek(t *testing.T) {
	m := loadModel(t)
	sentences := readHeldout(t, 4000)
	slips := strings.NewReplacer("η", "ι", "ω", "ο", "αι", "ε")
	cases := []struct {
		name      string
		threshold float64
		input     func(s string, rng *rand.Rand) string
	}{
		{"correct", 0.99, func(s string, _ *rand.Rand) string { return s }},
		{"no-accents", 0.965, func(s string, _ *rand.Rand) string { return stripAccents(s) }},
		{"slips", 0.965, func(s string, rng *rand.Rand) string {
			fields := strings.Fields(stripAccents(s))
			for i, f := range fields {
				if rng.Float64() < 0.2 {
					fields[i] = slips.Replace(f)
				}
			}
			return strings.Join(fields, " ")
		}},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			rng := rand.New(rand.NewPCG(uint64(i), 7))
			sc := &score{errors: map[string]int{}}
			for _, s := range sentences {
				sc.add(m.Convert(c.input(s, rng)), s)
			}
			sc.report(t, c.name)
			if acc := sc.wordAccuracy(); acc < c.threshold {
				t.Errorf("word accuracy %.2f%% below %.1f%%", 100*acc, 100*c.threshold)
			}
		})
	}
}

// TestSpeed guards the "super fast" requirement on a single core.
func TestSpeed(t *testing.T) {
	m := loadModel(t)
	sentences := readHeldout(t, 1000)
	rng := rand.New(rand.NewPCG(1, 1))
	inputs := make([]string, len(sentences))
	nwords := 0
	for i, s := range sentences {
		inputs[i] = greeklishify(s, styles[0], rng)
		nwords += len(strings.Fields(s))
	}
	start := time.Now()
	for _, in := range inputs {
		m.Convert(in)
	}
	per := time.Since(start) / time.Duration(nwords)
	t.Logf("%s per word, %d words", per, nwords)
	if per > time.Millisecond {
		t.Errorf("%s per word is too slow", per)
	}
}

func BenchmarkConvert(b *testing.B) {
	m := loadModel(b)
	in := "kosta, I ekdosi ine online 1.4.2 na to kanw release sto staging ?"
	for b.Loop() {
		m.Convert(in)
	}
}

// FuzzConvert checks that arbitrary input never panics and never loses
// or invents lines.
func FuzzConvert(f *testing.F) {
	m := loadModel(f)
	for _, s := range []string{"gia sou", "8elw 3erw", "a'b'c'", "...", "😀 ok", "Ελλ'ηνικά", "x\ny", "1.2.3.4.5", "ççç", "́̈"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if !utf8.ValidString(s) {
			return
		}
		out := m.Convert(s)
		if strings.Count(out, "\n") != strings.Count(s, "\n") {
			t.Errorf("line count changed: %q → %q", s, out)
		}
	})
}

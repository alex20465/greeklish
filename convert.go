package greeklish

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Convert turns Greeklish, rough Greek or a mix of both into Greek, line by
// line. English words, numbers, URLs and code are kept.
func (m *Model) Convert(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = m.convertLine(line)
	}
	return strings.Join(lines, "\n")
}

// acronym reports a short all-caps Latin word such as API, CI or AWS. In a
// line that is not shouted in capitals it is kept as written.
func acronym(t token) bool {
	n := utf8.RuneCountInString(t.text)
	return t.kind == kindWord && latinWord(t.text) && n >= 2 && n <= 5 && strings.ToUpper(t.text) == t.text
}

func sentenceEnd(t token) bool {
	return t.kind == kindPunct && strings.ContainsAny(t.text, ".!?;…")
}

func (m *Model) convertLine(line string) string {
	toks := tokenize(line)
	accents, caps, words := false, true, 0
	for _, t := range toks {
		accents = accents || (t.kind == kindWord && hasAccent(t.text))
		caps = caps && strings.ToUpper(t.text) == t.text
		if t.kind == kindWord {
			words++
		}
	}
	shouting := caps && words >= 2
	for i, t := range toks {
		if !shouting && acronym(t) {
			toks[i].kind = kindVerbatim
		}
	}
	out := make([]string, len(toks))
	start := 0
	for i := range toks {
		if sentenceEnd(toks[i]) || i == len(toks)-1 {
			m.convertSentence(toks[start:i+1], out[start:i+1], accents)
			start = i + 1
		}
	}
	return join(toks, out)
}

// slot is one language model position: a word with its candidates, or a
// fixed token such as a number or a comma.
type slot struct {
	tok   int
	cands []candidate
}

func (m *Model) convertSentence(toks []token, out []string, accents bool) {
	var slots []slot
	for i, t := range toks {
		out[i] = t.text
		switch {
		case t.kind == kindWord:
			slots = append(slots, slot{i, m.candidates(t.text, accents)})
		case t.kind == kindVerbatim:
			slots = append(slots, slot{i, []candidate{{id: idLat, keep: true}}})
		case t.kind == kindNumber:
			slots = append(slots, slot{i, []candidate{{id: idNum, keep: true}}})
		case punctID[t.text] != 0:
			slots = append(slots, slot{i, []candidate{{id: punctID[t.text], keep: true}}})
			if t.text == "?" {
				out[i] = ";"
			}
		}
	}
	choice := m.viterbi(slots)
	picked := make([]candidate, len(slots))
	for n, s := range slots {
		picked[n] = s.cands[choice[n]]
	}
	last := toks[len(toks)-1].text
	applyGrammar(picked, last == "?" || last == ";")
	first := true
	for n, s := range slots {
		c, t := picked[n], toks[s.tok]
		if !c.keep {
			out[s.tok] = applyCase(t.text, c.text, first || (c.id < uint32(len(m.flags)) && m.flags[c.id]&flagProper != 0))
		}
		if t.kind == kindWord || t.kind == kindVerbatim || t.kind == kindNumber {
			first = false
		}
	}
}

var (
	interrogatives = map[string]string{"που": "πού", "πως": "πώς"}
	// clauseLeads may precede a question word at the start of a clause.
	clauseLeads = map[string]bool{"εσύ": true, "εσείς": true, "και": true, "ρε": true, "μα": true, "αλλά": true, "ε": true, "λοιπόν": true, "τότε": true}
)

// applyGrammar fixes what a trigram model cannot see: the final ν rule and,
// in a question, the accent on πού and πώς opening a clause ("εσύ πώς
// είσαι;"), while a relative που inside the clause stays unaccented.
func applyGrammar(p []candidate, question bool) {
	clauseWord, prev := 0, ""
	for n := range p {
		c := &p[n]
		switch {
		case c.id == idComma:
			clauseWord = 0
			continue
		case c.keep:
			clauseWord++
			continue
		}
		if n+1 < len(p) && !p[n+1].keep {
			c.text = finalNu(c.text, p[n+1].text)
		}
		if accented, ok := interrogatives[c.text]; ok && question && (clauseWord == 0 || clauseWord == 1 && clauseLeads[prev]) {
			c.text = accented
		}
		prev = c.text
		clauseWord++
	}
}

// viterbi finds the cheapest path through the slots under the trigram model.
// The state is the pair of candidates at the previous and current slot.
func (m *Model) viterbi(slots []slot) []int {
	if len(slots) == 0 {
		return nil
	}
	bos := slot{cands: []candidate{{id: idBOS}}}
	eos := slot{cands: []candidate{{id: idEOS}}}
	all := append(append([]slot{bos}, slots...), eos)

	type cell struct {
		score float64
		back  int
	}
	// dp[i][j][k]: best score with candidate j at slot i-1 and k at slot i.
	dp := make([][][]cell, len(all))
	dp[1] = [][]cell{make([]cell, len(all[1].cands))}
	for k, c := range all[1].cands {
		dp[1][0][k] = cell{m.bigramLogp(idBOS, c.id) - c.cost, 0}
	}
	for i := 2; i < len(all); i++ {
		prev2, prev, cur := all[i-2].cands, all[i-1].cands, all[i].cands
		dp[i] = make([][]cell, len(prev))
		for j := range prev {
			dp[i][j] = make([]cell, len(cur))
			for k, c := range cur {
				best := cell{math.Inf(-1), 0}
				bi := m.bigramLogp(prev[j].id, c.id)
				for h := range prev2 {
					s := dp[i-1][h][j].score + m.trigramLogp(prev2[h].id, prev[j].id, c.id, bi)
					if s > best.score {
						best = cell{s, h}
					}
				}
				best.score -= c.cost
				dp[i][j][k] = best
			}
		}
	}

	last := len(all) - 1
	j, best := 0, math.Inf(-1)
	for jj := range dp[last] {
		if s := dp[last][jj][0].score; s > best {
			j, best = jj, s
		}
	}
	path := make([]int, len(all))
	path[last-1] = j
	k := 0
	for i := last; i >= 2; i-- {
		h := dp[i][path[i-1]][k].back
		k = path[i-1]
		path[i-2] = h
	}
	return path[1:last]
}

var nuWords = map[string]bool{"τη": true, "στη": true, "δε": true}

// finalNu applies the final ν rule: την, στην and δεν keep their ν before a
// vowel or κ, π, τ, ξ, ψ, μπ, ντ, γκ, τσ, τζ. μη is left out because it
// stays μη before nouns and adjectives ("η μη εφαρμογή").
func finalNu(word, next string) string {
	if !nuWords[word] {
		return word
	}
	n := []rune(stripGreek(next))
	if len(n) == 0 {
		return word
	}
	if strings.ContainsRune("αεηιουωκπτξψ", n[0]) {
		return word + "ν"
	}
	if len(n) > 1 && strings.Contains(" μπ ντ γκ τσ τζ", string(n[:2])) {
		return word + "ν"
	}
	return word
}

// applyCase carries the user's casing over to the Greek word: all caps stay
// all caps (without accents, as Greek typography wants), a capitalized word
// stays capitalized, and sentence starts and proper nouns get a capital.
func applyCase(orig, word string, capital bool) string {
	letters, upper := 0, 0
	for _, r := range orig {
		if unicode.IsLetter(r) {
			letters++
			if unicode.IsUpper(r) {
				upper++
			}
		}
	}
	switch {
	case letters >= 2 && upper == letters:
		return strings.ToUpper(stripAccents(word))
	case capital || (letters >= 2 && unicode.IsUpper(firstRune(orig))):
		return capitalize(word)
	}
	return word
}

func firstRune(s string) rune {
	r, _ := utf8.DecodeRuneInString(s)
	return r
}

func capitalize(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

func stripAccents(s string) string {
	var b strings.Builder
	for _, r := range s {
		if u, ok := unaccent[r]; ok && r != 'ς' {
			r = u
		}
		b.WriteRune(r)
	}
	return b.String()
}

// join reassembles tokens with normalized spacing: none before closing
// punctuation, one after a comma or question mark that was glued to a word.
func join(toks []token, out []string) string {
	var b strings.Builder
	for i, t := range toks {
		closing := t.kind == kindPunct && strings.ContainsAny(t.text, ",.;:!?)]}»…%")
		opening := i > 0 && toks[i-1].kind == kindPunct && strings.ContainsAny(toks[i-1].text, "([{«")
		glued := i > 0 && toks[i-1].kind == kindPunct && strings.ContainsAny(toks[i-1].text, ",;!?") && t.kind == kindWord
		if i > 0 && !closing && !opening && (t.space || glued) {
			b.WriteByte(' ')
		}
		b.WriteString(out[i])
	}
	return b.String()
}

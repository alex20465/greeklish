package greeklish

import (
	"cmp"
	"math"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Emission costs in nats, tuned against the stress tests.
const (
	editCost         = 7.0 // one typo: deletion, insertion, substitution or transposition
	maxPathCost      = 8.0
	keepEnglishScale = 0.6
	keepCharScale    = 0.8
	keepGreeklish    = 4.0
	accentCost       = 3.5 // Greek input whose explicit accents differ from the candidate
	oovCost          = 4.0 // fallback transliteration of a word missing from the vocabulary
	typedUnknownCost = 6.0 // Greek input word absent from the vocabulary, kept as typed
	maxCandidates    = 8
	minEditRunes     = 4
	commonLogp       = -12.0 // ln P of a word frequent enough to skip the typo search
	loanScale        = 1.0   // weight of an English word being more frequent than its Greek reading
	functionWordLogp = -8.0  // ln P above which a Greek reading counts as a function word
)

type candidate struct {
	id   uint32 // language model token
	text string // lowercase output, or the original text when kept
	cost float64
	keep bool
}

type search struct {
	m     *Model
	rules map[string][]rule
	best  map[int]float64 // form index → cheapest path
	seen  map[searchState]float64
	// minEdit is the input length from which deletions, insertions and
	// substitutions are tried; transpositions are tried from 3 letters.
	minEdit int
}

// span is the range of forms sharing the prefix built so far.
type span struct{ lo, hi int }

type searchState struct {
	pos, edits int
	out        string
}

// candidates lists plausible readings of one word token, cheapest first.
// accents tells whether the user types Greek with accents, in which case
// typed Greek words are trusted as written.
func (m *Model) candidates(word string, accents bool) []candidate {
	latin := latinWord(word)
	if !latin && !greekWord(word) {
		return []candidate{{id: idLat, text: word, keep: true}}
	}
	input, rules := strings.ToLower(word), latinRules
	if !latin {
		input, rules = stripGreek(word), greekRules
	}
	in := []rune(input)
	s := &search{m: m, rules: rules, best: map[int]float64{}}
	s.minEdit = minEditRunes
	if len(in) < minEditRunes {
		s.minEdit = len(in) + 1 // short words only get transpositions
	}
	s.run(in, 0, 0)
	if len(in) >= 3 && !s.common() {
		s.run(in, 0, 1)
	}

	cands := m.expand(s.best, word, latin, accents)
	if lp, ok := m.english[input]; ok && latin {
		// an English word is read as Greek only when Greek uses that
		// reading about as often: "to" can be το, "laptop" stays laptop
		for i := range cands {
			cands[i].cost += loanScale * max(0, float64(lp-m.uni[cands[i].id]))
		}
	}
	if latin {
		bestGreek, commonGreek := math.Inf(1), math.Inf(-1)
		for _, c := range cands {
			bestGreek = min(bestGreek, c.cost)
			if c.cost < 1 {
				commonGreek = max(commonGreek, float64(m.uni[c.id]))
			}
		}
		cands = append(cands, candidate{id: idLat, text: word, cost: m.keepCost(input, bestGreek, commonGreek), keep: true})
		if len(cands) == 1 && !strings.ContainsAny(input, "380") && m.greeklishness(input) > 0 {
			cands = append(cands, candidate{id: idUnk, text: transliterate(in), cost: oovCost})
		}
	} else if typed := lowerGreek(word); len(cands) == 0 || accents && !slices.ContainsFunc(cands, func(c candidate) bool { return c.text == typed }) {
		// from someone who types accents, a rare word missing from the
		// vocabulary is kept as typed rather than corrected into a common one
		cands = append(cands, candidate{id: idUnk, text: typed, cost: typedUnknownCost})
	}
	return cands
}

func (s *search) run(in []rune, cost float64, edits int) {
	s.seen = make(map[searchState]float64, 256)
	s.dfs(in, 0, "", span{0, len(s.m.forms)}, cost, edits)
}

// common reports whether an exact reading is frequent enough that a typo
// reading, at editCost, could hardly beat it. Skipping the typo search then
// makes most words several times faster.
func (s *search) common() bool {
	for f := range s.best {
		if s.m.uni[s.m.formStart[f]] >= commonLogp {
			return true
		}
	}
	return false
}

// narrow returns the part of sp whose forms start with next, which extends
// the prefix sp was built for. Searching only inside sp keeps it cheap.
func (s *search) narrow(sp span, next string) span {
	f := s.m.forms[sp.lo:sp.hi]
	lo := sort.Search(len(f), func(i int) bool { return f[i] >= next })
	hi := lo + sort.Search(len(f)-lo, func(i int) bool { return !strings.HasPrefix(f[lo+i], next) })
	return span{sp.lo + lo, sp.lo + hi}
}

func (s *search) dfs(in []rune, pos int, out string, sp span, cost float64, edits int) {
	if cost > maxPathCost {
		return
	}
	st := searchState{pos, edits, out}
	if c, ok := s.seen[st]; ok && c <= cost {
		return
	}
	s.seen[st] = cost
	if pos == len(in) && s.m.forms[sp.lo] == out {
		if c, seen := s.best[sp.lo]; !seen || cost < c {
			s.best[sp.lo] = cost
		}
	}
	for l := 1; l <= 3 && pos+l <= len(in); l++ {
		for _, r := range s.rules[string(in[pos:pos+l])] {
			next := out + r.out
			if nsp := s.narrow(sp, next); nsp.lo < nsp.hi {
				s.dfs(in, pos+l, next, nsp, cost+r.cost, edits)
			}
		}
	}
	if edits == 0 {
		return
	}
	if pos+1 < len(in) && in[pos] != in[pos+1] {
		for _, r1 := range s.rules[string(in[pos+1])] {
			next := out + r1.out
			nsp := s.narrow(sp, next)
			if nsp.lo == nsp.hi {
				continue
			}
			for _, r2 := range s.rules[string(in[pos])] {
				if nsp2 := s.narrow(nsp, next+r2.out); nsp2.lo < nsp2.hi {
					s.dfs(in, pos+2, next+r2.out, nsp2, cost+editCost+r1.cost+r2.cost, edits-1)
				}
			}
		}
	}
	if len(in) < s.minEdit {
		return
	}
	if pos < len(in) {
		s.dfs(in, pos+1, out, sp, cost+editCost, edits-1)
	}
	// insertion and substitution, only with letters some form continues with
	for i := sp.lo; i < sp.hi; {
		if len(s.m.forms[i]) == len(out) {
			i++
			continue
		}
		_, size := utf8.DecodeRuneInString(s.m.forms[i][len(out):])
		next := s.m.forms[i][:len(out)+size]
		nsp := s.narrow(span{i, sp.hi}, next)
		s.dfs(in, pos, next, nsp, cost+editCost, edits-1)
		if pos < len(in) {
			s.dfs(in, pos+1, next, nsp, cost+editCost, edits-1)
		}
		i = nsp.hi
	}
}

// expand turns matched forms into accented vocabulary words and keeps the
// most promising by path cost and frequency.
func (m *Model) expand(best map[int]float64, word string, latin, accents bool) []candidate {
	typed := ""
	if !latin && accents {
		typed = lowerGreek(word)
	}
	var cands []candidate
	for f, cost := range best {
		for id := m.formStart[f]; id < m.formStart[f+1]; id++ {
			c := candidate{id: id, text: m.words[id], cost: cost}
			if typed != "" && c.text != typed {
				c.cost += accentCost
			}
			cands = append(cands, c)
		}
	}
	rank := func(c candidate) float64 { return c.cost - float64(m.uni[c.id]) }
	slices.SortFunc(cands, func(a, b candidate) int { return cmp.Or(cmp.Compare(rank(a), rank(b)), cmp.Compare(a.id, b.id)) })
	return cands[:min(len(cands), maxCandidates)]
}

// keepCost prices leaving a Latin word untouched. The language model only
// scores the <lat> class, so this adds -ln P(word | Latin) the way a Greek
// word pays for its identity: English frequency for English words, a high
// price for unknown words that read like Greeklish, highest when the word
// has an exact Greek reading. commonGreek is ln P of the most frequent exact
// Greek reading: short function words that are as common in Greek as in
// English ("to", "me", "na") are read as Greek in Greeklish text.
func (m *Model) keepCost(w string, bestGreek, commonGreek float64) float64 {
	g := m.greeklishness(w)
	if lp, ok := m.english[w]; ok {
		cost := 1 + keepEnglishScale*float64(-lp) + keepGreeklish*math.Max(0, g)
		if commonGreek > float64(lp)-1 && commonGreek > functionWordLogp {
			cost += 2
		}
		return cost + max(0, commonGreek-float64(lp))
	}
	if math.IsInf(bestGreek, 1) {
		return 0
	}
	return keepCharScale*m.charCost(w) + keepGreeklish*math.Max(0, g)
}

// transliterate spells an unknown Greeklish word with the cheapest rules,
// longest pattern first, and fixes the final sigma.
func transliterate(in []rune) string {
	var b strings.Builder
	for pos := 0; pos < len(in); {
		l := min(3, len(in)-pos)
		for ; l > 1; l-- {
			if _, ok := latinRules[string(in[pos:pos+l])]; ok {
				break
			}
		}
		rs := latinRules[string(in[pos:pos+l])]
		if len(rs) == 0 {
			b.WriteRune(unicode.ToLower(in[pos]))
		} else {
			b.WriteString(rs[0].out)
		}
		pos += l
	}
	return lowerGreek(b.String())
}

package greeklish

import (
	"cmp"
	"iter"
	"math"
	"runtime"
	"slices"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// Pruning thresholds: they trade model size against coverage.
const (
	minWordCount    = 3
	minBigramCount  = 2
	minTrigramCount = 3
	maxWordRunes    = 32
	properShare     = 8 // in tenths: a word capitalized this often mid-sentence is a proper noun
)

// lmTokens maps a sentence to language model tokens: lowercase Greek words,
// <lat>, <num> and sentence punctuation. Other punctuation is dropped.
func lmTokens(sentence string) []string {
	var out []string
	for _, t := range tokenize(sentence) {
		switch {
		case t.kind == kindWord && greekWord(t.text):
			out = append(out, lowerGreek(t.text))
		case t.kind == kindWord || t.kind == kindVerbatim:
			out = append(out, "<lat>")
		case t.kind == kindNumber:
			out = append(out, "<num>")
		case punctID[t.text] != 0:
			out = append(out, specialWords[punctID[t.text]])
		}
	}
	return out
}

// capitalized reports a Greek word written Like-this, as proper nouns are.
func capitalized(s string) bool {
	r, size := utf8.DecodeRuneInString(s)
	return unicode.IsUpper(r) && strings.ToLower(s[size:]) == s[size:]
}

// Train builds a model. corpus is iterated twice, so it must yield the same
// sentences each time. english maps English words to their frequency.
func Train(corpus func() iter.Seq[string], english map[string]int) *Model {
	counts, caps := countWords(corpus())
	m := &Model{}
	ids := m.buildVocab(counts, caps)
	bi, tri, histories := countNgrams(corpus(), ids)
	m.buildNgrams(bi, tri, histories)
	m.buildEnglish(english)
	return m
}

func countWords(sentences iter.Seq[string]) (map[string]int, map[string]int) {
	counts, caps := map[string]int{}, map[string]int{}
	for s := range sentences {
		words := 0
		for _, t := range tokenize(s) {
			if t.kind != kindWord {
				continue
			}
			words++
			if !greekWord(t.text) {
				counts["<lat>"]++
				continue
			}
			w := lowerGreek(t.text)
			counts[w]++
			if words > 1 && capitalized(t.text) {
				caps[w]++
			}
		}
	}
	return counts, caps
}

type vocabEntry struct {
	word, form string
	count      int
}

func (m *Model) buildVocab(counts, caps map[string]int) map[string]uint32 {
	var vocab []vocabEntry
	for w, c := range counts {
		if c >= minWordCount && greekWord(w) && utf8.RuneCountInString(w) <= maxWordRunes {
			vocab = append(vocab, vocabEntry{w, stripGreek(w), c})
		}
	}
	slices.SortFunc(vocab, func(a, b vocabEntry) int {
		return cmp.Or(strings.Compare(a.form, b.form), cmp.Compare(b.count, a.count), strings.Compare(a.word, b.word))
	})
	ids := make(map[string]uint32, len(vocab)+int(numSpecial))
	m.words = append([]string{}, specialWords...)
	m.flags = make([]uint8, numSpecial, len(vocab)+int(numSpecial))
	for i, w := range specialWords {
		ids[w] = uint32(i)
	}
	canonical := map[string]string{}
	for i := 0; i < len(vocab); {
		j := i
		for j < len(vocab) && vocab[j].form == vocab[i].form {
			j++
		}
		foldMisspellings(vocab[i:j], func(from, to string) { canonical[from] = to })
		i = j
	}
	for _, e := range vocab {
		if _, folded := canonical[e.word]; folded {
			continue
		}
		id := uint32(len(m.words))
		ids[e.word] = id
		m.words = append(m.words, e.word)
		var flag uint8
		if caps[e.word]*10 > e.count*properShare {
			flag = flagProper
		}
		m.flags = append(m.flags, flag)
		if len(m.forms) == 0 || m.forms[len(m.forms)-1] != e.form {
			m.forms = append(m.forms, e.form)
			m.formStart = append(m.formStart, id)
		}
	}
	m.formStart = append(m.formStart, uint32(len(m.words)))
	for from, to := range canonical {
		ids[from] = ids[to]
	}
	return ids
}

// foldMisspellings maps unaccented words of two or more syllables, which
// are always misspelled, to the most frequent accented word of their form
// group, so their counts strengthen the correct spelling.
func foldMisspellings(group []vocabEntry, fold func(from, to string)) {
	i := slices.IndexFunc(group, func(e vocabEntry) bool { return hasAccent(e.word) })
	if i < 0 {
		return
	}
	for _, e := range group {
		if !hasAccent(e.word) && syllables(e.form) >= 2 {
			fold(e.word, group[i].word)
		}
	}
}

// syllables counts runs of vowels. It undercounts hiatus (ιδέα counts as
// two), which only makes folding more cautious: γεια and μια stay one.
func syllables(w string) int {
	n, inVowel := 0, false
	for _, r := range w {
		v := strings.ContainsRune("αεηιουω", r)
		if v && !inVowel {
			n++
		}
		inVowel = v
	}
	return n
}

// countNgrams tokenizes in parallel and counts in one goroutine.
func countNgrams(sentences iter.Seq[string], ids map[string]uint32) (map[uint64]uint32, map[uint64]uint32, []int) {
	batches := make(chan []string, 64)
	encoded := make(chan [][]uint32, 64)
	var wg sync.WaitGroup
	for range runtime.NumCPU() {
		wg.Go(func() {
			for batch := range batches {
				out := make([][]uint32, 0, len(batch))
				for _, s := range batch {
					out = append(out, encode(lmTokens(s), ids))
				}
				encoded <- out
			}
		})
	}
	go func() {
		batch := make([]string, 0, 4096)
		for s := range sentences {
			batch = append(batch, s)
			if len(batch) == cap(batch) {
				batches <- batch
				batch = make([]string, 0, 4096)
			}
		}
		batches <- batch
		close(batches)
		wg.Wait()
		close(encoded)
	}()

	histories := make([]int, len(ids))
	bi, tri := map[uint64]uint32{}, map[uint64]uint32{}
	for batch := range encoded {
		for _, seq := range batch {
			for i := 1; i < len(seq); i++ {
				histories[seq[i-1]]++
				bi[biKey(seq[i-1], seq[i])]++
				if i > 1 {
					tri[triKey(seq[i-2], seq[i-1], seq[i])]++
				}
			}
		}
	}
	return bi, tri, histories
}

func encode(tokens []string, ids map[string]uint32) []uint32 {
	seq := make([]uint32, 0, len(tokens)+2)
	seq = append(seq, idBOS)
	for _, t := range tokens {
		id, ok := ids[t]
		if !ok {
			id = idUnk
		}
		seq = append(seq, id)
	}
	return append(seq, idEOS)
}

func quantize(lp float64) uint8 {
	return uint8(min(255, math.Round(-lp/quantStep)))
}

func (m *Model) buildNgrams(bi, tri map[uint64]uint32, histories []int) {
	unigrams := make([]int, len(m.words))
	total := 0
	for k, c := range bi {
		unigrams[k&(1<<idBits-1)] += int(c)
		total += int(c)
	}
	m.uni = make([]float32, len(m.words))
	for id, c := range unigrams {
		m.uni[id] = float32(math.Log(float64(max(c, 1)) / float64(total)))
	}
	m.bigrams = prune(bi, minBigramCount, func(k uint64) int { return histories[k>>idBits] })
	m.trigrams = prune(tri, minTrigramCount, func(k uint64) int { return int(bi[k>>idBits]) })
}

func prune(counts map[uint64]uint32, minCount uint32, history func(uint64) int) ngrams {
	type entry struct {
		hash uint64
		val  uint8
	}
	var entries []entry
	for k, c := range counts {
		if c >= minCount {
			entries = append(entries, entry{hashKey(k), quantize(math.Log(float64(c) / float64(history(k))))})
		}
	}
	bucket := func(h uint64) uint64 { return h >> (64 - bucketBits) }
	slices.SortFunc(entries, func(a, b entry) int {
		return cmp.Or(cmp.Compare(bucket(a.hash), bucket(b.hash)), cmp.Compare(uint32(a.hash), uint32(b.hash)))
	})
	n := ngrams{start: make([]uint32, 1<<bucketBits+1), keys: make([]uint32, len(entries)), vals: make([]uint8, len(entries))}
	for i, e := range entries {
		n.keys[i], n.vals[i] = uint32(e.hash), e.val
		n.start[bucket(e.hash)+1]++
	}
	for b := 1; b < len(n.start); b++ {
		n.start[b] += n.start[b-1]
	}
	return n
}

func (m *Model) buildEnglish(english map[string]int) {
	total := 0
	for _, c := range english {
		total += c
	}
	m.english = make(map[string]float32, len(english))
	en := make([]float64, charAlphabet*charAlphabet*charAlphabet)
	for w, c := range english {
		m.english[w] = float32(math.Log(float64(c) / float64(total)))
		charTrigrams(w, func(i int) { en[i]++ })
	}
	gr := make([]float64, len(en))
	for id := numSpecial; int(id) < len(m.words); id++ {
		for _, style := range romanStyles {
			charTrigrams(romanize(m.words[id], style), func(i int) { gr[i]++ })
		}
	}
	m.charGR, m.charEN = charModel(gr), charModel(en)
}

// charModel turns trigram counts into add-half smoothed ln P(c | a b).
func charModel(counts []float64) []float32 {
	out := make([]float32, len(counts))
	for h := 0; h < len(counts); h += charAlphabet {
		sum := 0.0
		for _, c := range counts[h : h+charAlphabet] {
			sum += c
		}
		for i, c := range counts[h : h+charAlphabet] {
			out[h+i] = float32(math.Log((c + 0.5) / (sum + 0.5*charAlphabet)))
		}
	}
	return out
}

package greeklish

import (
	"encoding/binary"
	"errors"
	"math"
	"slices"
	"strings"
)

// Special token ids. Every Latin or verbatim token is <lat> and every number
// is <num>, so the model learns where Greek writers drop in foreign words.
const (
	idBOS uint32 = iota
	idEOS
	idUnk
	idLat
	idNum
	idComma
	idPeriod
	idQuestion
	idBang
	numSpecial
	noID = math.MaxUint32
)

var specialWords = []string{"<s>", "</s>", "<unk>", "<lat>", "<num>", ",", ".", ";", "!"}

// punctID maps punctuation that the language model sees to its token.
// The question mark is a Greek semicolon.
var punctID = map[string]uint32{",": idComma, ".": idPeriod, "...": idPeriod, "…": idPeriod, ";": idQuestion, "?": idQuestion, "!": idBang}

const (
	idBits       = 21     // vocabulary limit of 2M words, three ids fit a uint64
	backoff      = -0.916 // ln(0.4), stupid backoff
	quantStep    = 0.1    // nats per quantized step
	flagProper   = 1
	charAlphabet = 28 // a-z, word boundary, anything else
)

// Model is an immutable word trigram language model plus the indexes the
// converter needs. It is safe for concurrent use.
type Model struct {
	words     []string  // id → lowercase accented word
	uni       []float32 // id → ln P(word)
	flags     []uint8
	forms     []string // sorted unaccented forms
	formStart []uint32 // words of forms[i] are ids formStart[i]..formStart[i+1]
	bigrams   ngrams
	trigrams  ngrams
	english   map[string]float32 // ln P(word) in English
	charGR    []float32          // character trigram ln P for romanized Greek
	charEN    []float32          // and for English
}

func biKey(a, b uint32) uint64     { return uint64(a)<<idBits | uint64(b) }
func triKey(a, b, c uint32) uint64 { return uint64(a)<<(2*idBits) | uint64(b)<<idBits | uint64(c) }

// ngrams stores n-grams by a 50-bit hash instead of the 63-bit key: the top
// bucketBits select a bucket through start, the low 32 bits are stored. An
// absent n-gram matches a stored one with probability about n / 2^50, which
// is negligible, at 5 bytes per n-gram instead of 9.
type ngrams struct {
	start []uint32 // bucket b holds entries start[b]..start[b+1]
	keys  []uint32
	vals  []uint8 // quantized -ln P
}

const bucketBits = 18

func hashKey(k uint64) uint64 {
	k ^= k >> 33
	k *= 0xff51afd7ed558ccd
	k ^= k >> 33
	k *= 0xc4ceb9fe1a85ec53
	return k ^ k>>33
}

func (n *ngrams) lookup(k uint64) (float64, bool) {
	h := hashKey(k)
	b := h >> (64 - bucketBits)
	lo, hi := n.start[b], n.start[b+1]
	i, ok := slices.BinarySearch(n.keys[lo:hi], uint32(h))
	if !ok {
		return 0, false
	}
	return -float64(n.vals[int(lo)+i]) * quantStep, true
}

// bigramLogp returns ln P(c | b) with stupid backoff to the unigram.
func (m *Model) bigramLogp(b, c uint32) float64 {
	if v, ok := m.bigrams.lookup(biKey(b, c)); ok {
		return v
	}
	return backoff + float64(m.uni[c])
}

// trigramLogp returns ln P(c | a b) given bi = bigramLogp(b, c), which
// callers compute once and reuse for every a.
func (m *Model) trigramLogp(a, b, c uint32, bi float64) float64 {
	if v, ok := m.trigrams.lookup(triKey(a, b, c)); ok {
		return v
	}
	return backoff + bi
}

func charIndex(r byte) int {
	switch {
	case r >= 'a' && r <= 'z':
		return int(r - 'a')
	case r == '^':
		return 26
	}
	return 27
}

func charTrigrams(word string, fn func(i int)) {
	s := "^^" + word + "^"
	for i := 2; i < len(s); i++ {
		fn((charIndex(s[i-2])*charAlphabet+charIndex(s[i-1]))*charAlphabet + charIndex(s[i]))
	}
}

// charCost is -ln P(word) under the English character model.
func (m *Model) charCost(word string) float64 {
	sum := 0.0
	charTrigrams(word, func(i int) { sum -= float64(m.charEN[i]) })
	return sum
}

// greeklishness is the average per-character log ratio of the Greeklish
// character model over the English one: positive means it reads as Greeklish.
func (m *Model) greeklishness(word string) float64 {
	sum, n := 0.0, 0
	charTrigrams(word, func(i int) {
		sum += float64(m.charGR[i] - m.charEN[i])
		n++
	})
	return sum / float64(n)
}

// Binary layout: magic, then sections in a fixed order. Strings are stored
// as one newline-joined blob, numbers little endian.

const magic = "GRKL5"

type writer struct{ buf []byte }

func (w *writer) u32(v uint32) { w.buf = binary.LittleEndian.AppendUint32(w.buf, v) }

func (w *writer) strs(s []string) {
	blob := strings.Join(s, "\n")
	w.u32(uint32(len(blob)))
	w.buf = append(w.buf, blob...)
}

func (w *writer) f32s(v []float32) {
	w.u32(uint32(len(v)))
	for _, x := range v {
		w.u32(math.Float32bits(x))
	}
}

func (w *writer) u32s(v []uint32) {
	w.u32(uint32(len(v)))
	for _, x := range v {
		w.u32(x)
	}
}

func (w *writer) u8s(v []uint8) {
	w.u32(uint32(len(v)))
	w.buf = append(w.buf, v...)
}

// MarshalBinary serializes the model.
func (m *Model) MarshalBinary() ([]byte, error) {
	w := &writer{buf: []byte(magic)}
	w.strs(m.words)
	w.f32s(m.uni)
	w.u8s(m.flags)
	w.strs(m.forms)
	w.u32s(m.formStart)
	for _, n := range []*ngrams{&m.bigrams, &m.trigrams} {
		w.u32s(n.start)
		w.u32s(n.keys)
		w.u8s(n.vals)
	}
	en := make([]string, 0, len(m.english))
	for word := range m.english {
		en = append(en, word)
	}
	slices.Sort(en)
	enP := make([]float32, len(en))
	for i, word := range en {
		enP[i] = m.english[word]
	}
	w.strs(en)
	w.f32s(enP)
	w.f32s(m.charGR)
	w.f32s(m.charEN)
	return w.buf, nil
}

type reader struct {
	buf []byte
	err error
}

func (r *reader) u32() uint32 {
	if len(r.buf) < 4 {
		r.err = errors.New("greeklish: truncated model")
		return 0
	}
	v := binary.LittleEndian.Uint32(r.buf)
	r.buf = r.buf[4:]
	return v
}

func (r *reader) take(n int) []byte {
	if r.err != nil || len(r.buf) < n {
		r.err = errors.New("greeklish: truncated model")
		return nil
	}
	b := r.buf[:n]
	r.buf = r.buf[n:]
	return b
}

func (r *reader) strs() []string {
	b := r.take(int(r.u32()))
	if len(b) == 0 {
		return nil
	}
	return strings.Split(string(b), "\n")
}

func (r *reader) f32s() []float32 {
	b := r.take(4 * int(r.u32()))
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return v
}

func (r *reader) u32s() []uint32 {
	b := r.take(4 * int(r.u32()))
	v := make([]uint32, len(b)/4)
	for i := range v {
		v[i] = binary.LittleEndian.Uint32(b[4*i:])
	}
	return v
}

func (r *reader) u8s() []uint8 { return r.take(int(r.u32())) }

// UnmarshalBinary loads a model produced by MarshalBinary. The byte slices
// for quantized values alias data, which must not be modified afterwards.
func (m *Model) UnmarshalBinary(data []byte) error {
	if !strings.HasPrefix(string(data[:min(len(data), len(magic))]), magic) {
		return errors.New("greeklish: not a model file")
	}
	r := &reader{buf: data[len(magic):]}
	m.words = r.strs()
	m.uni = r.f32s()
	m.flags = r.u8s()
	m.forms = r.strs()
	m.formStart = r.u32s()
	for _, n := range []*ngrams{&m.bigrams, &m.trigrams} {
		n.start = r.u32s()
		n.keys = r.u32s()
		n.vals = r.u8s()
	}
	en := r.strs()
	enP := r.f32s()
	m.charGR = r.f32s()
	m.charEN = r.f32s()
	if r.err != nil {
		return r.err
	}
	m.english = make(map[string]float32, len(en))
	for i, word := range en {
		m.english[word] = enP[i]
	}
	return nil
}

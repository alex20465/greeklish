package greeklish

// rule maps an input spelling to an unaccented Greek spelling (final sigma
// folded to σ). Cost is in nats: 0 for spellings people use all the time,
// higher for rarer habits. The language model breaks ties between zero-cost
// alternatives, which is why "i" maps to five vowels at no cost.
type rule struct {
	out  string
	cost float64
}

var latinRules = map[string][]rule{
	"a":  {{"α", 0}},
	"b":  {{"μπ", 0}, {"β", 0.3}},
	"c":  {{"κ", 2.5}, {"σ", 2.5}},
	"ch": {{"χ", 0}},
	"d":  {{"δ", 0}, {"ντ", 0.2}},
	"dh": {{"δ", 0.5}},
	"e":  {{"ε", 0}, {"αι", 0}},
	"f":  {{"φ", 0}},
	"g":  {{"γ", 0}, {"γκ", 0.3}},
	"h":  {{"η", 0}, {"χ", 0}},
	"i":  {{"ι", 0}, {"η", 0}, {"υ", 0.2}, {"ει", 0}, {"οι", 0}, {"υι", 0.5}},
	"j":  {{"τζ", 0.5}, {"ζ", 1}, {"γ", 1}},
	"k":  {{"κ", 0}},
	"kh": {{"χ", 0.5}},
	"ks": {{"ξ", 0}},
	"l":  {{"λ", 0}},
	"m":  {{"μ", 0}},
	"n":  {{"ν", 0}},
	"o":  {{"ο", 0}, {"ω", 0}},
	"p":  {{"π", 0}},
	"ph": {{"φ", 0.5}},
	"ps": {{"ψ", 0}},
	"q":  {{"κ", 1}},
	"r":  {{"ρ", 0}},
	"s":  {{"σ", 0}},
	"t":  {{"τ", 0}},
	"th": {{"θ", 0}},
	"u":  {{"ου", 0}, {"υ", 0}},
	"v":  {{"β", 0}},
	"w":  {{"ω", 0}, {"ο", 0.7}, {"ου", 1}},
	"x":  {{"χ", 0}, {"ξ", 0}},
	"y":  {{"υ", 0}, {"γ", 0}, {"ι", 0.5}},
	"z":  {{"ζ", 0}},
	"8":  {{"θ", 0}},
	"3":  {{"ξ", 0}},
	"0":  {{"θ", 0.5}},
	"'":  {{"'", 0}},
	// Explicit digraphs: αυ/ευ are spelled with v or f, and the greedy
	// fallback transliteration needs the common vowel pairs as one unit.
	"av": {{"αυ", 0}}, "af": {{"αυ", 0}}, "ev": {{"ευ", 0}}, "ef": {{"ευ", 0}},
	"ou": {{"ου", 0}}, "ei": {{"ει", 0}}, "oi": {{"οι", 0}}, "ai": {{"αι", 0}},
	"mp": {{"μπ", 0}}, "nt": {{"ντ", 0}}, "gk": {{"γκ", 0}}, "gg": {{"γγ", 0}, {"γκ", 0.3}},
}

// Greek input is matched against itself, with the classic spelling
// confusions (ι/η/υ/ει/οι, ο/ω, ε/αι) allowed at a small cost.
var greekRules = map[string][]rule{
	"ι":  {{"ι", 0}, {"η", 0.6}, {"υ", 0.6}, {"ει", 0.6}, {"οι", 0.6}},
	"η":  {{"η", 0}, {"ι", 0.6}, {"υ", 0.6}, {"ει", 0.6}, {"οι", 0.6}},
	"υ":  {{"υ", 0}, {"ι", 0.6}, {"η", 0.6}, {"οι", 0.6}},
	"ει": {{"ει", 0}, {"ι", 0.6}, {"η", 0.6}},
	"οι": {{"οι", 0}, {"ι", 0.6}, {"η", 0.6}},
	"ο":  {{"ο", 0}, {"ω", 0.6}},
	"ω":  {{"ω", 0}, {"ο", 0.6}},
	"ε":  {{"ε", 0}, {"αι", 0.6}},
	"αι": {{"αι", 0}, {"ε", 0.6}},
}

const doubleCost = 0.4

func init() {
	// Consonant doubling is the most common Greeklish and Greek spelling slip:
	// "alos" for άλλος and "allos" with a single λ are both accepted.
	pairs := map[string]string{"l": "λ", "m": "μ", "n": "ν", "p": "π", "s": "σ", "t": "τ", "k": "κ", "r": "ρ", "g": "γ"}
	for lat, gr := range pairs {
		latinRules[lat] = append(latinRules[lat], rule{gr + gr, doubleCost})
		latinRules[lat+lat] = append(latinRules[lat+lat], rule{gr + gr, 0}, rule{gr, doubleCost})
		greekRules[gr] = append(greekRules[gr], rule{gr, 0}, rule{gr + gr, doubleCost})
		greekRules[gr+gr] = append(greekRules[gr+gr], rule{gr + gr, 0}, rule{gr, doubleCost})
	}
	for _, r := range "αβγδεζηθικλμνξοπρστυφχψω'" {
		s := string(r)
		if _, ok := greekRules[s]; !ok {
			greekRules[s] = []rule{{s, 0}}
		}
	}
}

type romanStyle struct{ pairs, single map[string]string }

// romanStyles are the two dominant ways of typing Greeklish: by sound
// (i, o, e, b, d) and by letter shape (h, w, ai, mp, nt).
var romanStyles = []romanStyle{
	{
		pairs:  map[string]string{"ου": "ou", "ει": "i", "οι": "i", "αι": "e", "μπ": "b", "ντ": "d", "γκ": "g", "αυ": "af", "ευ": "ef"},
		single: map[string]string{"η": "i", "υ": "i", "ω": "o", "θ": "th", "ξ": "ks", "χ": "x"},
	},
	{
		pairs:  map[string]string{"ου": "ou", "ει": "ei", "οι": "oi", "αι": "ai", "μπ": "mp", "ντ": "nt", "γκ": "gk", "αυ": "au", "ευ": "eu"},
		single: map[string]string{"η": "h", "υ": "y", "ω": "w", "θ": "th", "ξ": "ks", "χ": "ch"},
	},
}

// romanize writes a Greek word as Greeklish in the given style. It feeds
// the character model that tells Greeklish apart from English.
func romanize(word string, style romanStyle) string {
	var out []byte
	rs := []rune(stripGreek(word))
	for i := 0; i < len(rs); i++ {
		if i+1 < len(rs) {
			if s, ok := style.pairs[string(rs[i:i+2])]; ok {
				out = append(out, s...)
				i++
				continue
			}
		}
		if s, ok := style.single[string(rs[i])]; ok {
			out = append(out, s...)
		} else {
			out = append(out, romanSingle[rs[i]]...)
		}
	}
	return string(out)
}

var romanSingle = map[rune]string{
	'α': "a", 'β': "v", 'γ': "g", 'δ': "d", 'ε': "e", 'ζ': "z", 'η': "i", 'θ': "th", 'ι': "i", 'κ': "k",
	'λ': "l", 'μ': "m", 'ν': "n", 'ξ': "ks", 'ο': "o", 'π': "p", 'ρ': "r", 'σ': "s", 'τ': "t", 'υ': "i",
	'φ': "f", 'χ': "x", 'ψ': "ps", 'ω': "o", '\'': "'",
}

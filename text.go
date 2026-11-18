package greeklish

import (
	"strings"
	"unicode"
)

type kind uint8

const (
	kindWord     kind = iota // letters, possibly with Greeklish digits 3/8/0 and an elision apostrophe
	kindNumber               // 12, 1.4.2, 12:30, 3,5
	kindVerbatim             // URLs, emails, paths, identifiers, code: never converted
	kindPunct                // a single punctuation rune or a run like "..."
	kindOther                // emoji and symbols
)

type token struct {
	text  string
	kind  kind
	space bool // whitespace preceded the token in the input
}

var unaccent = map[rune]rune{
	'ά': 'α', 'έ': 'ε', 'ή': 'η', 'ί': 'ι', 'ό': 'ο', 'ύ': 'υ', 'ώ': 'ω',
	'ϊ': 'ι', 'ϋ': 'υ', 'ΐ': 'ι', 'ΰ': 'υ', 'ς': 'σ',
	'Ά': 'Α', 'Έ': 'Ε', 'Ή': 'Η', 'Ί': 'Ι', 'Ό': 'Ο', 'Ύ': 'Υ', 'Ώ': 'Ω', 'Ϊ': 'Ι', 'Ϋ': 'Υ',
}

// stripGreek lowercases, removes accents and diaeresis and folds final sigma,
// producing the key under which the vocabulary is indexed.
func stripGreek(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		r = unicode.ToLower(r)
		if u, ok := unaccent[r]; ok {
			r = u
		}
		b.WriteRune(r)
	}
	return b.String()
}

// lowerGreek lowercases while keeping accents and fixing the final sigma.
func lowerGreek(s string) string {
	rs := []rune(strings.ToLower(s))
	for i, r := range rs {
		if r == 'σ' && (i == len(rs)-1 || !unicode.IsLetter(rs[i+1])) {
			rs[i] = 'ς'
		}
		if r == 'ς' && i < len(rs)-1 && unicode.IsLetter(rs[i+1]) {
			rs[i] = 'σ'
		}
	}
	return string(rs)
}

func hasAccent(s string) bool {
	for _, r := range s {
		if u, ok := unaccent[r]; ok && u != 'σ' {
			return true
		}
	}
	return false
}

func isGreek(r rune) bool { return unicode.Is(unicode.Greek, r) }

func isLatin(r rune) bool { return r < 128 && unicode.IsLetter(r) }

// greekWord reports whether every letter of s is Greek.
func greekWord(s string) bool {
	seen := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			if !isGreek(r) {
				return false
			}
			seen = true
		}
	}
	return seen
}

func latinWord(s string) bool {
	seen := false
	for _, r := range s {
		switch {
		case isLatin(r):
			seen = true
		case r == '\'' || r == '3' || r == '8' || r == '0':
		default:
			return false
		}
	}
	return seen
}

var normalizer = strings.NewReplacer("’", "'", "‘", "'", "´", "'", "`", "'", ";", ";", "·", "·", " ", " ", "\t", " ")

// tokenize splits one line into tokens. Greeklish digits 3, 8 and 0 stay
// inside words ("8elw", "e3w"); any other digit mixed with letters marks the
// chunk as verbatim ("v1.4", "h264").
func tokenize(line string) []token {
	var toks []token
	for i, chunk := range strings.Fields(normalizer.Replace(line)) {
		space := i > 0
		core, tail := splitTrailingPunct(chunk)
		if isVerbatim(core) {
			toks = append(toks, token{text: core, kind: kindVerbatim, space: space})
			toks = append(toks, punctTokens(tail)...)
			continue
		}
		before := len(toks)
		toks = splitChunk(toks, chunk)
		if len(toks) > before {
			toks[before].space = space
		}
	}
	return toks
}

func splitTrailingPunct(chunk string) (string, string) {
	end := len(chunk)
	for end > 0 && strings.ContainsRune(".,;:!?)]}\"'»…", rune(chunk[end-1])) {
		end--
	}
	if strings.HasSuffix(chunk[:end], "»") {
		end -= len("»")
	}
	return chunk[:end], chunk[end:]
}

func punctTokens(s string) []token {
	return splitChunk(nil, s)
}

func isVerbatim(s string) bool {
	if s == "" {
		return false
	}
	if strings.Contains(s, "://") || strings.HasPrefix(s, "www.") || strings.ContainsAny(s, "/\\_=<>{}[]|~$") {
		return true
	}
	if (s[0] == '@' || s[0] == '#') && len(s) > 1 {
		return true
	}
	if at := strings.IndexByte(s, '@'); at > 0 && at < len(s)-1 {
		return true
	}
	letters, digits, prevLower, camel, dotted := 0, 0, false, false, false
	for i, r := range s {
		switch {
		case unicode.IsLetter(r):
			letters++
			if unicode.IsUpper(r) && prevLower {
				camel = true
			}
			prevLower = unicode.IsLower(r)
		case unicode.IsDigit(r):
			if r != '3' && r != '8' && r != '0' {
				digits++
			}
			prevLower = false
		case r == '.':
			if i > 0 && i < len(s)-1 && unicode.IsLetter(rune(s[i+1])) {
				dotted = true
			}
		}
	}
	return letters > 0 && (camel || dotted || digits > 0)
}

// splitChunk breaks a whitespace-free chunk into words, numbers and punctuation.
// An elided word ("s'", "t'") forces a space before the word that follows it.
func splitChunk(toks []token, chunk string) []token {
	rs := []rune(chunk)
	spaceNext := false
	for i := 0; i < len(rs); {
		r := rs[i]
		j := i + 1
		var t token
		switch {
		case unicode.IsLetter(r) || isWordDigit(rs, i):
			for j < len(rs) && (unicode.IsLetter(rs[j]) || unicode.Is(unicode.Mn, rs[j]) || isWordDigit(rs, j)) {
				j++
			}
			if j < len(rs) && rs[j] == '\'' && (j+1 == len(rs) || unicode.IsLetter(rs[j+1])) {
				j++
			}
			t = token{text: string(rs[i:j]), kind: kindWord}
		case unicode.IsDigit(r):
			for j < len(rs) && (unicode.IsDigit(rs[j]) || (strings.ContainsRune(".,:/-", rs[j]) && j+1 < len(rs) && unicode.IsDigit(rs[j+1]))) {
				j++
			}
			t = token{text: string(rs[i:j]), kind: kindNumber}
		case unicode.IsPunct(r):
			for r == '.' && j < len(rs) && rs[j] == '.' {
				j++
			}
			t = token{text: string(rs[i:j]), kind: kindPunct}
		default:
			t = token{text: string(r), kind: kindOther}
		}
		t.space = spaceNext
		spaceNext = t.kind == kindWord && strings.HasSuffix(t.text, "'")
		toks = append(toks, t)
		i = j
	}
	return toks
}

// isWordDigit reports whether rs[i] is a Greeklish letter digit touching a letter.
func isWordDigit(rs []rune, i int) bool {
	if rs[i] != '3' && rs[i] != '8' && rs[i] != '0' {
		return false
	}
	return (i > 0 && isLatin(rs[i-1])) || (i+1 < len(rs) && isLatin(rs[i+1]))
}

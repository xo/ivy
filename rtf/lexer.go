package rtf

// tokenKind is the kind of a token of RTF.
type tokenKind int

// The kinds of tokens.
const (
	tokenEOF tokenKind = iota
	tokenOpen
	tokenClose
	// tokenWord is a control word, such as \par or \fs24.
	tokenWord
	// tokenSymbol is a control symbol, such as \~ or \'e9.
	tokenSymbol
	// tokenText is a run of text.
	tokenText
)

// maxWordLength is the longest control word that RTF 1.9.1 allows.
const maxWordLength = 32

// token is a token of RTF.
type token struct {
	kind tokenKind
	// word is the name of a control word, or the character of a control
	// symbol.
	word string
	// param is the parameter of a control word, and hasParam tells whether
	// it has one.
	param    int
	hasParam bool
	// text is the text of a run, or the byte of the symbol \'hh.
	text []byte
}

// lexer splits RTF into tokens.
type lexer struct {
	data []byte
	pos  int
}

// isLetter reports whether c is an ASCII letter.
func isLetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// hexValue returns the value of a hexadecimal digit, or -1.
func hexValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

// next returns the next token.
func (l *lexer) next() token {
	for l.pos < len(l.data) {
		c := l.data[l.pos]
		switch c {
		case '{':
			l.pos++
			return token{kind: tokenOpen}
		case '}':
			l.pos++
			return token{kind: tokenClose}
		case '\\':
			return l.control()
		case '\r', '\n', 0:
			l.pos++
			continue
		}
		start := l.pos
		for l.pos < len(l.data) {
			c := l.data[l.pos]
			if c == '{' || c == '}' || c == '\\' || c == '\r' || c == '\n' || c == 0 {
				break
			}
			l.pos++
		}
		return token{kind: tokenText, text: l.data[start:l.pos]}
	}
	return token{kind: tokenEOF}
}

// control reads a control word or a control symbol.
func (l *lexer) control() token {
	l.pos++
	if l.pos >= len(l.data) {
		return token{kind: tokenEOF}
	}
	c := l.data[l.pos]
	if !isLetter(c) {
		l.pos++
		switch c {
		case '\'':
			// \'hh is a byte in the code page of the text.
			if l.pos+1 < len(l.data) {
				hi, lo := hexValue(l.data[l.pos]), hexValue(l.data[l.pos+1])
				if hi >= 0 && lo >= 0 {
					l.pos += 2
					return token{kind: tokenSymbol, word: "'", text: []byte{byte(hi<<4 | lo)}}
				}
			}
			return token{kind: tokenSymbol, word: "'"}
		case '\r', '\n':
			// A backslash before the end of a line is a \par.
			return token{kind: tokenWord, word: "par"}
		}
		return token{kind: tokenSymbol, word: string(c)}
	}
	start := l.pos
	for l.pos < len(l.data) && isLetter(l.data[l.pos]) && l.pos-start < maxWordLength {
		l.pos++
	}
	t := token{kind: tokenWord, word: string(l.data[start:l.pos])}
	// A word longer than the limit goes on: its rest is skipped.
	for l.pos < len(l.data) && isLetter(l.data[l.pos]) {
		l.pos++
	}
	neg := false
	if l.pos < len(l.data) && l.data[l.pos] == '-' && l.pos+1 < len(l.data) && l.data[l.pos+1] >= '0' && l.data[l.pos+1] <= '9' {
		neg = true
		l.pos++
	}
	digits := 0
	v := 0
	for l.pos < len(l.data) && l.data[l.pos] >= '0' && l.data[l.pos] <= '9' {
		if digits < 10 {
			v = v*10 + int(l.data[l.pos]-'0')
		}
		digits++
		l.pos++
	}
	if digits > 0 {
		t.hasParam = true
		if neg {
			v = -v
		}
		t.param = v
	}
	// A space ends a control word, and is part of it.
	if l.pos < len(l.data) && l.data[l.pos] == ' ' {
		l.pos++
	}
	return t
}

// binary returns the next n bytes of the data, for \binN.
func (l *lexer) binary(n int) []byte {
	if n < 0 {
		n = 0
	}
	end := min(l.pos+n, len(l.data))
	b := l.data[l.pos:end]
	l.pos = end
	return b
}

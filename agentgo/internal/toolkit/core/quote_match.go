package core

// normQuoteRune 将常见弯引号映射为直引号，用于「引号容错」下的逐字符比较。
func normQuoteRune(r rune) rune {
	switch r {
	case '\u201c', '\u201d': // " "
		return '"'
	case '\u2018', '\u2019': // ' '
		return '\''
	default:
		return r
	}
}

// Package highlight colors source code at build time, so a page only draws ready-made tokens.
//
// A file is tokenized whole, never line by line, so strings and comments that span lines are
// right. Each line becomes a compact string of token classes and lengths: "k3_1f4" is three
// characters of keyword, one plain, four of function name. Lengths count UTF-16 code units,
// as JavaScript strings do.
package highlight

import (
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// MaxBytes is the largest file that is highlighted; larger files show as plain text.
const MaxBytes = 512 << 10

// Lines returns one encoded line per line of text, or nil when the file's language isn't
// recognized from its name or the file is too large.
func Lines(path, text string) []string {
	if len(text) > MaxBytes || strings.IndexByte(text, 0) >= 0 {
		return nil
	}
	lexer := lexers.Match(path)
	if lexer == nil {
		return nil
	}
	it, err := chroma.Coalesce(lexer).Tokenise(nil, text)
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range chroma.SplitTokensIntoLines(it.Tokens()) {
		var b strings.Builder
		last, run := byte(0), 0
		flush := func() {
			if run > 0 {
				b.WriteByte(last)
				b.WriteString(strconv.Itoa(run))
			}
		}
		for _, t := range line {
			v := strings.TrimRight(t.Value, "\r\n")
			n := utf16Len(v)
			if n == 0 {
				continue
			}
			c := class(t.Type)
			if c != last {
				flush()
				last, run = c, 0
			}
			run += n
		}
		flush()
		out = append(out, b.String())
	}
	// A file ending in a newline has no tokens after it: drop the empty last line.
	if strings.HasSuffix(text, "\n") && len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

// class folds Chroma's token types into the few colors a diff needs.
func class(t chroma.TokenType) byte {
	switch {
	case t == chroma.KeywordType || t == chroma.NameClass || t == chroma.NameNamespace:
		return 't' // types
	case t == chroma.KeywordConstant || t == chroma.NameConstant:
		return 'v' // constants: true, nil, None
	case t.InCategory(chroma.Keyword):
		return 'k'
	case t == chroma.CommentPreproc || t == chroma.CommentPreprocFile:
		return 'p'
	case t.InCategory(chroma.Comment):
		return 'c'
	case t.InCategory(chroma.LiteralString):
		return 's'
	case t.InCategory(chroma.LiteralNumber):
		return 'n'
	case t == chroma.NameFunction || t == chroma.NameFunctionMagic:
		return 'f'
	case t == chroma.NameBuiltin || t == chroma.NameBuiltinPseudo:
		return 'b'
	case t == chroma.NameTag:
		return 'g'
	case t == chroma.NameAttribute:
		return 'a'
	case t == chroma.NameDecorator:
		return 'd'
	default:
		return '_'
	}
}

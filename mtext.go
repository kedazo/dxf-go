package dxf

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// MTextRun is a piece of MTEXT content with the same formatting.
type MTextRun struct {
	Text string
	// NewParagraph is set if a paragraph break (\P) precedes the run.
	NewParagraph bool

	Font   string // \f font family or \F font file; empty for the text style's font
	Bold   bool
	Italic bool
	// Height is an absolute text height set by \H, or 0 for the entity's height; HeightFactor is the product of the
	// relative heights (\H…x).
	Height       float64
	HeightFactor float64
	Color        Color // \C, ByLayer() when not set
	TrueColor    int   // \c as 0xRRGGBB, -1 when not set
	WidthFactor  float64
	ObliqueAngle float64 // degrees
	Tracking     float64
	Underline    bool
	Overline     bool
	Strike       bool
	// Stacked is set for stacked fractions (\S); Text is then "numerator/denominator", or only the part that is not
	// blank. Stacks with both parts blank (ArchiCAD uses them as spacers) produce no run.
	Stacked bool
	// Superscript and Subscript are set for \Sa^; and \S^b; stacks (e.g. the 2 of m²); Text is then that part.
	Superscript bool
	Subscript   bool
}

// FormattedText returns the complete MTEXT content including formatting codes: the extended text chunks (code 3)
// followed by the text (code 1).
func (m *MText) FormattedText() string {
	return strings.Join(m.ExtendedText, "") + m.Text
}

// Runs returns the MTEXT content split into formatted runs.
func (m *MText) Runs() []MTextRun {
	return ParseMTextRuns(m.FormattedText())
}

// PlainText returns the MTEXT content without formatting codes; paragraph breaks become newlines and superscript
// digits become their Unicode forms (m² for m\S2^;).
func (m *MText) PlainText() string {
	return mtextRunsToPlainText(m.Runs())
}

// PlainText returns the TEXT value with its control codes (%%u, %%o, %%k, %%c, %%d, %%p, %%nnn) and character
// escapes (\U+XXXX, \M+nXXXX) resolved.
func (t *Text) PlainText() string {
	return decodePercentCodes(decodeCharacterEscapes(t.Value), true)
}

// mbcsCodePages are the code pages of \M+nXXXX escapes, by n.
var mbcsCodePages = map[byte]string{
	'1': "ANSI_932", // Japanese (Shift-JIS)
	'2': "ANSI_950", // Traditional Chinese (Big5)
	'3': "ANSI_949", // Korean (Wansung)
	'5': "ANSI_936", // Simplified Chinese (GB 2312)
	// 4 is Korean Johab, which has no decoder here
}

// decodeCharacterEscape decodes the \U+XXXX (Unicode) or \M+nXXXX (a double-byte character of an Asian code page)
// escape at s[i:], returning the character and the escape's length.
func decodeCharacterEscape(s string, i int) (decoded string, length int, ok bool) {
	rest := s[i:]
	switch {
	case len(rest) >= 7 && strings.HasPrefix(rest, `\U+`) && isHexDigits(rest[3:7]):
		code, _ := strconv.ParseUint(rest[3:7], 16, 32)
		return string(rune(code)), 7, true
	case len(rest) >= 8 && strings.HasPrefix(rest, `\M+`) && isHexDigits(rest[4:8]):
		codePage, known := mbcsCodePages[rest[3]]
		if !known {
			return "", 0, false
		}
		code, _ := strconv.ParseUint(rest[4:8], 16, 32)
		decoded, err := encodingFromCodePage(codePage).NewDecoder().Bytes([]byte{byte(code >> 8), byte(code)})
		if err != nil || !utf8.Valid(decoded) || strings.ContainsRune(string(decoded), utf8.RuneError) {
			return "", 0, false
		}
		return string(decoded), 8, true
	}
	return "", 0, false
}

// decodeCharacterEscapes replaces every \U+XXXX and \M+nXXXX escape in s.
func decodeCharacterEscapes(s string) string {
	if !strings.Contains(s, `\U+`) && !strings.Contains(s, `\M+`) {
		return s
	}
	var builder strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\\' {
			if decoded, length, ok := decodeCharacterEscape(s, i); ok {
				builder.WriteString(decoded)
				i += length
				continue
			}
		}
		builder.WriteByte(s[i])
		i++
	}
	return builder.String()
}

func mtextRunsToPlainText(runs []MTextRun) string {
	var builder strings.Builder
	for _, run := range runs {
		if run.NewParagraph {
			builder.WriteByte('\n')
		}
		if run.Superscript {
			builder.WriteString(superscriptDigits(run.Text))
		} else {
			builder.WriteString(run.Text)
		}
	}
	return builder.String()
}

// superscriptDigits returns text in Unicode superscript digits if it consists of digits only, otherwise unchanged.
func superscriptDigits(text string) string {
	const superscripts = "⁰¹²³⁴⁵⁶⁷⁸⁹"
	var builder strings.Builder
	for _, r := range text {
		if r < '0' || r > '9' {
			return text
		}
		builder.WriteString(string([]rune(superscripts)[r-'0']))
	}
	return builder.String()
}

// ParseMTextRuns splits MTEXT content into runs of equally formatted text, resolving escapes, special characters and
// stacked fractions.
func ParseMTextRuns(text string) []MTextRun {
	parser := mtextParser{input: text}
	parser.state = MTextRun{HeightFactor: 1, Color: ByLayer(), TrueColor: -1, WidthFactor: 1, Tracking: 1}
	parser.parse()
	return parser.runs
}

type mtextParser struct {
	input            string
	position         int
	state            MTextRun
	stack            []MTextRun
	text             strings.Builder
	runs             []MTextRun
	pendingParagraph bool
}

func (p *mtextParser) parse() {
	for p.position < len(p.input) {
		c := p.input[p.position]
		switch {
		case c == '\\' && p.position+1 < len(p.input):
			p.position++
			p.parseCode(p.input[p.position])
		case c == '{':
			p.position++
			p.flush()
			p.stack = append(p.stack, p.state)
		case c == '}':
			p.position++
			p.flush()
			if len(p.stack) > 0 {
				p.state = p.stack[len(p.stack)-1]
				p.stack = p.stack[:len(p.stack)-1]
			}
		case c == '^' && p.position+1 < len(p.input):
			p.parseCaret(p.input[p.position+1])
			p.position += 2
		case c == '%' && strings.HasPrefix(p.input[p.position:], "%%"):
			p.parsePercent()
		default:
			p.text.WriteByte(c)
			p.position++
		}
	}
	p.flush()
	if p.pendingParagraph {
		run := p.state
		run.Text = ""
		run.NewParagraph = true
		p.runs = append(p.runs, run)
	}
}

// parseCode handles the code after a backslash; position is at the code letter.
func (p *mtextParser) parseCode(code byte) {
	p.position++
	switch code {
	case '\\', '{', '}':
		p.text.WriteByte(code)
	case 'P', 'X', 'N':
		// paragraph break, dimension line break and column break all start a new line
		p.flush()
		if p.pendingParagraph {
			// consecutive breaks keep an empty paragraph
			run := p.state
			run.NewParagraph = true
			p.runs = append(p.runs, run)
		}
		p.pendingParagraph = true
	case '~':
		p.text.WriteString(" ")
	case 'U', 'M':
		// \U+XXXX and \M+nXXXX character escapes; anything else stays as written
		if decoded, length, ok := decodeCharacterEscape(p.input, p.position-2); ok {
			p.text.WriteString(decoded)
			p.position += length - 2
		} else {
			p.text.WriteByte('\\')
			p.text.WriteByte(code)
		}
	case 'L', 'l', 'O', 'o', 'K', 'k':
		p.flush()
		on := code >= 'A' && code <= 'Z'
		switch code {
		case 'L', 'l':
			p.state.Underline = on
		case 'O', 'o':
			p.state.Overline = on
		default:
			p.state.Strike = on
		}
	case 'f', 'F':
		p.flush()
		p.parseFont(p.readArgument())
	case 'H':
		p.flush()
		if value, relative, ok := parseRelativeValue(p.readArgument()); ok {
			if relative {
				p.state.HeightFactor *= value
			} else {
				p.state.Height = value
				p.state.HeightFactor = 1
			}
		}
	case 'W':
		p.flush()
		if value, relative, ok := parseRelativeValue(p.readArgument()); ok {
			if relative {
				p.state.WidthFactor *= value
			} else {
				p.state.WidthFactor = value
			}
		}
	case 'T':
		p.flush()
		if value, relative, ok := parseRelativeValue(p.readArgument()); ok {
			if relative {
				p.state.Tracking *= value
			} else {
				p.state.Tracking = value
			}
		}
	case 'Q':
		p.flush()
		if value, err := strconv.ParseFloat(strings.TrimSpace(p.readArgument()), 64); err == nil {
			p.state.ObliqueAngle = value
		}
	case 'C':
		p.flush()
		if value, err := strconv.Atoi(strings.TrimSpace(p.readArgument())); err == nil {
			p.state.Color = Color(value)
		}
	case 'c':
		p.flush()
		if value, err := strconv.Atoi(strings.TrimSpace(p.readArgument())); err == nil {
			p.state.TrueColor = value
		}
	case 'S':
		p.parseStack(p.readArgument())
	case 'A', 'p':
		// alignment and paragraph properties don't change the text
		p.readArgument()
	default:
		// an unknown code; it has no argument we could skip reliably
	}
}

// readArgument returns the text up to the terminating semicolon (or the end) and moves past it.
func (p *mtextParser) readArgument() string {
	rest := p.input[p.position:]
	end := strings.IndexByte(rest, ';')
	if end < 0 {
		p.position = len(p.input)
		return rest
	}
	p.position += end + 1
	return rest[:end]
}

func (p *mtextParser) parseFont(argument string) {
	parts := strings.Split(argument, "|")
	p.state.Font = parts[0]
	p.state.Bold = false
	p.state.Italic = false
	for _, part := range parts[1:] {
		if len(part) < 2 {
			continue
		}
		switch part[0] {
		case 'b':
			p.state.Bold = part[1:] != "0"
		case 'i':
			p.state.Italic = part[1:] != "0"
		}
	}
}

func (p *mtextParser) parseStack(argument string) {
	p.flush()
	numerator, denominator, separator := argument, "", byte(0)
	if index := strings.IndexAny(argument, "^/#"); index >= 0 {
		numerator, denominator, separator = argument[:index], argument[index+1:], argument[index]
	}
	numerator, denominator = strings.TrimSpace(numerator), strings.TrimSpace(denominator)
	run := p.state
	run.Stacked = true
	switch {
	case numerator == "" && denominator == "":
		return
	case denominator == "":
		run.Text = numerator
		run.Superscript = separator == '^'
	case numerator == "":
		run.Text = denominator
		run.Subscript = separator == '^'
	default:
		run.Text = numerator + "/" + denominator
	}
	p.appendRun(run)
}

func (p *mtextParser) parseCaret(next byte) {
	switch next {
	case 'I':
		p.text.WriteByte('\t')
	case 'J', 'M':
		p.text.WriteByte('\n')
	case ' ':
		p.text.WriteByte('^')
	default:
		p.text.WriteByte('^')
		p.text.WriteByte(next)
	}
}

func (p *mtextParser) parsePercent() {
	rest := p.input[p.position:]
	if len(rest) >= 3 {
		if special, ok := percentSpecialCharacters[rest[2]|0x20]; ok {
			p.text.WriteString(special)
			p.position += 3
			return
		}
		if rest[2] == '%' {
			p.text.WriteByte('%')
			p.position += 3
			return
		}
	}
	p.text.WriteString("%%")
	p.position += 2
}

func (p *mtextParser) flush() {
	if p.text.Len() == 0 {
		return
	}
	run := p.state
	run.Text = p.text.String()
	p.text.Reset()
	p.appendRun(run)
}

func (p *mtextParser) appendRun(run MTextRun) {
	run.NewParagraph = p.pendingParagraph
	p.pendingParagraph = false
	p.runs = append(p.runs, run)
}

// parseRelativeValue parses "1.5" or the relative form "1.5x".
func parseRelativeValue(argument string) (value float64, relative bool, ok bool) {
	argument = strings.TrimSpace(argument)
	if strings.HasSuffix(argument, "x") || strings.HasSuffix(argument, "X") {
		relative = true
		argument = argument[:len(argument)-1]
	}
	value, err := strconv.ParseFloat(argument, 64)
	return value, relative, err == nil
}

var percentSpecialCharacters = map[byte]string{
	'c': "⌀", // diameter
	'd': "°", // degree
	'p': "±", // plus-minus
}

// decodePercentCodes resolves the %% control codes of TEXT values.
func decodePercentCodes(value string, dropToggles bool) string {
	if !strings.Contains(value, "%%") {
		return value
	}
	var builder strings.Builder
	for i := 0; i < len(value); {
		if !strings.HasPrefix(value[i:], "%%") || i+2 >= len(value) {
			builder.WriteByte(value[i])
			i++
			continue
		}
		code := value[i+2]
		switch {
		case percentSpecialCharacters[code|0x20] != "":
			builder.WriteString(percentSpecialCharacters[code|0x20])
			i += 3
		case code == '%':
			builder.WriteByte('%')
			i += 3
		case dropToggles && (code|0x20 == 'u' || code|0x20 == 'o' || code|0x20 == 'k'):
			// underline, overline and strike-through toggles
			i += 3
		case code >= '0' && code <= '9' && i+5 <= len(value) && isDecimalDigits(value[i+2:i+5]):
			number, _ := strconv.Atoi(value[i+2 : i+5])
			builder.WriteRune(rune(number))
			i += 5
		default:
			builder.WriteString("%%")
			i += 2
		}
	}
	return builder.String()
}

func isDecimalDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

package dxf

import (
	"math"
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
	// StackType is the separator of a stack: horizontal bar (a/b), diagonal (a#b) or tolerance without a line (a^b).
	// Numerator and Denominator are its two parts, trimmed; a stack without a separator has only a Numerator.
	StackType   MTextStackType
	Numerator   string
	Denominator string

	// VerticalAlignment (\A) places the run within its line: at the bottom (the default), centred or at the top. It
	// matters for stacks and runs smaller than the line; ArchiCAD writes \A1; (centred).
	VerticalAlignment MTextVerticalAlignment
	// Paragraph holds the paragraph properties (\p…;) in effect for the run. They are written at the start of a
	// paragraph (ArchiCAD repeats them after every \P), so all runs of a paragraph normally agree.
	Paragraph MTextParagraph
}

// MTextStackType is the separator of a stacked fraction (\S).
type MTextStackType int

const (
	MTextStackNone       MTextStackType = iota // not a stack, or a stack without a separator
	MTextStackHorizontal                       // a/b: one part above the other, divided by a horizontal bar
	MTextStackDiagonal                         // a#b: divided by a diagonal slash
	MTextStackTolerance                        // a^b: one part above the other without a line (also a^ and ^b)
)

// MTextVerticalAlignment is the alignment of a run within its line (\A0; \A1; \A2;).
type MTextVerticalAlignment int

const (
	MTextVerticalAlignmentBottom MTextVerticalAlignment = iota
	MTextVerticalAlignmentCenter
	MTextVerticalAlignmentTop
)

// MTextParagraphAlignment is the horizontal alignment of a paragraph (\pq?;).
type MTextParagraphAlignment int

const (
	// MTextParagraphAlignmentDefault means no \pq code: the paragraph follows the MTEXT's attachment point.
	MTextParagraphAlignmentDefault     MTextParagraphAlignment = iota
	MTextParagraphAlignmentLeft                                // \pql;
	MTextParagraphAlignmentRight                               // \pqr;
	MTextParagraphAlignmentCenter                              // \pqc;
	MTextParagraphAlignmentJustified                           // \pqj;
	MTextParagraphAlignmentDistributed                         // \pqd;
)

// MTextParagraph holds the paragraph properties of the \p code, e.g. \pxi-3,l3,qc,t4,c8,r12;. Indents and tab
// stops are kept as written; ezdxf reads them as multiples of the MTEXT's text height (InitialTextHeight).
type MTextParagraph struct {
	Alignment MTextParagraphAlignment
	// FirstLineIndent (i) is relative to LeftIndent (l); RightIndent (r) is measured from the right edge.
	FirstLineIndent float64
	LeftIndent      float64
	RightIndent     float64
	// TabStops (t) are absolute positions; nil means the default stops. Runs share the slice: don't modify it.
	TabStops []MTextTabStop
}

// MTextTabStop is a tab stop of a paragraph.
type MTextTabStop struct {
	Position float64
	Type     MTextTabStopType
}

// MTextTabStopType is how text is aligned at a tab stop: no prefix is left, c centre and r right.
type MTextTabStopType int

const (
	MTextTabStopLeft MTextTabStopType = iota
	MTextTabStopCenter
	MTextTabStopRight
)

// FormattedText returns the complete MTEXT content including formatting codes: the extended text chunks (code 3)
// followed by the text (code 1).
func (m *MText) FormattedText() string {
	return strings.Join(m.ExtendedText, "") + m.Text
}

// Direction returns the direction of the text's baseline in world coordinates. It is XAxisDirection when the file had
// one (HasXAxisDirection) or it was changed from the default; otherwise it is RotationAngle (degrees) in the MTEXT's
// object coordinate system. The writer writes it as the direction vector.
func (m *MText) Direction() Vector {
	if m.HasXAxisDirection || m.XAxisDirection != *NewXAxis() {
		if direction := m.XAxisDirection.Normalize(); !direction.IsZero(0) {
			return direction
		}
	}
	normal := m.ExtrusionDirection
	if normal.IsZero(0) {
		normal = *NewZAxis()
	}
	sin, cos := math.Sincos(m.RotationAngle * math.Pi / 180)
	return OCSToWCSMatrix(normal).TransformVector(Vector{cos, sin, 0})
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

// PlainText returns the attribute's text without control codes: the embedded MText's plain text for a multiline
// attribute, otherwise the value resolved like a TEXT value (see Text.PlainText).
func (a *Attribute) PlainText() string {
	return attributePlainText(a.Value, a.IsMultiline(), &a.MText)
}

// PlainText returns the attribute definition's default text without control codes, like Attribute.PlainText.
func (ad *AttributeDefinition) PlainText() string {
	return attributePlainText(ad.Value, ad.IsMultiline(), &ad.MText)
}

func attributePlainText(value string, isMultiline bool, mtext *MText) string {
	if isMultiline && mtext.FormattedText() != "" {
		return mtext.PlainText()
	}
	return decodePercentCodes(decodeCharacterEscapes(value), true)
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
		case c == '%' && strings.HasPrefix(p.input[p.position:], "%%"):
			p.parsePercent()
		case c == '\r':
			// ^M (alone or as ^M^J) is a line break like ^J
			p.text.WriteByte('\n')
			p.position++
			if p.position < len(p.input) && p.input[p.position] == '\n' {
				p.position++
			}
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
		p.parseStack()
	case 'A':
		// one digit and an optional semicolon: ArchiCAD writes \A1{\H0.7x;\S2^ ;} for superscripts
		p.flush()
		p.state.VerticalAlignment = MTextVerticalAlignmentBottom
		if p.position < len(p.input) {
			if digit := p.input[p.position]; digit >= '0' && digit <= '2' {
				p.state.VerticalAlignment = MTextVerticalAlignment(digit - '0')
			}
			p.position++
		}
		if p.position < len(p.input) && p.input[p.position] == ';' {
			p.position++
		}
	case 'p':
		p.flush()
		p.state.Paragraph = parseMTextParagraph(p.state.Paragraph, p.readArgument())
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

// parseMTextParagraph applies the properties of a \p code (without the \p and the semicolon) to paragraph:
// i, l, r (indents), q (alignment: l r c j d), t (tab stops, to the end) and x (ignored); * resets a property.
func parseMTextParagraph(paragraph MTextParagraph, argument string) MTextParagraph {
	for i := 0; i < len(argument); {
		command := argument[i]
		i++
		switch command {
		case 'i', 'l', 'r':
			value, length := parseFloatPrefix(argument[i:])
			if length == 0 && strings.HasPrefix(argument[i:], "*") {
				length = 1
			}
			i += length
			switch command {
			case 'i':
				paragraph.FirstLineIndent = value
			case 'l':
				paragraph.LeftIndent = value
			default:
				paragraph.RightIndent = value
			}
		case 'q':
			paragraph.Alignment = MTextParagraphAlignmentDefault
			if i < len(argument) {
				if index := strings.IndexByte("lrcjd", argument[i]); index >= 0 {
					paragraph.Alignment = MTextParagraphAlignment(index + 1)
				}
				i++
			}
		case 't':
			// the tab stops run to the end; an empty list resets them
			var tabStops []MTextTabStop
			for i < len(argument) {
				stop := MTextTabStop{}
				switch argument[i] {
				case 'c':
					stop.Type = MTextTabStopCenter
					i++
				case 'r':
					stop.Type = MTextTabStopRight
					i++
				}
				value, length := parseFloatPrefix(argument[i:])
				if length == 0 {
					// a comma or something invalid
					i++
					continue
				}
				stop.Position = value
				tabStops = append(tabStops, stop)
				i += length
			}
			paragraph.TabStops = tabStops
		}
		// x, commas and unknown letters are skipped
	}
	return paragraph
}

// parseFloatPrefix parses the number at the start of s ([+-]digits[.digits]) and returns its length, 0 if none.
func parseFloatPrefix(s string) (float64, int) {
	end := 0
	if end < len(s) && (s[end] == '+' || s[end] == '-') {
		end++
	}
	digits := 0
	for ; end < len(s) && (s[end] >= '0' && s[end] <= '9' || s[end] == '.'); end++ {
		digits++
	}
	if digits == 0 {
		return 0, 0
	}
	value, err := strconv.ParseFloat(s[:end], 64)
	if err != nil {
		return 0, 0
	}
	return value, end
}

// parseStack parses the \S argument up to the semicolon; position is after the S. A backslash escapes the next
// character, so \S1\/2; is not a fraction and \; doesn't end the stack.
func (p *mtextParser) parseStack() {
	p.flush()
	var parts [2]strings.Builder
	part, separator := 0, byte(0)
	for p.position < len(p.input) {
		c := p.input[p.position]
		p.position++
		switch {
		case c == ';':
			p.appendStack(parts[0].String(), parts[1].String(), separator)
			return
		case c == '\\' && p.position < len(p.input):
			parts[part].WriteByte(p.input[p.position])
			p.position++
		case part == 0 && (c == '^' || c == '/' || c == '#'):
			part, separator = 1, c
		default:
			parts[part].WriteByte(c)
		}
	}
	p.appendStack(parts[0].String(), parts[1].String(), separator)
}

func (p *mtextParser) appendStack(numerator, denominator string, separator byte) {
	numerator, denominator = strings.TrimSpace(numerator), strings.TrimSpace(denominator)
	run := p.state
	run.Stacked = true
	run.Numerator, run.Denominator = numerator, denominator
	switch separator {
	case '/':
		run.StackType = MTextStackHorizontal
	case '#':
		run.StackType = MTextStackDiagonal
	case '^':
		run.StackType = MTextStackTolerance
	}
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
	'c': "Ø", // diameter: CAD fonts draw Ø (U+00D8), and unlike ⌀ (U+2300) every Latin font has it
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

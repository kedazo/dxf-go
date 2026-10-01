package dxf

import (
	"testing"
)

func TestMTextPlainText(t *testing.T) {
	for _, testCase := range []struct{ formatted, plain string }{
		{"plain", "plain"},
		{"Előtér", "Előtér"},
		{"first\\Psecond", "first\nsecond"},
		{"a\\P\\Pb", "a\n\nb"},
		{"trailing\\P", "trailing\n"},
		// ArchiCAD style font switches
		{"{\\fArial Narrow|b0|i0|c238|p34;Előtér}", "Előtér"},
		{"\\A1;{\\pqc;\\fArial|b1|i0|c204|p34;konyha + étkező}", "konyha + étkező"},
		{"{\\H0.7x;\\C1;red} and {\\c16711680;blue}", "red and blue"},
		{"\\Fromans.shx;\\W0.8;\\Q15;\\T1.1;text", "text"},
		{"\\Lunder\\l \\Oover\\o \\Kstrike\\k", "under over strike"},
		// escapes and special characters
		{"a\\\\b \\{c\\}", "a\\b {c}"},
		{"no\\~break", "no break"},
		{"%%c50 45%%d %%p0.5 100%%%", "Ø50 45° ±0.5 100%"},
		// caret escapes are decoded by the reader, so ^ and control characters arrive as themselves
		{"tab\tand\nnewline x^2", "tab\tand\nnewline x^2"},
		{"a\rb\r\nc", "a\nb\nc"},
		// stacked fractions
		{"1\\S1/2;\"", "11/2\""},
		{"\\S+0.01^-0.02;", "+0.01/-0.02"},
		{"\\S3#4;", "3/4"},
		// ArchiCAD: superscripts and empty stacks used as spacers
		{"{\\fArial Narrow|b0|i0|c238|p0;288,47 m{\\H0.66x;\\S2^  ;}}", "288,47 m²"},
		{"\\A1;\\pt0;pny 2,20{\\H0.66x;\\S^  ;} /0,28{\\H0.66x;\\S^  ;\\S^  ;}", "pny 2,20 /0,28"},
		{"x\\S10^;", "x¹⁰"},
		{"x\\Sab^;", "xab"},
		{"H\\S^2;O", "H2O"},
		{"\\S1/;", "1"},
		// character escapes; decoded backslashes and braces are text, not codes
		{"\\U+0150r\\U+00E9s", "Őrés"},
		{"a\\P\\U+0151", "a\nő"},
		{"\\U+005C\\U+007Bx\\U+007D", "\\{x}"},
		{"\\U+12G4", "\\U+12G4"},
		{"\\M+182A0 \\M+2A440 \\M+3B0A1 \\M+5B0A1", "あ 一 가 啊"},
		{"\\M+4B0A1", "\\M+4B0A1"},
		// unterminated codes don't break anything
		{"\\H2.5", ""},
		{"{unclosed", "unclosed"},
		{"closed}", "closed"},
	} {
		mtext := NewMText()
		mtext.Text = testCase.formatted
		assertEqString(t, testCase.plain, mtext.PlainText())
	}
}

func TestMTextPlainTextJoinsExtendedText(t *testing.T) {
	mtext := NewMText()
	mtext.ExtendedText = []string{"{\\fArial;fir", "st"}
	mtext.Text = " part}"
	assertEqString(t, "{\\fArial;first part}", mtext.FormattedText())
	assertEqString(t, "first part", mtext.PlainText())
}

func TestMTextRuns(t *testing.T) {
	runs := ParseMTextRuns("plain {\\fArial|b1|i1;bold \\H2.5;big}\\P{\\H0.5x;\\C3;\\c255;small\\S1^2;}")
	assertEqInt(t, 5, len(runs))

	assertEqString(t, "plain ", runs[0].Text)
	assertEqString(t, "", runs[0].Font)
	assertEqBool(t, false, runs[0].Bold)
	assertEqFloat64(t, 1.0, runs[0].HeightFactor)
	assertEqInt(t, int(ByLayer()), int(runs[0].Color))
	assertEqInt(t, -1, runs[0].TrueColor)

	assertEqString(t, "bold ", runs[1].Text)
	assertEqString(t, "Arial", runs[1].Font)
	assertEqBool(t, true, runs[1].Bold)
	assertEqBool(t, true, runs[1].Italic)
	assertEqFloat64(t, 0.0, runs[1].Height)

	assertEqString(t, "big", runs[2].Text)
	assertEqFloat64(t, 2.5, runs[2].Height)
	assertEqBool(t, true, runs[2].Bold)

	assertEqString(t, "small", runs[3].Text)
	assertEqBool(t, true, runs[3].NewParagraph)
	assertEqString(t, "", runs[3].Font)
	assertEqFloat64(t, 0.5, runs[3].HeightFactor)
	assertEqInt(t, 3, int(runs[3].Color))
	assertEqInt(t, 255, runs[3].TrueColor)

	assertEqString(t, "1/2", runs[4].Text)
	assertEqBool(t, true, runs[4].Stacked)
	assertEqBool(t, false, runs[4].NewParagraph)
	assertEqFloat64(t, 0.5, runs[4].HeightFactor)
}

func TestMTextDirection(t *testing.T) {
	// a direction vector wins over the rotation
	withVector := parseEntity(t, "MTEXT",
		NewDoubleCodePair(11, 0), NewDoubleCodePair(21, 1), NewDoubleCodePair(31, 0),
		NewDoubleCodePair(50, 45),
	).(*MText)
	assertEqBool(t, true, withVector.HasXAxisDirection)
	assertNearVector(t, Vector{0, 1, 0}, withVector.Direction())

	// without one, group 50 is the rotation in degrees
	rotated := parseEntity(t, "MTEXT", NewDoubleCodePair(50, 90)).(*MText)
	assertEqBool(t, false, rotated.HasXAxisDirection)
	assertEqFloat64(t, 90, rotated.RotationAngle)
	assertNearVector(t, Vector{0, 1, 0}, rotated.Direction())

	// the rotation is in the object coordinate system: seen from below, OCS X is WCS -X
	mirrored := parseEntity(t, "MTEXT", NewDoubleCodePair(210, 0), NewDoubleCodePair(220, 0), NewDoubleCodePair(230, -1)).(*MText)
	assertNearVector(t, Vector{-1, 0, 0}, mirrored.Direction())

	assertNearVector(t, Vector{1, 0, 0}, NewMText().Direction())
}

func TestWriteMTextDirectionAsVector(t *testing.T) {
	// a 50 after 11 would override the direction vector, so the direction is only written as the vector
	rotated := NewMText()
	rotated.RotationAngle = 30
	actual := drawingCodePairsFromEntity(t, rotated, R2018)
	assertContainsCodePairs(t, []CodePair{
		NewDoubleCodePair(11, rotated.Direction().X),
		NewDoubleCodePair(21, rotated.Direction().Y),
		NewDoubleCodePair(31, 0),
	}, actual)
	assertNotContainsCodePairs(t, []CodePair{NewDoubleCodePair(50, 30)}, actual)

	reloaded := parseEntity(t, "MTEXT", NewDoubleCodePair(11, rotated.Direction().X), NewDoubleCodePair(21, rotated.Direction().Y)).(*MText)
	assertNearVector(t, rotated.Direction(), reloaded.Direction())
}

func TestMTextRunsSuperscriptAndSubscript(t *testing.T) {
	runs := ParseMTextRuns("m\\S2^ ;\\S^ ;H\\S^2;")
	assertEqInt(t, 4, len(runs))
	assertEqString(t, "2", runs[1].Text)
	assertEqBool(t, true, runs[1].Stacked)
	assertEqBool(t, true, runs[1].Superscript)
	assertEqBool(t, false, runs[1].Subscript)
	assertEqString(t, "2", runs[3].Text)
	assertEqBool(t, false, runs[3].Superscript)
	assertEqBool(t, true, runs[3].Subscript)
}

func TestMTextRunsStackType(t *testing.T) {
	runs := ParseMTextRuns("\\S1/2;\\S3#4;\\S+0.1^-0.2;\\S5;\\S a\\/b ^ c\\;d;")
	assertEqInt(t, 5, len(runs))
	for i, expected := range []struct {
		stackType              MTextStackType
		numerator, denominator string
	}{
		{MTextStackHorizontal, "1", "2"},
		{MTextStackDiagonal, "3", "4"},
		{MTextStackTolerance, "+0.1", "-0.2"},
		{MTextStackNone, "5", ""},
		// escaped separators and semicolons are text
		{MTextStackTolerance, "a/b", "c;d"},
	} {
		assertEqBool(t, true, runs[i].Stacked)
		assertEqInt(t, int(expected.stackType), int(runs[i].StackType))
		assertEqString(t, expected.numerator, runs[i].Numerator)
		assertEqString(t, expected.denominator, runs[i].Denominator)
	}
	assertEqString(t, "3/4", runs[1].Text)
	assertEqBool(t, false, runs[3].Superscript)
}

func TestMTextRunsVerticalAlignment(t *testing.T) {
	// ArchiCAD writes \A1 without a semicolon before a brace; it must not swallow the scope or the height
	runs := ParseMTextRuns("\\A1;{\\fArial;m\\A2{\\H0.7x;\\S2^ ;} after}\\A0;x\\A;y")
	assertEqInt(t, 5, len(runs))
	assertEqInt(t, int(MTextVerticalAlignmentCenter), int(runs[0].VerticalAlignment))
	assertEqString(t, "2", runs[1].Text)
	assertEqFloat64(t, 0.7, runs[1].HeightFactor)
	assertEqInt(t, int(MTextVerticalAlignmentTop), int(runs[1].VerticalAlignment))
	assertEqString(t, " after", runs[2].Text)
	assertEqString(t, "Arial", runs[2].Font)
	assertEqFloat64(t, 1, runs[2].HeightFactor)
	// \A2 stands before the inner scope, so it applies until the outer one ends
	assertEqInt(t, int(MTextVerticalAlignmentTop), int(runs[2].VerticalAlignment))
	assertEqInt(t, int(MTextVerticalAlignmentBottom), int(runs[3].VerticalAlignment))
	// like ezdxf, \A always takes the next character; anything but 0, 1 and 2 means bottom
	assertEqString(t, "y", runs[4].Text)
	assertEqInt(t, int(MTextVerticalAlignmentBottom), int(runs[4].VerticalAlignment))

	mtext := NewMText()
	mtext.Text = "\\A1;{\\fArial;m\\A1{\\H0.7x;\\S2^ ;} after}\\P\\A1x"
	assertEqString(t, "m² after\nx", mtext.PlainText())
}

func TestMTextRunsParagraphProperties(t *testing.T) {
	runs := ParseMTextRuns("{\\pqc;title\\P\\pxqr;right}\\Pdefault\\P\\pxi-3,l3,r1.5,t4,c8,r12;item\\P\\pi*,l*,r*,q*,t;reset\\P\\pt2.5,44.9918;\\pqd;tabs")
	assertEqInt(t, 6, len(runs))

	assertEqInt(t, int(MTextParagraphAlignmentCenter), int(runs[0].Paragraph.Alignment))
	assertEqInt(t, int(MTextParagraphAlignmentRight), int(runs[1].Paragraph.Alignment))
	// paragraph properties are scoped by braces
	assertEqString(t, "default", runs[2].Text)
	assertEqInt(t, int(MTextParagraphAlignmentDefault), int(runs[2].Paragraph.Alignment))

	item := runs[3].Paragraph
	assertEqFloat64(t, -3, item.FirstLineIndent)
	assertEqFloat64(t, 3, item.LeftIndent)
	assertEqFloat64(t, 1.5, item.RightIndent)
	assertEqInt(t, 3, len(item.TabStops))
	assertEqFloat64(t, 4, item.TabStops[0].Position)
	assertEqInt(t, int(MTextTabStopLeft), int(item.TabStops[0].Type))
	assertEqFloat64(t, 8, item.TabStops[1].Position)
	assertEqInt(t, int(MTextTabStopCenter), int(item.TabStops[1].Type))
	assertEqFloat64(t, 12, item.TabStops[2].Position)
	assertEqInt(t, int(MTextTabStopRight), int(item.TabStops[2].Type))

	reset := runs[4].Paragraph
	assertEqInt(t, int(MTextParagraphAlignmentDefault), int(reset.Alignment))
	assertEqFloat64(t, 0, reset.FirstLineIndent+reset.LeftIndent+reset.RightIndent)
	assertEqInt(t, 0, len(reset.TabStops))

	// properties persist across paragraphs and later \p codes only change what they name
	tabs := runs[5].Paragraph
	assertEqInt(t, int(MTextParagraphAlignmentDistributed), int(tabs.Alignment))
	assertEqInt(t, 2, len(tabs.TabStops))
	assertEqFloat64(t, 44.9918, tabs.TabStops[1].Position)
}

func TestMTextRunsRelativeValuesMultiply(t *testing.T) {
	runs := ParseMTextRuns("{\\H2x;{\\H0.5x;\\W0.5;\\W2x;x}}")
	assertEqInt(t, 1, len(runs))
	assertEqFloat64(t, 1.0, runs[0].HeightFactor)
	assertEqFloat64(t, 1.0, runs[0].WidthFactor)
}

func TestMTextRunsWidthFactorAndObliqueAnglePresence(t *testing.T) {
	// a style with width factor 0.8 and a 15° slant
	runs := ParseMTextRuns("a{\\W1;\\Q0;b}{\\W0.5x;c}{\\W2;\\W0.5x;\\Q-10;d}e")
	assertEqInt(t, 5, len(runs))
	expected := []struct {
		text           string
		hasWidth       bool
		width, oblique float64
	}{
		{"a", false, 0.8, 15}, // no codes: the style's values
		{"b", true, 1, 0},     // explicit \W1; \Q0; cancel the style
		{"c", false, 0.4, 15}, // relative to the style's factor
		{"d", true, 1, -10},   // absolute, then relative to that
		{"e", false, 0.8, 15}, // the braces ended the scope
	}
	for i, e := range expected {
		run := runs[i]
		assertEqString(t, e.text, run.Text)
		assertEqBool(t, e.hasWidth, run.HasWidthFactor)
		assertEqFloat64(t, e.width, run.EffectiveWidthFactor(0.8))
		assertEqFloat64(t, e.oblique, run.EffectiveObliqueAngle(15))
	}
}

func TestMTextRunsFormattingToggles(t *testing.T) {
	runs := ParseMTextRuns("\\Lu\\l\\Oo\\o\\Ks\\k")
	assertEqInt(t, 3, len(runs))
	assertEqBool(t, true, runs[0].Underline)
	assertEqBool(t, true, runs[1].Overline)
	assertEqBool(t, false, runs[1].Underline)
	assertEqBool(t, true, runs[2].Strike)
}

func TestAttributePlainText(t *testing.T) {
	attribute := NewAttribute()
	attribute.Value = "%%c50 \\U+0150r"
	assertEqString(t, "Ø50 Őr", attribute.PlainText())

	// a multiline attribute's text is its embedded MText
	attribute.AttributeType = 2
	attribute.MText.Text = "{\\fArial;first}\\Psecond"
	assertEqString(t, "first\nsecond", attribute.PlainText())

	definition := NewAttributeDefinition()
	definition.Value = "%%uDefault%%u"
	assertEqString(t, "Default", definition.PlainText())
	definition.AttributeType = 4
	definition.MText.Text = "a\\Pb"
	assertEqString(t, "a\nb", definition.PlainText())
}

func TestTextPlainText(t *testing.T) {
	for _, testCase := range []struct{ value, plain string }{
		{"plain", "plain"},
		{"%%c100", "Ø100"},
		{"90%%D %%P1 %%%", "90° ±1 %"},
		{"%%uunderlined%%u and %%ooverlined%%o", "underlined and overlined"},
		{"%%065%%066", "AB"},
		{"%%", "%%"},
		{"50%%x", "50%%x"},
		{"\\U+0150r \\M+182A0", "Őr あ"},
		{"C:\\path\\Users", "C:\\path\\Users"},
	} {
		text := NewText()
		text.Value = testCase.value
		assertEqString(t, testCase.plain, text.PlainText())
	}
}

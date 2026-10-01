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
		{"%%c50 45%%d %%p0.5 100%%%", "⌀50 45° ±0.5 100%"},
		{"tab^Iand^Jnewline^ caret", "tab\tand\nnewline^caret"},
		// stacked fractions
		{"1\\S1/2;\"", "11/2\""},
		{"\\S+0.01^-0.02;", "+0.01/-0.02"},
		{"\\S3#4;", "3/4"},
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

func TestMTextRunsRelativeValuesMultiply(t *testing.T) {
	runs := ParseMTextRuns("{\\H2x;{\\H0.5x;\\W0.5;\\W2x;x}}")
	assertEqInt(t, 1, len(runs))
	assertEqFloat64(t, 1.0, runs[0].HeightFactor)
	assertEqFloat64(t, 1.0, runs[0].WidthFactor)
}

func TestMTextRunsFormattingToggles(t *testing.T) {
	runs := ParseMTextRuns("\\Lu\\l\\Oo\\o\\Ks\\k")
	assertEqInt(t, 3, len(runs))
	assertEqBool(t, true, runs[0].Underline)
	assertEqBool(t, true, runs[1].Overline)
	assertEqBool(t, false, runs[1].Underline)
	assertEqBool(t, true, runs[2].Strike)
}

func TestTextPlainText(t *testing.T) {
	for _, testCase := range []struct{ value, plain string }{
		{"plain", "plain"},
		{"%%c100", "⌀100"},
		{"90%%D %%P1 %%%", "90° ±1 %"},
		{"%%uunderlined%%u and %%ooverlined%%o", "underlined and overlined"},
		{"%%065%%066", "AB"},
		{"%%", "%%"},
		{"50%%x", "50%%x"},
	} {
		text := NewText()
		text.Value = testCase.value
		assertEqString(t, testCase.plain, text.PlainText())
	}
}

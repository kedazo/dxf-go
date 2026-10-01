package dxf

import (
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// codePageEncodings maps the `$DWGCODEPAGE` names AutoCAD writes to text encodings. Pre-2007 files store text in this
// code page; 2007+ files are always UTF-8.
var codePageEncodings = map[string]encoding.Encoding{
	"ANSI_874":   charmap.Windows874,
	"ANSI_932":   japanese.ShiftJIS,
	"ANSI_936":   simplifiedchinese.GBK,
	"ANSI_949":   korean.EUCKR,
	"ANSI_950":   traditionalchinese.Big5,
	"ANSI_1250":  charmap.Windows1250,
	"ANSI_1251":  charmap.Windows1251,
	"ANSI_1252":  charmap.Windows1252,
	"ANSI_1253":  charmap.Windows1253,
	"ANSI_1254":  charmap.Windows1254,
	"ANSI_1255":  charmap.Windows1255,
	"ANSI_1256":  charmap.Windows1256,
	"ANSI_1257":  charmap.Windows1257,
	"ANSI_1258":  charmap.Windows1258,
	"DOS437":     charmap.CodePage437,
	"DOS850":     charmap.CodePage850,
	"DOS852":     charmap.CodePage852,
	"DOS855":     charmap.CodePage855,
	"DOS860":     charmap.CodePage860,
	"DOS863":     charmap.CodePage863,
	"DOS865":     charmap.CodePage865,
	"DOS866":     charmap.CodePage866,
	"DOS932":     japanese.ShiftJIS,
	"ISO8859-1":  charmap.ISO8859_1,
	"ISO8859-2":  charmap.ISO8859_2,
	"ISO8859-3":  charmap.ISO8859_3,
	"ISO8859-4":  charmap.ISO8859_4,
	"ISO8859-5":  charmap.ISO8859_5,
	"ISO8859-6":  charmap.ISO8859_6,
	"ISO8859-7":  charmap.ISO8859_7,
	"ISO8859-8":  charmap.ISO8859_8,
	"ISO8859-9":  charmap.ISO8859_9,
	"ISO8859-10": charmap.ISO8859_10,
	"KOI8-R":     charmap.KOI8R,
	"MACINTOSH":  charmap.Macintosh,
	"GB2312":     simplifiedchinese.GBK, // GB2312 is a subset of GBK
	"BIG5":       traditionalchinese.Big5,
}

// encodingFromCodePage returns the encoding for a `$DWGCODEPAGE` value, or nil if it is unknown.
func encodingFromCodePage(name string) encoding.Encoding {
	return codePageEncodings[strings.ToUpper(strings.TrimSpace(name))]
}

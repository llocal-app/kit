package survey

import (
	"math"
	"path"
	"slices"
	"strconv"
	"strings"
)

// Screens is what the front end's source shows about its screens, counted over the files that
// hold markup: words of text, controls, the colours and font sizes the code names, whether it
// is set up for phones, and the accessibility basics the markup misses. It reads the source
// only, never a page as a browser draws it.
type Screens struct {
	Files     int `json:"files"`
	Words     int `json:"words"`
	Controls  int `json:"controls"`
	Colors    int `json:"colors"`
	FontSizes int `json:"font_sizes"`
	// Viewport: a page sets the viewport for phones, or the framework does by default.
	Viewport bool `json:"viewport,omitempty"`
	// Responsive counts the layout rules for screen widths: media queries and classes like md:.
	Responsive int    `json:"responsive"`
	Signs      []Sign `json:"signs,omitempty"`
}

// Sign is an accessibility basic the markup misses, how often, and the files it is in. Kind is
// image_no_alt, field_no_label, icon_button, heading_skip or low_contrast.
type Sign struct {
	Kind  string   `json:"kind"`
	Count int      `json:"count"`
	Files []string `json:"files"`
}

var (
	uiExts      = []string{".jsx", ".tsx", ".vue", ".svelte", ".astro", ".html", ".htm"}
	uiLibrary   = compile(`(^|/)components/ui/`)
	scriptStyle = compile(`(?is)<(script|style)\b.*?</(script|style)>`)
	// textNode, hexColor: the byte before a match is checked in code, as a pattern starting with
	// a literal is scanned many times faster.
	textNode = compile(`>([^<>]+)<`)
	block    = compile(`\{[^{}]*\}`)
	control  = compile(`<(button|Button|a|Link|NavLink|input|Input|select|Select|textarea|Textarea)\b`)
	hexColor = compile(`#([0-9a-fA-F]{6}|[0-9a-fA-F]{3})\b`)
	cssSize  = compile(`font-size:\s*[\d.]+(?:px|rem|em)|fontSize:\s*["']?[\d.]+`)
	viewport = compile(`name=["']viewport["']|export\s+const\s+viewport\b`)
	// Class names are matched a word at a time (classes), not over the whole text.
	twColor     = compile(`^(?:bg|text|border|from|to|via|ring|fill|stroke)-(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-\d{2,3}$`)
	twSize      = compile(`^text-(?:xs|sm|base|lg|xl|[2-9]xl|\[[\d.]+(?:px|rem|em)\])$`)
	breakpoints = []string{"sm:", "md:", "lg:", "xl:", "2xl:"}
	imgTag      = compile(`<(?:img|Image)\b[^>]*>`)
	fieldTag    = compile(`<(?:input|Input|textarea|Textarea|select|Select)\b[^>]*>`)
	quietField  = compile(`type=["'](?:hidden|submit|button|image|reset)["']`)
	labelTag    = compile(`<(?:label|Label)\b`)
	hasName     = compile(`aria-label(?:ledby)?=|\btitle=`)
	iconButton  = compile(`(?s)<(?:button|Button)\b([^>]*)>\s*(?:<svg\b[^>]*>(?:[^<]|<[^/bB]|</[^sbB])*</svg>|<(?:svg|[A-Z]\w*)\b[^>]*?/>)\s*</(?:button|Button)>`)
	heading     = compile(`<h([1-6])\b`)
	cssColor    = compile(`(?i)(?:^|[\s;{,"'])color\s*:\s*["']?#([0-9a-f]{6}|[0-9a-f]{3})\b`)
	cssBack     = compile(`(?i)background(?:-color|Color)?\s*:\s*["']?#([0-9a-f]{6}|[0-9a-f]{3})\b`)
)

// screens counts what the files that draw the screens hold, leaving out tests and the copies
// of a component library under components/ui.
func (p *project) screens() {
	sc := Screens{Viewport: p.s.fw("nextjs")}
	colors, sizes := map[string]bool{}, map[string]bool{}
	signs := map[string]*Sign{}
	sign := func(kind, file string, n int) {
		if n == 0 {
			return
		}
		if signs[kind] == nil {
			signs[kind] = &Sign{Kind: kind}
		}
		signs[kind].Count += n
		signs[kind].Files = add(signs[kind].Files, file)
	}
	for _, n := range p.code {
		if !slices.Contains(uiExts, path.Ext(n)) || notRuntime.MatchString(n) || uiLibrary.MatchString(n) {
			continue
		}
		t := p.text[n]
		if !strings.Contains(t, "</") && !strings.Contains(t, "/>") {
			continue
		}
		sc.Files++
		sc.Viewport = sc.Viewport || viewport.MatchString(t)
		sc.Responsive += strings.Count(t, "@media")
		sc.Controls += len(control.FindAllString(t, -1))
		for _, loc := range hexColor.FindAllStringSubmatchIndex(t, -1) {
			if loc[0] > 0 && strings.IndexByte(":\"' \t\n([", t[loc[0]-1]) >= 0 {
				colors[strings.ToLower(t[loc[2]:loc[3]])] = true
			}
		}
		for _, s := range cssSize.FindAllString(t, -1) {
			sizes[s] = true
		}
		for _, c := range classes(t) {
			if slices.ContainsFunc(breakpoints, func(b string) bool { return strings.HasPrefix(c, b) }) {
				sc.Responsive++
			}
			c = c[strings.LastIndexByte(c, ':')+1:]
			switch {
			// Only a name ending in a digit can be a colour, which spares the pattern most words.
			case strings.ContainsAny(c[max(0, len(c)-1):], "0123456789") && twColor.MatchString(c):
				_, shade, _ := strings.Cut(c, "-")
				colors[shade] = true
			case twSize.MatchString(c):
				sizes[c] = true
			}
		}
		markup := scriptStyle.ReplaceAllString(t, "")
		for _, loc := range textNode.FindAllStringSubmatchIndex(markup, -1) {
			if loc[0] > 0 && (markup[loc[0]-1] == '=' || markup[loc[0]-1] == '-') {
				continue
			}
			if text := block.ReplaceAllString(markup[loc[2]:loc[3]], " "); !strings.ContainsAny(text, ";={}") {
				sc.Words += len(slices.DeleteFunc(strings.Fields(text), func(w string) bool { return !strings.ContainsAny(strings.ToLower(w), "abcdefghijklmnopqrstuvwxyz") }))
			}
		}
		noAlt := 0
		for _, tag := range imgTag.FindAllString(t, -1) {
			if !strings.Contains(tag, "alt=") {
				noAlt++
			}
		}
		sign("image_no_alt", n, noAlt)
		fields := 0
		for _, tag := range fieldTag.FindAllString(t, -1) {
			if !quietField.MatchString(tag) && !hasName.MatchString(tag) {
				fields++
			}
		}
		sign("field_no_label", n, max(0, fields-len(labelTag.FindAllString(t, -1))))
		icons := 0
		for _, m := range iconButton.FindAllStringSubmatch(t, -1) {
			if !hasName.MatchString(m[1]) {
				icons++
			}
		}
		sign("icon_button", n, icons)
		skips, last := 0, 0
		for _, m := range heading.FindAllStringSubmatch(t, -1) {
			level := int(m[1][0] - '0')
			if last > 0 && level > last+1 {
				skips++
			}
			last = level
		}
		sign("heading_skip", n, skips)
		low := 0
		for _, b := range block.FindAllString(t, -1) {
			if !strings.Contains(b, "ackground") {
				continue
			}
			if fg := cssColor.FindStringSubmatch(b); fg != nil {
				if bg := cssBack.FindStringSubmatch(b); bg != nil && contrast(fg[1], bg[1]) < 4.5 {
					low++
				}
			}
		}
		sign("low_contrast", n, low)
	}
	if sc.Files == 0 {
		return
	}
	sc.Colors, sc.FontSizes = len(colors), len(sizes)
	for _, kind := range []string{"image_no_alt", "field_no_label", "icon_button", "heading_skip", "low_contrast"} {
		if s := signs[kind]; s != nil {
			s.Files = capped(&p.s, s.Files)
			sc.Signs = append(sc.Signs, *s)
		}
	}
	p.s.Screens = &sc
}

// classes are the words of t that may be class names, split at spaces, quotes, braces and the
// like, but not at the square brackets of a class like text-[13px].
func classes(t string) []string {
	return strings.FieldsFunc(t, func(r rune) bool { return r <= ' ' || strings.ContainsRune("\"'`{}(),;<>=", r) })
}

// contrast is the contrast ratio of two colours written in hex, as WCAG works it out.
func contrast(a, b string) float64 {
	la, lb := luminance(a), luminance(b)
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

func luminance(hex string) float64 {
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	var l float64
	for i, weight := range []float64{0.2126, 0.7152, 0.0722} {
		v, _ := strconv.ParseUint(hex[2*i:2*i+2], 16, 8)
		c := float64(v) / 255
		if c <= 0.03928 {
			c /= 12.92
		} else {
			c = math.Pow((c+0.055)/1.055, 2.4)
		}
		l += weight * c
	}
	return l
}

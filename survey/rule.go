package survey

import (
	"regexp"
	"regexp/syntax"
	"slices"
	"strings"
)

// rule is a regular expression that skips text holding none of the literals every match
// must contain. Most rules match few files, and strings.Contains is far faster than the
// regexp scan it saves.
type rule struct {
	*regexp.Regexp
	needles []string
}

func compile(expr string) rule {
	re := regexp.MustCompile(expr)
	parsed, _ := syntax.Parse(expr, syntax.Perl)
	return rule{re, needles(parsed.Simplify())}
}

// needles are strings one of which every match contains, or nil when there is no such set.
func needles(re *syntax.Regexp) []string {
	switch re.Op {
	case syntax.OpLiteral:
		if re.Flags&syntax.FoldCase == 0 {
			return []string{string(re.Rune)}
		}
	case syntax.OpCapture, syntax.OpPlus:
		return needles(re.Sub[0])
	case syntax.OpRepeat:
		if re.Min > 0 {
			return needles(re.Sub[0])
		}
	case syntax.OpConcat:
		var best []string
		for _, sub := range re.Sub {
			if n := needles(sub); n != nil && (best == nil || shortest(n) > shortest(best)) {
				best = n
			}
		}
		return best
	case syntax.OpAlternate:
		var all []string
		for _, sub := range re.Sub {
			n := needles(sub)
			if n == nil {
				return nil
			}
			all = append(all, n...)
		}
		return all
	}
	return nil
}

func shortest(words []string) int {
	return len(slices.MinFunc(words, func(a, b string) int { return len(a) - len(b) }))
}

func (r rule) maybe(t string) bool {
	return r.needles == nil || slices.ContainsFunc(r.needles, func(n string) bool { return strings.Contains(t, n) })
}

func (r rule) MatchString(t string) bool { return r.maybe(t) && r.Regexp.MatchString(t) }

func (r rule) FindStringSubmatch(t string) []string {
	if !r.maybe(t) {
		return nil
	}
	return r.Regexp.FindStringSubmatch(t)
}

func (r rule) FindAllStringSubmatch(t string, n int) [][]string {
	if !r.maybe(t) {
		return nil
	}
	return r.Regexp.FindAllStringSubmatch(t, n)
}

func (r rule) FindAllStringSubmatchIndex(t string, n int) [][]int {
	if !r.maybe(t) {
		return nil
	}
	return r.Regexp.FindAllStringSubmatchIndex(t, n)
}

// Package agent is the kit's instructions for a builder's AI agent: the steps
// llocal.app/agent.md starts with, the Claude Code plugin, the Cursor rule, the AGENTS.md
// snippet and the line to paste for each AI tool (connect.json), which llocal.app serves
// under /agent/. connect.json's published says whether the kit's public repository is up;
// until it is, llocal.app serves AGENT.md and STANDARD.md for that time (ForKit).
package agent

import (
	"embed"
	"regexp"
)

//go:embed AGENT.md AGENTS.md connect.json cursor all:claude
var Files embed.FS

// Steps is AGENT.md.
//
//go:embed AGENT.md
var Steps []byte

// unpublished marks text in AGENT.md and STANDARD.md that holds only once the kit is
// public, after a comment with what holds until then:
//
//	<!-- unpublished
//	what holds now
//	-->
//	what holds once the kit is public
//	<!-- /unpublished -->
var unpublished = regexp.MustCompile(`(?s)<!-- unpublished\n(.*?)-->\n(.*?)<!-- /unpublished -->\n`)

// ForKit is text from AGENT.md or STANDARD.md with the part that holds whether the kit is
// published or not.
func ForKit(b []byte, published bool) []byte {
	return unpublished.ReplaceAll(b, []byte(map[bool]string{false: "$1", true: "$2"}[published]))
}

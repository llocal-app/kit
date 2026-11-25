// Package agent is the kit's instructions for a builder's AI agent: the steps
// llocal.app/agent.md starts with, the Claude Code plugin, the Cursor rule, the AGENTS.md
// snippet and the line to paste for each AI tool (connect.json), which llocal.app serves
// under /agent/.
package agent

import "embed"

//go:embed AGENT.md AGENTS.md connect.json cursor all:claude
var Files embed.FS

// Steps is AGENT.md.
//
//go:embed AGENT.md
var Steps []byte

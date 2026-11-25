package kit

import _ "embed"

// Standard is STANDARD.md, which llocal's agent reads and llocal.app/agent.md serves.
//
//go:embed STANDARD.md
var Standard []byte

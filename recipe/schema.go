package recipe

import _ "embed"

// Schema is llocal.json's JSON schema, which llocal.app serves at /llocal.schema.json.
//
//go:embed llocal.schema.json
var Schema []byte

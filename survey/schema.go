package survey

import _ "embed"

// Schema is survey.json's JSON schema, which llocal.app serves at /survey.schema.json.
//
//go:embed survey.schema.json
var Schema []byte

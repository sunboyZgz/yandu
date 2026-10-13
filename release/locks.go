package release

import _ "embed"

//go:embed dependencies.lock.json
var Dependencies []byte

//go:embed storage-components.lock.json
var StorageComponents []byte

//go:build localnode

package config

import _ "embed"

//go:embed nodes/nodes.local.yml
var nodesConfYmlBytes []byte
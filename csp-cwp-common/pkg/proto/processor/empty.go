//NOTE: only kept here as a hack for go mod shortcomings when a folder has only proto files and no go files
package processor

import (
	_ "github.com/gogo/protobuf/gogoproto"
	_ "github.com/gogo/protobuf/jsonpb"
)

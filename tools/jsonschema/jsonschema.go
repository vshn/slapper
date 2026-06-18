package main

import (
	"encoding/json"
	"os"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/vshn/slapper/pkg/servicebundle"
)

func main() {
	schema, err := jsonschema.For[servicebundle.ServiceBundle](nil)
	if err != nil {
		panic(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(schema); err != nil {
		panic(err)
	}
}

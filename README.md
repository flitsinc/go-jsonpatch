# go-jsonpatch

go-jsonpatch is a tiny library that applies a subset of JSON Patch operations to a map-based document.

## Installation

```
go get github.com/blixt/go-jsonpatch/jsonpatch
```

## Example

```go
package main

import (
    "fmt"
    "github.com/blixt/go-jsonpatch/jsonpatch"
)

func main() {
    doc := map[string]any{"greeting": "world", "counter": 0}
    patch := []map[string]any{
        {"op": "str_ins", "path": "/greeting", "pos": 0, "str": "Hello "},
        {"op": "inc", "path": "/counter", "inc": 1},
    }
    if err := jsonpatch.Apply(doc, patch); err != nil {
        panic(err)
    }
    fmt.Println(doc)
    // Output: map[greeting:"Hello world" counter:1]
}
```

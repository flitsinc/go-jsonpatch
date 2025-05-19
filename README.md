# go-jsonpatch

go-jsonpatch is a tiny library that applies a subset of JSON Patch operations to a map-based document.

## Installation

```
go get github.com/flitsinc/go-jsonpatch/jsonpatch
```

## Example

```go
package main

import (
    "fmt"
    "github.com/flitsinc/go-jsonpatch/jsonpatch"
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

## Supported operations

go-jsonpatch implements the operations from [RFC 6902](https://datatracker.ietf.org/doc/html/rfc6902) along with a few extensions. Paths are specified using JSON Pointer notation.

- **add** (RFC 6902): add a value to a map key or slice index
- **remove** (RFC 6902): remove a value from a map or slice
- **replace** (RFC 6902): replace an existing value
- **move** (RFC 6902): move a value from one path to another
- **copy** (RFC 6902): copy a value from one path to another
- **test** (RFC 6902): assert a value equals the provided one
- **str_ins**: insert the given substring at `pos` in the string found at the path
- **str_del**: delete `len` characters starting at `pos` in the string at the path
- **inc**: increment a numeric value by the provided amount

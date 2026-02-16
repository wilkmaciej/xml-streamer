## xml-streamer

Streaming XML parser for Go with XPath query support. Parses XML incrementally, emitting matched elements through a channel for memory-efficient processing of large documents.

## Requirements

1. Go 1.26 or higher

## Installation

```shell
go get github.com/wilkmaciej/xml-streamer
```

## Usage

```go
package main

import (
	"context"
	"fmt"
	"os"

	xmlstreamer "github.com/wilkmaciej/xml-streamer"
	"github.com/wilkmaciej/xpath"
)

func main() {
	file, _ := os.Open("data.xml")
	defer file.Close()

	expr, _ := xpath.Compile("title")

	parser := xmlstreamer.NewParser(context.Background(), file, []string{"item"}, 0)

	for node := range parser.Stream() {
		fmt.Println(xmlstreamer.ElementString(node.Evaluate(expr)))
		node.Release()
	}
}
```

`NewParser` accepts any `io.Reader`, a list of element names to stream, and a channel buffer size (0 for default of 8). Each emitted `*XMLElement` supports XPath evaluation via `Evaluate()` and should be returned to the pool with `Release()` after processing.

See [perf_test/main.go](perf_test/main.go) for a more complete example with multiple XPath expressions and gzip decompression.

## Testing

```shell
go test -race ./...
```

To run the performance benchmark:

```shell
GOMAXPROCS=1 go run ./perf_test/
```

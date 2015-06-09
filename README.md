# A Go rstatd client

[![GoDoc](https://godoc.org/github.com/yhat/rstatd?status.svg)](https://godoc.org/github.com/yhat/rstatd)

```go
package main

import (
    "fmt"

    "github.com/yhat/rstatd"
)

func main() {
    stats, err := rstatd.ReadStats()
    if err != nil {
        panic(err)
    }
    fmt.Println(stats.CPUUser, stats.CPUNice, stats.CPUSys, stats.CPUIdle)
}
```

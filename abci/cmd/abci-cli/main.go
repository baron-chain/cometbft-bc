package main

import (
    "fmt"
    "os"
)

// main initializes and executes the Baron Chain CLI
func main() {
    if err := Execute(); err != nil {
        fmt.Fprintf(os.Stderr, "baron-cli error: %v\n", err)
        os.Exit(1)
    }
}

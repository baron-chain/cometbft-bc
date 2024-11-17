package main

import (
    "bufio"
    "context"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/baron-chain/cometbft-bc/abci/types"
    bcnet "github.com/baron-chain/cometbft-bc/libs/net"
)

const (
    defaultSocketPath = "unix://baron-test.sock"
    logInterval      = 1000
    bufferSize       = 1024 * 1024 // 1MB
    timeout          = 30 * time.Second
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    // Connect to Baron Chain server
    conn, err := connectToServer(ctx)
    if err != nil {
        log.Fatalf("Baron Chain connection error: %v", err)
    }
    defer conn.Close()

    // Start response reader
    done := make(chan error, 1)
    go readResponses(conn, done)

    // Send requests
    if err := sendRequests(conn); err != nil {
        log.Fatalf("Baron Chain request error: %v", err)
    }

    // Wait for completion or error
    select {
    case err := <-done:
        if err != nil {
            log.Fatalf("Baron Chain response error: %v", err)
        }
    case <-ctx.Done():
        log.Fatalf("Baron Chain test timeout")
    }
}

func connectToServer(ctx context.Context) (net.Conn, error) {
    conn, err := bcnet.Connect(defaultSocketPath)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to Baron Chain server: %w", err)
    }

    // Set connection timeout
    if tc, ok := conn.(*net.TCPConn); ok {
        tc.SetKeepAlive(true)
        tc.SetKeepAlivePeriod(3 * time.Minute)
    }

    return conn, nil
}

func readResponses(conn net.Conn, done chan<- error) {
    var counter int
    reader := bufio.NewReaderSize(conn, bufferSize)

    for {
        res := &types.Response{}
        if err := types.ReadMessage(reader, res); err != nil {
            done <- fmt.Errorf("response read error: %w", err)
            return
        }

        counter++
        if counter%logInterval == 0 {
            log.Printf("Baron Chain: Processed %d responses\n", counter)
        }

        // Process response based on type
        processResponse(res)
    }
}

func sendRequests(conn net.Conn) error {
    var counter int
    writer := bufio.NewWriterSize(conn, bufferSize)

    for {
        // Create test request
        req := createTestRequest()

        // Send request
        if err := types.WriteMessage(req, writer); err != nil {
            return fmt.Errorf("request write error: %w", err)
        }

        if err := writer.Flush(); err != nil {
            return fmt.Errorf("buffer flush error: %w", err)
        }

        counter++
        if counter%logInterval == 0 {
            log.Printf("Baron Chain: Sent %d requests\n", counter)
        }
    }
}

func createTestRequest() *types.Request {
    return types.ToRequestEcho("baron-chain-test")
}

func processResponse(res *types.Response) {
    switch r := res.Value.(type) {
    case *types.Response_Echo:
        // Process echo response
        handleEchoResponse(r.Echo)
    case *types.Response_Exception:
        // Handle exception
        log.Printf("Baron Chain exception: %s", r.Exception.Error)
    default:
        // Handle other response types
    }
}

func handleEchoResponse(echo *types.ResponseEcho) {
    if echo.Message != "baron-chain-test" {
        log.Printf("Unexpected echo response: %s", echo.Message)
    }
}

func init() {
    log.SetOutput(os.Stdout)
    log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
}

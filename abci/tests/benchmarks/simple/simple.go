package main

import (
    "bufio"
    "context"
    "fmt"
    "io"
    "log"
    "reflect"
    "time"

    "github.com/baron-chain/cometbft-bc/abci/types"
    bcnet "github.com/baron-chain/cometbft-bc/libs/net"
)

const (
    defaultSocketPath = "unix://test.sock"
    logInterval      = 1000
    bufferSize       = 1024 * 1024 // 1MB
    timeout          = 30 * time.Second
)

type BCClient struct {
    conn   io.ReadWriter
    writer *bufio.Writer
    ctx    context.Context
    cancel context.CancelFunc
}

func newBCClient(socketPath string) (*BCClient, error) {
    conn, err := bcnet.Connect(socketPath)
    if err != nil {
        return nil, fmt.Errorf("baron chain connection failed: %w", err)
    }

    ctx, cancel := context.WithTimeout(context.Background(), timeout)

    return &BCClient{
        conn:   conn,
        writer: bufio.NewWriterSize(conn, bufferSize),
        ctx:    ctx,
        cancel: cancel,
    }, nil
}

func (c *BCClient) close() {
    c.cancel()
    if closer, ok := c.conn.(io.Closer); ok {
        closer.Close()
    }
}

func (c *BCClient) makeRequest(req *types.Request) (*types.Response, error) {
    // Write request with flush
    if err := c.writeRequest(req); err != nil {
        return nil, fmt.Errorf("baron chain request write failed: %w", err)
    }

    // Read response with flush verification
    return c.readResponse()
}

func (c *BCClient) writeRequest(req *types.Request) error {
    // Write main request
    if err := types.WriteMessage(req, c.writer); err != nil {
        return fmt.Errorf("request write failed: %w", err)
    }

    // Write flush request
    if err := types.WriteMessage(types.ToRequestFlush(), c.writer); err != nil {
        return fmt.Errorf("flush request write failed: %w", err)
    }

    // Flush writer
    if err := c.writer.Flush(); err != nil {
        return fmt.Errorf("buffer flush failed: %w", err)
    }

    return nil
}

func (c *BCClient) readResponse() (*types.Response, error) {
    // Read main response
    response := &types.Response{}
    if err := types.ReadMessage(c.conn, response); err != nil {
        return nil, fmt.Errorf("response read failed: %w", err)
    }

    // Read and verify flush response
    flushResponse := &types.Response{}
    if err := types.ReadMessage(c.conn, flushResponse); err != nil {
        return nil, fmt.Errorf("flush response read failed: %w", err)
    }

    if _, ok := flushResponse.Value.(*types.Response_Flush); !ok {
        return nil, fmt.Errorf("unexpected flush response type: %v", reflect.TypeOf(flushResponse))
    }

    return response, nil
}

func main() {
    // Initialize client
    client, err := newBCClient(defaultSocketPath)
    if err != nil {
        log.Fatalf("Baron Chain client initialization failed: %v", err)
    }
    defer client.close()

    // Process requests
    var counter int

    for {
        select {
        case <-client.ctx.Done():
            log.Printf("Baron Chain client timeout after %d requests", counter)
            return
        default:
            // Create and send echo request
            req := types.ToRequestEcho("baron-chain-test")
            resp, err := client.makeRequest(req)
            if err != nil {
                log.Fatalf("Baron Chain request failed at count %d: %v", counter, err)
            }

            // Process response if needed
            if _, ok := resp.Value.(*types.Response_Echo); !ok {
                log.Fatalf("Unexpected response type at count %d: %v", counter, reflect.TypeOf(resp))
            }

            // Log progress
            counter++
            if counter%logInterval == 0 {
                fmt.Printf("Processed %d Baron Chain requests\n", counter)
            }
        }
    }
}

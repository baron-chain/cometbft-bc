package abcicli_test

import (
    "fmt"
    "sync"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    abcicli "github.com/baron-chain/cometbft-bc/abci/client"
    "github.com/baron-chain/cometbft-bc/abci/server"
    "github.com/baron-chain/cometbft-bc/abci/types"
    bcrand "github.com/baron-chain/cometbft-bc/libs/rand"
    "github.com/baron-chain/cometbft-bc/libs/service"
)

const (
    testTimeout     = time.Second
    responseTimeout = 20 * time.Millisecond
    blockDelay     = 200 * time.Millisecond
    portRangeStart = 20000
    portRangeEnd   = 30000
)

type MockApp struct {
    types.BaseApplication
    wg *sync.WaitGroup
}

func (m MockApp) BeginBlock(req types.RequestBeginBlock) types.ResponseBeginBlock {
    time.Sleep(blockDelay)
    return types.ResponseBeginBlock{}
}

func (m MockApp) CheckTx(r types.RequestCheckTx) types.ResponseCheckTx {
    if m.wg != nil {
        m.wg.Wait()
    }
    return m.BaseApplication.CheckTx(r)
}

func setupTestEnvironment(t *testing.T, app types.Application) (service.Service, abcicli.Client) {
    port := portRangeStart + bcrand.Int32()%(portRangeEnd-portRangeStart)
    addr := fmt.Sprintf("localhost:%d", port)

    server, err := server.NewServer(addr, "socket", app)
    require.NoError(t, err, "Failed to create server")
    
    err = server.Start()
    require.NoError(t, err, "Failed to start server")

    client := abcicli.NewSocketClient(addr, true)
    err = client.Start()
    require.NoError(t, err, "Failed to start client")

    return server, client
}

func TestSyncCallsSuccess(t *testing.T) {
    app := MockApp{}
    server, client := setupTestEnvironment(t, app)
    defer func() {
        require.NoError(t, server.Stop(), "Failed to stop server")
        require.NoError(t, client.Stop(), "Failed to stop client")
    }()

    responseChan := make(chan error, 1)
    go func() {
        reqRes := client.BeginBlockAsync(types.RequestBeginBlock{})
        err := client.FlushSync()
        if err != nil {
            responseChan <- err
            return
        }

        res := reqRes.Response.GetBeginBlock()
        if res == nil {
            responseChan <- fmt.Errorf("null begin block response")
            return
        }
        responseChan <- client.Error()
    }()

    select {
    case <-time.After(testTimeout):
        t.Fatal("Test timed out waiting for response")
    case err := <-responseChan:
        assert.NoError(t, err, "Expected successful sync call")
    }
}

func TestSyncCallsFailure(t *testing.T) {
    app := MockApp{}
    server, client := setupTestEnvironment(t, app)
    defer func() {
        _ = client.Stop()
    }()

    responseChan := make(chan error, 1)
    go func() {
        reqRes := client.BeginBlockAsync(types.RequestBeginBlock{})
        flush := client.FlushAsync()

        // Wait for network operations
        time.Sleep(responseTimeout)

        // Force connection failure
        require.NoError(t, server.Stop(), "Failed to stop server")

        // Wait for pending operations
        reqRes.Wait()
        flush.Wait()
        responseChan <- client.Error()
    }()

    select {
    case <-time.After(testTimeout):
        t.Fatal("Test timed out waiting for error response")
    case err := <-responseChan:
        assert.Error(t, err, "Expected error due to server shutdown")
    }
}

func TestCallbackBehavior(t *testing.T) {
    t.Run("Late Callback", func(t *testing.T) {
        wg := &sync.WaitGroup{}
        wg.Add(1)
        app := MockApp{wg: wg}
        
        _, client := setupTestEnvironment(t, app)
        defer func() {
            require.NoError(t, client.Stop(), "Failed to stop client")
        }()

        reqRes := client.CheckTxAsync(types.RequestCheckTx{})
        callbackChan := make(chan struct{})
        
        reqRes.SetCallback(func(_ *types.Response) {
            close(callbackChan)
        })

        wg.Done()
        
        select {
        case <-time.After(testTimeout):
            t.Fatal("Callback was not invoked")
        case <-callbackChan:
            // Success - callback was invoked
        }

        var callbackInvoked bool
        reqRes.SetCallback(func(_ *types.Response) {
            callbackInvoked = true
        })
        assert.True(t, callbackInvoked, "Late callback should be invoked immediately")
    })

    t.Run("Early Callback", func(t *testing.T) {
        wg := &sync.WaitGroup{}
        wg.Add(1)
        app := MockApp{wg: wg}
        
        _, client := setupTestEnvironment(t, app)
        defer func() {
            require.NoError(t, client.Stop(), "Failed to stop client")
        }()

        reqRes := client.CheckTxAsync(types.RequestCheckTx{})
        callbackChan := make(chan struct{})
        
        reqRes.SetCallback(func(_ *types.Response) {
            close(callbackChan)
        })
        
        wg.Done()

        select {
        case <-time.After(testTimeout):
            t.Fatal("Early callback was not invoked")
        case <-callbackChan:
            // Success - callback was invoked
        }
    })
}

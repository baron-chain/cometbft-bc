package pex

import (
	"errors"
	"net"
	"sync"

	"github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cometbft/cometbft/libs/log"
	"github.com/cometbft/cometbft/libs/service"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/p2p/pex"
	"github.com/cometbft/cometbft/version"
	"github.com/cosmos/gogoproto/proto"
)

const (
	testListenAddr  = "0.0.0.0:98992"
	testDialAddr    = "127.0.0.1"
	testDialPort    = "123.123.123"
	testPeerPort    = 98991
	testAddrBookDir = "./testdata/addrbook1"
)

var (
	errNilReactor = errors.New("pex reactor initialization failed")
	errNilMessage = errors.New("failed to unmarshal message")

	// Singleton instances protected by mutex
	testState struct {
		sync.Once
		pexReactor *pex.Reactor
		peer       p2p.Peer
		err        error
	}
)

// TestPeer implements a mock peer for testing
type TestPeer struct {
	*service.BaseService
	store map[string]interface{}
}

// Ensure TestPeer implements p2p.Peer interface
var _ p2p.Peer = (*TestPeer)(nil)

// NewTestPeer creates a new test peer instance
func NewTestPeer() *TestPeer {
	peer := &TestPeer{
		store: make(map[string]interface{}),
	}
	peer.BaseService = service.NewBaseService(nil, "TestPeer", peer)
	return peer
}

// initializeTestState sets up the global test state
func initializeTestState() error {
	testState.Do(func() {
		// Initialize address book and reactor
		addrBook := pex.NewAddrBook(testAddrBookDir, false)
		reactor := pex.NewReactor(addrBook, &pex.ReactorConfig{SeedMode: false})
		if reactor == nil {
			testState.err = errNilReactor
			return
		}

		// Setup reactor
		reactor.SetLogger(log.NewNopLogger())
		testState.pexReactor = reactor

		// Initialize and add peer
		testState.peer = NewTestPeer()
		reactor.AddPeer(testState.peer)
	})

	return testState.err
}

// getTestNodeInfo creates default node info for testing
func getTestNodeInfo() p2p.DefaultNodeInfo {
	privKey := ed25519.GenPrivKey()
	nodeID := p2p.PubKeyToID(privKey.PubKey())

	return p2p.DefaultNodeInfo{
		ProtocolVersion: p2p.NewProtocolVersion(
			version.P2PProtocol,
			version.BlockProtocol,
			0,
		),
		DefaultNodeID: nodeID,
		ListenAddr:    testListenAddr,
		Moniker:       "test-node",
	}
}

// createTestSwitch creates a P2P switch for testing
func createTestSwitch() *p2p.Switch {
	cfg := config.DefaultP2PConfig()
	cfg.PexReactor = true

	return p2p.MakeSwitch(
		cfg,
		0,
		testDialAddr,
		testDialPort,
		func(i int, sw *p2p.Switch) *p2p.Switch { return sw },
	)
}

// TestPeer implementation of p2p.Peer interface
func (tp *TestPeer) FlushStop()                           {}
func (tp *TestPeer) ID() p2p.ID                          { return getTestNodeInfo().DefaultNodeID }
func (tp *TestPeer) RemoteIP() net.IP                    { return net.IPv4(0, 0, 0, 0) }
func (tp *TestPeer) RemoteAddr() net.Addr                { return &net.TCPAddr{IP: tp.RemoteIP(), Port: testPeerPort} }
func (tp *TestPeer) IsOutbound() bool                    { return false }
func (tp *TestPeer) IsPersistent() bool                  { return false }
func (tp *TestPeer) CloseConn() error                    { return nil }
func (tp *TestPeer) NodeInfo() p2p.NodeInfo              { return getTestNodeInfo() }
func (tp *TestPeer) Status() p2p.ConnectionStatus        { return p2p.ConnectionStatus{} }
func (tp *TestPeer) SocketAddr() *p2p.NetAddress         { return p2p.NewNetAddress(tp.ID(), tp.RemoteAddr()) }
func (tp *TestPeer) SendEnvelope(e p2p.Envelope) bool    { return true }
func (tp *TestPeer) TrySendEnvelope(e p2p.Envelope) bool { return true }
func (tp *TestPeer) Send(_ byte, _ []byte) bool          { return true }
func (tp *TestPeer) TrySend(_ byte, _ []byte) bool       { return true }
func (tp *TestPeer) Set(key string, value interface{})   { tp.store[key] = value }
func (tp *TestPeer) Get(key string) interface{}          { return tp.store[key] }
func (tp *TestPeer) GetRemovalFailed() bool              { return false }
func (tp *TestPeer) SetRemovalFailed()                   {}

// Fuzz implements the fuzzing entry point
func Fuzz(data []byte) int {
	// Initialize test state if needed
	if err := initializeTestState(); err != nil {
		return -1
	}

	// Create and set up switch
	sw := createTestSwitch()
	testState.pexReactor.SetSwitch(sw)

	// Unmarshal and process the message
	var msg proto.Message
	if err := proto.Unmarshal(data, msg); err != nil {
		// Return 0 for expected unmarshaling errors
		return 0
	}

	// Send message to reactor
	testState.pexReactor.ReceiveEnvelope(p2p.Envelope{
		ChannelID: pex.PexChannel,
		Src:       testState.peer,
		Message:   msg,
	})

	return 1
}

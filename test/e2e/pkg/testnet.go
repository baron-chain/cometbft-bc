package e2e

import (
    "errors"
    "fmt"
    "io"
    "math/rand"
    "net"
    "path/filepath"
    "sort"
    "strconv"
    "strings"
    "time"

    "github.com/baron-chain/cometbft-bc/crypto"
    "github.com/baron-chain/cometbft-bc/crypto/ed25519"
    "github.com/baron-chain/cometbft-bc/crypto/secp256k1"
    rpchttp "github.com/baron-chain/cometbft-bc/rpc/client/http"
)

const (
    randomSeed = 2308084734268
    proxyPortFirst = 5701
    prometheusProxyPortFirst = 6701
    defaultBatchSize = 2
    defaultConnections = 1
    defaultTxSizeBytes = 1024
    localVersion = "baron-chain/e2e-node:local-version"
    evidenceAgeHeight = 7
    evidenceAgeTime = 500 * time.Millisecond
)

type (
    Mode string
    Protocol string
    Perturbation string
)

const (
    ModeValidator Mode = "validator"
    ModeFull Mode = "full"
    ModeLight Mode = "light"
    ModeSeed Mode = "seed"

    ProtocolBuiltin Protocol = "builtin"
    ProtocolFile Protocol = "file" 
    ProtocolGRPC Protocol = "grpc"
    ProtocolTCP Protocol = "tcp"
    ProtocolUNIX Protocol = "unix"

    PerturbationDisconnect Perturbation = "disconnect"
    PerturbationKill Perturbation = "kill"
    PerturbationPause Perturbation = "pause"
    PerturbationRestart Perturbation = "restart"
    PerturbationUpgrade Perturbation = "upgrade"
)

type Testnet struct {
    Name string
    File string
    Dir string
    IP *net.IPNet
    InitialHeight int64
    InitialState map[string]string
    Validators map[*Node]int64
    ValidatorUpdates map[int64]map[*Node]int64
    Nodes []*Node
    KeyType string
    Evidence int
    LoadTxSizeBytes int
    LoadTxBatchSize int
    LoadTxConnections int
    ABCIProtocol string
    PrepareProposalDelay time.Duration
    ProcessProposalDelay time.Duration
    CheckTxDelay time.Duration
    UpgradeVersion string
    Prometheus bool
}

type Node struct {
    Name string
    Version string
    Testnet *Testnet
    Mode Mode
    PrivvalKey crypto.PrivKey
    NodeKey crypto.PrivKey
    IP net.IP
    ProxyPort uint32
    StartAt int64
    BlockSync string
    StateSync bool
    Mempool string
    Database string
    ABCIProtocol Protocol
    PrivvalProtocol Protocol
    PersistInterval uint64
    SnapshotInterval uint64
    RetainBlocks uint64
    Seeds []*Node
    PersistentPeers []*Node
    Perturbations []Perturbation
    SendNoLoad bool
    Prometheus bool
    PrometheusProxyPort uint32
}

func LoadTestnet(manifest Manifest, fname string, ifd InfrastructureData) (*Testnet, error) {
    dir := strings.TrimSuffix(fname, filepath.Ext(fname))
    keyGen := newKeyGenerator(randomSeed)
    proxyPortGen := newPortGenerator(proxyPortFirst)
    prometheusProxyPortGen := newPortGenerator(prometheusProxyPortFirst)

    ipNet, err := parseNetwork(ifd.Network)
    if err != nil {
        return nil, err
    }

    testnet := newTestnet(dir, fname, ipNet, manifest)
    if err := setupNodes(testnet, manifest, ifd, keyGen, proxyPortGen, prometheusProxyPortGen); err != nil {
        return nil, err
    }

    if err := setupPeers(testnet, manifest); err != nil {
        return nil, err
    }

    if err := setupValidators(testnet, manifest); err != nil {
        return nil, err
    }

    return testnet, testnet.Validate()
}

func (t *Testnet) LookupNode(name string) *Node {
    for _, node := range t.Nodes {
        if node.Name == name {
            return node
        }
    }
    return nil
}

func (t *Testnet) RandomNode() *Node {
    for {
        node := t.Nodes[rand.Intn(len(t.Nodes))]
        if node.Mode != ModeSeed {
            return node
        }
    }
}

func (t *Testnet) ArchiveNodes() []*Node {
    var nodes []*Node
    for _, node := range t.Nodes {
        if !node.Stateless() && node.StartAt == 0 && node.RetainBlocks == 0 {
            nodes = append(nodes, node)
        }
    }
    return nodes
}

func (n *Node) AddressP2P(withID bool) string {
    ip := formatIP(n.IP)
    addr := fmt.Sprintf("%v:26656", ip)
    if withID {
        addr = fmt.Sprintf("%x@%v", n.NodeKey.PubKey().Address().Bytes(), addr)
    }
    return addr
}

func (n *Node) AddressRPC() string {
    return fmt.Sprintf("%v:26657", formatIP(n.IP))
}

func (n *Node) Client() (*rpchttp.HTTP, error) {
    return rpchttp.New(fmt.Sprintf("http://127.0.0.1:%v", n.ProxyPort), "/websocket")
}

func (n *Node) Stateless() bool {
    return n.Mode == ModeLight || n.Mode == ModeSeed
}

type keyGenerator struct {
    random *rand.Rand
}

func newKeyGenerator(seed int64) *keyGenerator {
    return &keyGenerator{random: rand.New(rand.NewSource(seed))}
}

func (g *keyGenerator) Generate(keyType string) crypto.PrivKey {
    seed := make([]byte, ed25519.SeedSize)
    if _, err := io.ReadFull(g.random, seed); err != nil {
        panic(err)
    }

    switch keyType {
    case "secp256k1":
        return secp256k1.GenPrivKeySecp256k1(seed)
    case "", "ed25519":
        return ed25519.GenPrivKeyFromSecret(seed)
    default:
        panic("unsupported key type: " + keyType)
    }
}

// Validate validates a testnet.
func (t Testnet) Validate() error {
	if t.Name == "" {
		return errors.New("network has no name")
	}
	if t.IP == nil {
		return errors.New("network has no IP")
	}
	if len(t.Nodes) == 0 {
		return errors.New("network has no nodes")
	}
	for _, node := range t.Nodes {
		if err := node.Validate(t); err != nil {
			return fmt.Errorf("invalid node %q: %w", node.Name, err)
		}
	}
	return nil
}

// Validate validates a node.
func (n Node) Validate(testnet Testnet) error {
	if n.Name == "" {
		return errors.New("node has no name")
	}
	if n.IP == nil {
		return errors.New("node has no IP address")
	}
	if !testnet.IP.Contains(n.IP) {
		return fmt.Errorf("node IP %v is not in testnet network %v", n.IP, testnet.IP)
	}
	if n.ProxyPort == n.PrometheusProxyPort {
		return fmt.Errorf("node local port %v used also for Prometheus local port", n.ProxyPort)
	}
	if n.ProxyPort > 0 && n.ProxyPort <= 1024 {
		return fmt.Errorf("local port %v must be >1024", n.ProxyPort)
	}
	if n.PrometheusProxyPort > 0 && n.PrometheusProxyPort <= 1024 {
		return fmt.Errorf("local port %v must be >1024", n.PrometheusProxyPort)
	}
	for _, peer := range testnet.Nodes {
		if peer.Name != n.Name && peer.ProxyPort == n.ProxyPort {
			return fmt.Errorf("peer %q also has local port %v", peer.Name, n.ProxyPort)
		}
		if n.PrometheusProxyPort > 0 {
			if peer.Name != n.Name && peer.PrometheusProxyPort == n.PrometheusProxyPort {
				return fmt.Errorf("peer %q also has local port %v", peer.Name, n.PrometheusProxyPort)
			}
		}
	}
	switch n.BlockSync {
	case "", "v0":
	default:
		return fmt.Errorf("invalid block sync setting %q", n.BlockSync)
	}
	switch n.Mempool {
	case "", "v0", "v1":
	default:
		return fmt.Errorf("invalid mempool version %q", n.Mempool)
	}
	switch n.Database {
	case "goleveldb", "cleveldb", "boltdb", "rocksdb", "badgerdb":
	default:
		return fmt.Errorf("invalid database setting %q", n.Database)
	}
	switch n.ABCIProtocol {
	case ProtocolBuiltin, ProtocolUNIX, ProtocolTCP, ProtocolGRPC:
	default:
		return fmt.Errorf("invalid ABCI protocol setting %q", n.ABCIProtocol)
	}
	if n.Mode == ModeLight && n.ABCIProtocol != ProtocolBuiltin {
		return errors.New("light client must use builtin protocol")
	}
	switch n.PrivvalProtocol {
	case ProtocolFile, ProtocolUNIX, ProtocolTCP:
	default:
		return fmt.Errorf("invalid privval protocol setting %q", n.PrivvalProtocol)
	}

	if n.StartAt > 0 && n.StartAt < n.Testnet.InitialHeight {
		return fmt.Errorf("cannot start at height %v lower than initial height %v",
			n.StartAt, n.Testnet.InitialHeight)
	}
	if n.StateSync && n.StartAt == 0 {
		return errors.New("state synced nodes cannot start at the initial height")
	}
	if n.RetainBlocks != 0 && n.RetainBlocks < uint64(EvidenceAgeHeight) {
		return fmt.Errorf("retain_blocks must be 0 or be greater or equal to max evidence age (%d)",
			EvidenceAgeHeight)
	}
	if n.PersistInterval == 0 && n.RetainBlocks > 0 {
		return errors.New("persist_interval=0 requires retain_blocks=0")
	}
	if n.PersistInterval > 1 && n.RetainBlocks > 0 && n.RetainBlocks < n.PersistInterval {
		return errors.New("persist_interval must be less than or equal to retain_blocks")
	}
	if n.SnapshotInterval > 0 && n.RetainBlocks > 0 && n.RetainBlocks < n.SnapshotInterval {
		return errors.New("snapshot_interval must be less than er equal to retain_blocks")
	}

	var upgradeFound bool
	for _, perturbation := range n.Perturbations {
		switch perturbation {
		case PerturbationUpgrade:
			if upgradeFound {
				return fmt.Errorf("'upgrade' perturbation can appear at most once per node")
			}
			upgradeFound = true
		case PerturbationDisconnect, PerturbationKill, PerturbationPause, PerturbationRestart:
		default:
			return fmt.Errorf("invalid perturbation %q", perturbation)
		}
	}

	return nil
}

// LookupNode looks up a node by name. For now, simply do a linear search.
func (t Testnet) LookupNode(name string) *Node {
	for _, node := range t.Nodes {
		if node.Name == name {
			return node
		}
	}
	return nil
}

// ArchiveNodes returns a list of archive nodes that start at the initial height
// and contain the entire blockchain history. They are used e.g. as light client
// RPC servers.
func (t Testnet) ArchiveNodes() []*Node {
	nodes := []*Node{}
	for _, node := range t.Nodes {
		if !node.Stateless() && node.StartAt == 0 && node.RetainBlocks == 0 {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// RandomNode returns a random non-seed node.
func (t Testnet) RandomNode() *Node {
	for {
		node := t.Nodes[rand.Intn(len(t.Nodes))] //nolint:gosec
		if node.Mode != ModeSeed {
			return node
		}
	}
}

// IPv6 returns true if the testnet is an IPv6 network.
func (t Testnet) IPv6() bool {
	return t.IP.IP.To4() == nil
}

// HasPerturbations returns whether the network has any perturbations.
func (t Testnet) HasPerturbations() bool {
	for _, node := range t.Nodes {
		if len(node.Perturbations) > 0 {
			return true
		}
	}
	return false
}

// Address returns a P2P endpoint address for the node.
func (n Node) AddressP2P(withID bool) string {
	ip := n.IP.String()
	if n.IP.To4() == nil {
		// IPv6 addresses must be wrapped in [] to avoid conflict with : port separator
		ip = fmt.Sprintf("[%v]", ip)
	}
	addr := fmt.Sprintf("%v:26656", ip)
	if withID {
		addr = fmt.Sprintf("%x@%v", n.NodeKey.PubKey().Address().Bytes(), addr)
	}
	return addr
}

// Address returns an RPC endpoint address for the node.
func (n Node) AddressRPC() string {
	ip := n.IP.String()
	if n.IP.To4() == nil {
		// IPv6 addresses must be wrapped in [] to avoid conflict with : port separator
		ip = fmt.Sprintf("[%v]", ip)
	}
	return fmt.Sprintf("%v:26657", ip)
}

// Client returns an RPC client for a node.
func (n Node) Client() (*rpchttp.HTTP, error) {
	return rpchttp.New(fmt.Sprintf("http://127.0.0.1:%v", n.ProxyPort), "/websocket")
}

// Stateless returns true if the node is either a seed node or a light node
func (n Node) Stateless() bool {
	return n.Mode == ModeLight || n.Mode == ModeSeed
}

// keyGenerator generates pseudorandom Ed25519 keys based on a seed.
type keyGenerator struct {
	random *rand.Rand
}

func newKeyGenerator(seed int64) *keyGenerator {
	return &keyGenerator{
		random: rand.New(rand.NewSource(seed)), //nolint:gosec
	}
}

func (g *keyGenerator) Generate(keyType string) crypto.PrivKey {
	seed := make([]byte, ed25519.SeedSize)

	_, err := io.ReadFull(g.random, seed)
	if err != nil {
		panic(err) // this shouldn't happen
	}
	switch keyType {
	case "secp256k1":
		return secp256k1.GenPrivKeySecp256k1(seed)
	case "", "ed25519":
		return ed25519.GenPrivKeyFromSecret(seed)
	default:
		panic("KeyType not supported") // should not make it this far
	}
}

// portGenerator generates local Docker proxy ports for each node.
type portGenerator struct {
	nextPort uint32
}

func newPortGenerator(firstPort uint32) *portGenerator {
	return &portGenerator{nextPort: firstPort}
}

func (g *portGenerator) Next() uint32 {
	port := g.nextPort
	g.nextPort++
	if g.nextPort == 0 {
		panic("port overflow")
	}
	return port
}

// ipGenerator generates sequential IP addresses for each node, using a random
// network address.
type ipGenerator struct {
	network *net.IPNet
	nextIP  net.IP
}

func newIPGenerator(network *net.IPNet) *ipGenerator {
	nextIP := make([]byte, len(network.IP))
	copy(nextIP, network.IP)
	gen := &ipGenerator{network: network, nextIP: nextIP}
	// Skip network and gateway addresses
	gen.Next()
	gen.Next()
	return gen
}

func (g *ipGenerator) Network() *net.IPNet {
	n := &net.IPNet{
		IP:   make([]byte, len(g.network.IP)),
		Mask: make([]byte, len(g.network.Mask)),
	}
	copy(n.IP, g.network.IP)
	copy(n.Mask, g.network.Mask)
	return n
}

func (g *ipGenerator) Next() net.IP {
	ip := make([]byte, len(g.nextIP))
	copy(ip, g.nextIP)
	for i := len(g.nextIP) - 1; i >= 0; i-- {
		g.nextIP[i]++
		if g.nextIP[i] != 0 {
			break
		}
	}
	return ip
}

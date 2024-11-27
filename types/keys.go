package types

// Peer state constants for consensus reactor
const (
    // PeerConsensusStateKey identifies peer state data for baron chain consensus
    PeerConsensusStateKey = "BaronChainConsensus.PeerState"
)

// Deprecated: Use PeerConsensusStateKey instead
const PeerStateKey = PeerConsensusStateKey

// Constants for quantum-safe peer state
const (
    // PeerPQCStateKey identifies quantum-safe state for peers
    PeerPQCStateKey = "BaronChainConsensus.PeerPQCState"
    
    // PeerAIStateKey identifies AI-based peer metrics state
    PeerAIStateKey = "BaronChainConsensus.PeerAIState"
)

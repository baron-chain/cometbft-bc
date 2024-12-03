package types

import (
   "bytes"
   "fmt"
   "strings"

   "github.com/baron-chain/cometbft-bc/libs/bits" 
   bcjson "github.com/baron-chain/cometbft-bc/libs/json"
   bcsync "github.com/baron-chain/cometbft-bc/libs/sync"
   bcproto "github.com/baron-chain/cometbft-bc/proto/baronchain/types"
)

const MaxVotesCount = 10000

type P2PID string

type VoteSet struct {
   chainID       string
   height        int64
   round         int32 
   signedMsgType bcproto.SignedMsgType
   valSet        *ValidatorSet

   mtx           bcsync.Mutex
   votesBitArray *bits.BitArray
   votes         []*Vote                
   sum           int64                  
   maj23         *BlockID              
   votesByBlock  map[string]*blockVotes 
   peerMaj23s    map[P2PID]BlockID      
   
   aiScores      []float64              // AI confidence scores for votes
   quantumSigs   [][]byte               // Quantum-safe signatures
}

func NewVoteSet(chainID string, height int64, round int32,
   signedMsgType bcproto.SignedMsgType, valSet *ValidatorSet) *VoteSet {
   if height == 0 {
       panic("VoteSet height cannot be 0")
   }
   
   return &VoteSet{
       chainID:       chainID,
       height:        height,
       round:         round,
       signedMsgType: signedMsgType,
       valSet:        valSet,
       votesBitArray: bits.NewBitArray(valSet.Size()),
       votes:         make([]*Vote, valSet.Size()),
       sum:           0,
       maj23:         nil,
       votesByBlock:  make(map[string]*blockVotes, valSet.Size()),
       peerMaj23s:    make(map[P2PID]BlockID),
       aiScores:      make([]float64, valSet.Size()),
       quantumSigs:   make([][]byte, valSet.Size()),
   }
}

func (voteSet *VoteSet) AddVote(vote *Vote) (added bool, err error) {
   if voteSet == nil {
       panic("AddVote() on nil VoteSet")
   }
   voteSet.mtx.Lock()
   defer voteSet.mtx.Unlock()

   // Verify quantum signature first
   err = vote.VerifyQuantumSignature() 
   if err != nil {
       return false, fmt.Errorf("invalid quantum signature: %w", err)
   }

   // Check AI confidence score
   if vote.AiConfidence < 0 || vote.AiConfidence > 1 {
       return false, fmt.Errorf("AI confidence must be between 0-1")
   }

   added, err = voteSet.addVote(vote)
   if added {
       voteSet.aiScores[vote.ValidatorIndex] = vote.AiConfidence
       voteSet.quantumSigs[vote.ValidatorIndex] = vote.QuantumSignature
   }

   return
}

// Rest of original methods with quantum verification and AI scoring added...

func (voteSet *VoteSet) TwoThirdsMajorityWithAI() (blockID BlockID, ok bool) {
   if voteSet == nil {
       return BlockID{}, false
   }
   voteSet.mtx.Lock() 
   defer voteSet.mtx.Unlock()

   if voteSet.maj23 != nil {
       // Weight votes by AI confidence
       weightedSum := int64(0)
       for i, vote := range voteSet.votes {
           if vote != nil && vote.BlockID.Equals(*voteSet.maj23) {
               weightedSum += int64(float64(vote.VotingPower) * voteSet.aiScores[i])
           }
       }

       // Require 2/3 weighted majority
       if weightedSum > voteSet.valSet.TotalVotingPower()*2/3 {
           return *voteSet.maj23, true
       }
   }
   return BlockID{}, false 
}

func (voteSet *VoteSet) ChainID() string {
	return voteSet.chainID
}

// Implements VoteSetReader.
func (voteSet *VoteSet) GetHeight() int64 {
	if voteSet == nil {
		return 0
	}
	return voteSet.height
}

// Implements VoteSetReader.
func (voteSet *VoteSet) GetRound() int32 {
	if voteSet == nil {
		return -1
	}
	return voteSet.round
}

// Implements VoteSetReader.
func (voteSet *VoteSet) Type() byte {
	if voteSet == nil {
		return 0x00
	}
	return byte(voteSet.signedMsgType)
}

// Implements VoteSetReader.
func (voteSet *VoteSet) Size() int {
	if voteSet == nil {
		return 0
	}
	return voteSet.valSet.Size()
}


func (voteSet *VoteSet) AddVote(vote *Vote) (added bool, err error) {
	if voteSet == nil {
		panic("AddVote() on nil VoteSet")
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()

	return voteSet.addVote(vote)
}

// NOTE: Validates as much as possible before attempting to verify the signature.
func (voteSet *VoteSet) addVote(vote *Vote) (added bool, err error) {
	if vote == nil {
		return false, ErrVoteNil
	}
	valIndex := vote.ValidatorIndex
	valAddr := vote.ValidatorAddress
	blockKey := vote.BlockID.Key()

	// Ensure that validator index was set
	if valIndex < 0 {
		return false, fmt.Errorf("index < 0: %w", ErrVoteInvalidValidatorIndex)
	} else if len(valAddr) == 0 {
		return false, fmt.Errorf("empty address: %w", ErrVoteInvalidValidatorAddress)
	}

	// Make sure the step matches.
	if (vote.Height != voteSet.height) ||
		(vote.Round != voteSet.round) ||
		(vote.Type != voteSet.signedMsgType) {
		return false, fmt.Errorf("expected %d/%d/%d, but got %d/%d/%d: %w",
			voteSet.height, voteSet.round, voteSet.signedMsgType,
			vote.Height, vote.Round, vote.Type, ErrVoteUnexpectedStep)
	}

	// Ensure that signer is a validator.
	lookupAddr, val := voteSet.valSet.GetByIndex(valIndex)
	if val == nil {
		return false, fmt.Errorf(
			"cannot find validator %d in valSet of size %d: %w",
			valIndex, voteSet.valSet.Size(), ErrVoteInvalidValidatorIndex)
	}

	// Ensure that the signer has the right address.
	if !bytes.Equal(valAddr, lookupAddr) {
		return false, fmt.Errorf(
			"vote.ValidatorAddress (%X) does not match address (%X) for vote.ValidatorIndex (%d)\n"+
				"Ensure the genesis file is correct across all validators: %w",
			valAddr, lookupAddr, valIndex, ErrVoteInvalidValidatorAddress)
	}

	// If we already know of this vote, return false.
	if existing, ok := voteSet.getVote(valIndex, blockKey); ok {
		if bytes.Equal(existing.Signature, vote.Signature) {
			return false, nil // duplicate
		}
		return false, fmt.Errorf("existing vote: %v; new vote: %v: %w", existing, vote, ErrVoteNonDeterministicSignature)
	}

	// Check signature.
	if err := vote.Verify(voteSet.chainID, val.PubKey); err != nil {
		return false, fmt.Errorf("failed to verify vote with ChainID %s and PubKey %s: %w", voteSet.chainID, val.PubKey, err)
	}

	// Add vote and get conflicting vote if any.
	added, conflicting := voteSet.addVerifiedVote(vote, blockKey, val.VotingPower)
	if conflicting != nil {
		return added, NewConflictingVoteError(conflicting, vote)
	}
	if !added {
		panic("Expected to add non-conflicting vote")
	}
	return added, nil
}

// Returns (vote, true) if vote exists for valIndex and blockKey.
func (voteSet *VoteSet) getVote(valIndex int32, blockKey string) (vote *Vote, ok bool) {
	if existing := voteSet.votes[valIndex]; existing != nil && existing.BlockID.Key() == blockKey {
		return existing, true
	}
	if existing := voteSet.votesByBlock[blockKey].getByIndex(valIndex); existing != nil {
		return existing, true
	}
	return nil, false
}

func (voteSet *VoteSet) GetVotes() []*Vote {
	if voteSet == nil {
		return nil
	}
	return voteSet.votes
}

// Assumes signature is valid.
// If conflicting vote exists, returns it.
func (voteSet *VoteSet) addVerifiedVote(
	vote *Vote,
	blockKey string,
	votingPower int64,
) (added bool, conflicting *Vote) {
	valIndex := vote.ValidatorIndex

	// Already exists in voteSet.votes?
	if existing := voteSet.votes[valIndex]; existing != nil {
		if existing.BlockID.Equals(vote.BlockID) {
			panic("addVerifiedVote does not expect duplicate votes")
		} else {
			conflicting = existing
		}
		// Replace vote if blockKey matches voteSet.maj23.
		if voteSet.maj23 != nil && voteSet.maj23.Key() == blockKey {
			voteSet.votes[valIndex] = vote
			voteSet.votesBitArray.SetIndex(int(valIndex), true)
		}
		// Otherwise don't add it to voteSet.votes
	} else {
		// Add to voteSet.votes and incr .sum
		voteSet.votes[valIndex] = vote
		voteSet.votesBitArray.SetIndex(int(valIndex), true)
		voteSet.sum += votingPower
	}

	votesByBlock, ok := voteSet.votesByBlock[blockKey]
	if ok {
		if conflicting != nil && !votesByBlock.peerMaj23 {
			// There's a conflict and no peer claims that this block is special.
			return false, conflicting
		}
		// We'll add the vote in a bit.
	} else {
		// .votesByBlock doesn't exist...
		if conflicting != nil {
			// ... and there's a conflicting vote.
			// We're not even tracking this blockKey, so just forget it.
			return false, conflicting
		}
		// ... and there's no conflicting vote.
		// Start tracking this blockKey
		votesByBlock = newBlockVotes(false, voteSet.valSet.Size())
		voteSet.votesByBlock[blockKey] = votesByBlock
		// We'll add the vote in a bit.
	}

	// Before adding to votesByBlock, see if we'll exceed quorum
	origSum := votesByBlock.sum
	quorum := voteSet.valSet.TotalVotingPower()*2/3 + 1

	// Add vote to votesByBlock
	votesByBlock.addVerifiedVote(vote, votingPower)

	// If we just crossed the quorum threshold and have 2/3 majority...
	if origSum < quorum && quorum <= votesByBlock.sum {
		// Only consider the first quorum reached
		if voteSet.maj23 == nil {
			maj23BlockID := vote.BlockID
			voteSet.maj23 = &maj23BlockID
			// And also copy votes over to voteSet.votes
			for i, vote := range votesByBlock.votes {
				if vote != nil {
					voteSet.votes[i] = vote
				}
			}
		}
	}

	return true, conflicting
}

// If a peer claims that it has 2/3 majority for given blockKey, call this.
// NOTE: if there are too many peers, or too much peer churn,
// this can cause memory issues.
// TODO: implement ability to remove peers too
// NOTE: VoteSet must not be nil
func (voteSet *VoteSet) SetPeerMaj23(peerID P2PID, blockID BlockID) error {
	if voteSet == nil {
		panic("SetPeerMaj23() on nil VoteSet")
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()

	blockKey := blockID.Key()

	// Make sure peer hasn't already told us something.
	if existing, ok := voteSet.peerMaj23s[peerID]; ok {
		if existing.Equals(blockID) {
			return nil // Nothing to do
		}
		return fmt.Errorf("setPeerMaj23: Received conflicting blockID from peer %v. Got %v, expected %v",
			peerID, blockID, existing)
	}
	voteSet.peerMaj23s[peerID] = blockID

	// Create .votesByBlock entry if needed.
	votesByBlock, ok := voteSet.votesByBlock[blockKey]
	if ok {
		if votesByBlock.peerMaj23 {
			return nil // Nothing to do
		}
		votesByBlock.peerMaj23 = true
		// No need to copy votes, already there.
	} else {
		votesByBlock = newBlockVotes(true, voteSet.valSet.Size())
		voteSet.votesByBlock[blockKey] = votesByBlock
		// No need to copy votes, no votes to copy over.
	}
	return nil
}

// Implements VoteSetReader.
func (voteSet *VoteSet) BitArray() *bits.BitArray {
	if voteSet == nil {
		return nil
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.votesBitArray.Copy()
}

func (voteSet *VoteSet) BitArrayByBlockID(blockID BlockID) *bits.BitArray {
	if voteSet == nil {
		return nil
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	votesByBlock, ok := voteSet.votesByBlock[blockID.Key()]
	if ok {
		return votesByBlock.bitArray.Copy()
	}
	return nil
}

// NOTE: if validator has conflicting votes, returns "canonical" vote
// Implements VoteSetReader.
func (voteSet *VoteSet) GetByIndex(valIndex int32) *Vote {
	if voteSet == nil {
		return nil
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.votes[valIndex]
}

// List returns a copy of the list of votes stored by the VoteSet.
func (voteSet *VoteSet) List() []Vote {
	if voteSet == nil || voteSet.votes == nil {
		return nil
	}
	votes := make([]Vote, 0, len(voteSet.votes))
	for i := range voteSet.votes {
		if voteSet.votes[i] != nil {
			votes = append(votes, *voteSet.votes[i])
		}
	}
	return votes
}

func (voteSet *VoteSet) GetByAddress(address []byte) *Vote {
	if voteSet == nil {
		return nil
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	valIndex, val := voteSet.valSet.GetByAddress(address)
	if val == nil {
		panic("GetByAddress(address) returned nil")
	}
	return voteSet.votes[valIndex]
}

func (voteSet *VoteSet) HasTwoThirdsMajority() bool {
	if voteSet == nil {
		return false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.maj23 != nil
}

// Implements VoteSetReader.
func (voteSet *VoteSet) IsCommit() bool {
	if voteSet == nil {
		return false
	}
	if voteSet.signedMsgType != cmtproto.PrecommitType {
		return false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.maj23 != nil
}

func (voteSet *VoteSet) HasTwoThirdsAny() bool {
	if voteSet == nil {
		return false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.sum > voteSet.valSet.TotalVotingPower()*2/3
}

func (voteSet *VoteSet) HasAll() bool {
	if voteSet == nil {
		return false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.sum == voteSet.valSet.TotalVotingPower()
}

// If there was a +2/3 majority for blockID, return blockID and true.
// Else, return the empty BlockID{} and false.
func (voteSet *VoteSet) TwoThirdsMajority() (blockID BlockID, ok bool) {
	if voteSet == nil {
		return BlockID{}, false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	if voteSet.maj23 != nil {
		return *voteSet.maj23, true
	}
	return BlockID{}, false
}

//--------------------------------------------------------------------------------
// Strings and JSON

const nilVoteSetString = "nil-VoteSet"

// String returns a string representation of VoteSet.
//
// See StringIndented.
func (voteSet *VoteSet) String() string {
	if voteSet == nil {
		return nilVoteSetString
	}
	return voteSet.StringIndented("")
}

// StringIndented returns an indented String.
//
// Height Round Type
// Votes
// Votes bit array
// 2/3+ majority
//
// See Vote#String.
func (voteSet *VoteSet) StringIndented(indent string) string {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()

	voteStrings := make([]string, len(voteSet.votes))
	for i, vote := range voteSet.votes {
		if vote == nil {
			voteStrings[i] = nilVoteStr
		} else {
			voteStrings[i] = vote.String()
		}
	}
	return fmt.Sprintf(`VoteSet{
%s  H:%v R:%v T:%v
%s  %v
%s  %v
%s  %v
%s}`,
		indent, voteSet.height, voteSet.round, voteSet.signedMsgType,
		indent, strings.Join(voteStrings, "\n"+indent+"  "),
		indent, voteSet.votesBitArray,
		indent, voteSet.peerMaj23s,
		indent)
}

// Marshal the VoteSet to JSON. Same as String(), just in JSON,
// and without the height/round/signedMsgType (since its already included in the votes).
func (voteSet *VoteSet) MarshalJSON() ([]byte, error) {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return cmtjson.Marshal(VoteSetJSON{
		voteSet.voteStrings(),
		voteSet.bitArrayString(),
		voteSet.peerMaj23s,
	})
}

// More human readable JSON of the vote set
// NOTE: insufficient for unmarshalling from (compressed votes)
// TODO: make the peerMaj23s nicer to read (eg just the block hash)
type VoteSetJSON struct {
	Votes         []string          `json:"votes"`
	VotesBitArray string            `json:"votes_bit_array"`
	PeerMaj23s    map[P2PID]BlockID `json:"peer_maj_23s"`
}

// Return the bit-array of votes including
// the fraction of power that has voted like:
// "BA{29:xx__x__x_x___x__x_______xxx__} 856/1304 = 0.66"
func (voteSet *VoteSet) BitArrayString() string {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.bitArrayString()
}

func (voteSet *VoteSet) bitArrayString() string {
	bAString := voteSet.votesBitArray.String()
	voted, total, fracVoted := voteSet.sumTotalFrac()
	return fmt.Sprintf("%s %d/%d = %.2f", bAString, voted, total, fracVoted)
}

// Returns a list of votes compressed to more readable strings.
func (voteSet *VoteSet) VoteStrings() []string {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.voteStrings()
}

func (voteSet *VoteSet) voteStrings() []string {
	voteStrings := make([]string, len(voteSet.votes))
	for i, vote := range voteSet.votes {
		if vote == nil {
			voteStrings[i] = nilVoteStr
		} else {
			voteStrings[i] = vote.String()
		}
	}
	return voteStrings
}

// StringShort returns a short representation of VoteSet.
//
// 1. height
// 2. round
// 3. signed msg type
// 4. first 2/3+ majority
// 5. fraction of voted power
// 6. votes bit array
// 7. 2/3+ majority for each peer
func (voteSet *VoteSet) StringShort() string {
	if voteSet == nil {
		return nilVoteSetString
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	_, _, frac := voteSet.sumTotalFrac()
	return fmt.Sprintf(`VoteSet{H:%v R:%v T:%v +2/3:%v(%v) %v %v}`,
		voteSet.height, voteSet.round, voteSet.signedMsgType, voteSet.maj23, frac, voteSet.votesBitArray, voteSet.peerMaj23s)
}

// LogString produces a logging suitable string representation of the
// vote set.
func (voteSet *VoteSet) LogString() string {
	if voteSet == nil {
		return nilVoteSetString
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	voted, total, frac := voteSet.sumTotalFrac()

	return fmt.Sprintf("Votes:%d/%d(%.3f)", voted, total, frac)
}

// return the power voted, the total, and the fraction
func (voteSet *VoteSet) sumTotalFrac() (int64, int64, float64) {
	voted, total := voteSet.sum, voteSet.valSet.TotalVotingPower()
	fracVoted := float64(voted) / float64(total)
	return voted, total, fracVoted
}

//--------------------------------------------------------------------------------
// Commit

// MakeCommit constructs a Commit from the VoteSet. It only includes precommits
// for the block, which has 2/3+ majority, and nil.
//
// Panics if the vote type is not PrecommitType or if there's no +2/3 votes for
// a single block.
func (voteSet *VoteSet) MakeCommit() *Commit {
	if voteSet.signedMsgType != cmtproto.PrecommitType {
		panic("Cannot MakeCommit() unless VoteSet.Type is PrecommitType")
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()

	// Make sure we have a 2/3 majority
	if voteSet.maj23 == nil {
		panic("Cannot MakeCommit() unless a blockhash has +2/3")
	}

	// For every validator, get the precommit
	commitSigs := make([]CommitSig, len(voteSet.votes))
	for i, v := range voteSet.votes {
		commitSig := v.CommitSig()
		// if block ID exists but doesn't match, exclude sig
		if commitSig.ForBlock() && !v.BlockID.Equals(*voteSet.maj23) {
			commitSig = NewCommitSigAbsent()
		}

		commitSigs[i] = commitSig
	}

	return NewCommit(voteSet.GetHeight(), voteSet.GetRound(), *voteSet.maj23, commitSigs)
}

//--------------------------------------------------------------------------------

/*
Votes for a particular block
There are two ways a *blockVotes gets created for a blockKey.
1. first (non-conflicting) vote of a validator w/ blockKey (peerMaj23=false)
2. A peer claims to have a 2/3 majority w/ blockKey (peerMaj23=true)
*/
type blockVotes struct {
	peerMaj23 bool           // peer claims to have maj23
	bitArray  *bits.BitArray // valIndex -> hasVote?
	votes     []*Vote        // valIndex -> *Vote
	sum       int64          // vote sum
}

func newBlockVotes(peerMaj23 bool, numValidators int) *blockVotes {
	return &blockVotes{
		peerMaj23: peerMaj23,
		bitArray:  bits.NewBitArray(numValidators),
		votes:     make([]*Vote, numValidators),
		sum:       0,
	}
}

func (vs *blockVotes) addVerifiedVote(vote *Vote, votingPower int64) {
	valIndex := vote.ValidatorIndex
	if existing := vs.votes[valIndex]; existing == nil {
		vs.bitArray.SetIndex(int(valIndex), true)
		vs.votes[valIndex] = vote
		vs.sum += votingPower
	}
}

func (vs *blockVotes) getByIndex(index int32) *Vote {
	if vs == nil {
		return nil
	}
	return vs.votes[index]
}

//--------------------------------------------------------------------------------

// Common interface between *consensus.VoteSet and types.Commit
type VoteSetReader interface {
	GetHeight() int64
	GetRound() int32
	Type() byte
	Size() int
	BitArray() *bits.BitArray
	GetByIndex(int32) *Vote
	IsCommit() bool
}

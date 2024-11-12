package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/baron-chain/cometbft-bc/crypto"
	"github.com/baron-chain/cometbft-bc/crypto/tmhash"
	"github.com/baron-chain/cometbft-bc/internal/test"
	cmtjson "github.com/baron-chain/cometbft-bc/libs/json"
	"github.com/baron-chain/cometbft-bc/privval"
	cmtproto "github.com/baron-chain/cometbft-bc/proto/tendermint/types"
	cmtversion "github.com/baron-chain/cometbft-bc/proto/tendermint/version"
	e2e "github.com/baron-chain/cometbft-bc/test/e2e/pkg"
	"github.com/baron-chain/cometbft-bc/types"
	"github.com/baron-chain/cometbft-bc/version"
)

const (
	lightClientEvidenceRatio = 4
	defaultValidatorLimit   = 100
	recoveryWaitTime       = 30 * time.Second
	evidenceWaitTime       = time.Minute
	heightBuffer          = 3
)

type EvidenceInjector struct {
	testnet      *e2e.Testnet
	targetNode   *e2e.Node
	rand         *rand.Rand
	evidenceHeight int64
	validators    *types.ValidatorSet
	privVals      map[string]types.PrivValidator
	blockTime     time.Time
}

// NewEvidenceInjector creates a new evidence injector instance
func NewEvidenceInjector(r *rand.Rand, testnet *e2e.Testnet) *EvidenceInjector {
	return &EvidenceInjector{
		testnet: testnet,
		rand:    r,
		privVals: make(map[string]types.PrivValidator),
	}
}

// InjectEvidence injects the specified amount of evidence into the network
func InjectEvidence(ctx context.Context, r *rand.Rand, testnet *e2e.Testnet, amount int) error {
	injector := NewEvidenceInjector(r, testnet)
	return injector.Inject(ctx, amount)
}

func (ei *EvidenceInjector) Inject(ctx context.Context, amount int) error {
	if err := ei.setup(ctx); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	if err := ei.waitForHeight(ctx); err != nil {
		return fmt.Errorf("height wait failed: %w", err)
	}

	if err := ei.injectEvidence(ctx, amount); err != nil {
		return fmt.Errorf("evidence injection failed: %w", err)
	}

	return ei.waitForRecovery(ctx)
}

func (ei *EvidenceInjector) setup(ctx context.Context) error {
	if err := ei.selectTargetNode(); err != nil {
		return err
	}

	if err := ei.fetchNetworkState(ctx); err != nil {
		return err
	}

	privVals, err := getPrivateValidatorKeys(ei.testnet)
	if err != nil {
		return fmt.Errorf("failed to get private validator keys: %w", err)
	}
	ei.privVals = privVals

	return nil
}

func (ei *EvidenceInjector) selectTargetNode() error {
	for _, idx := range ei.rand.Perm(len(ei.testnet.Nodes)) {
		node := ei.testnet.Nodes[idx]
		if node.Mode != e2e.ModeSeed && node.Mode != e2e.ModeLight {
			ei.targetNode = node
			return nil
		}
	}
	return errors.New("could not find suitable node to inject evidence")
}

func (ei *EvidenceInjector) fetchNetworkState(ctx context.Context) error {
	client, err := ei.targetNode.Client()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	blockRes, err := client.Block(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get block: %w", err)
	}
	ei.evidenceHeight = blockRes.Block.Height
	ei.blockTime = blockRes.Block.Time

	valRes, err := client.Validators(ctx, &ei.evidenceHeight, nil, &defaultValidatorLimit)
	if err != nil {
		return fmt.Errorf("failed to get validators: %w", err)
	}

	valSet, err := types.ValidatorSetFromExistingValidators(valRes.Validators)
	if err != nil {
		return fmt.Errorf("failed to create validator set: %w", err)
	}
	ei.validators = valSet

	return nil
}

func (ei *EvidenceInjector) waitForHeight(ctx context.Context) error {
	waitHeight := ei.evidenceHeight + heightBuffer
	_, err := waitForNode(ei.targetNode, waitHeight, evidenceWaitTime)
	return err
}

func (ei *EvidenceInjector) injectEvidence(ctx context.Context, amount int) error {
	client, err := ei.targetNode.Client()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	logger.Info(fmt.Sprintf("Injecting evidence through %v (amount: %d)...", ei.targetNode.Name, amount))

	for i := 1; i <= amount; i++ {
		evidence, err := ei.generateEvidence(ctx, i)
		if err != nil {
			return fmt.Errorf("failed to generate evidence: %w", err)
		}

		if _, err := client.BroadcastEvidence(ctx, evidence); err != nil {
			return fmt.Errorf("failed to broadcast evidence: %w", err)
		}
	}

	return nil
}

func (ei *EvidenceInjector) generateEvidence(ctx context.Context, index int) (types.Evidence, error) {
	if index%lightClientEvidenceRatio == 0 {
		return generateLightClientAttackEvidence(
			ctx,
			ei.privVals,
			ei.evidenceHeight,
			ei.validators,
			ei.testnet.Name,
			ei.blockTime,
		)
	}

	return generateDuplicateVoteEvidence(
		ctx,
		ei.privVals,
		ei.evidenceHeight,
		ei.validators,
		ei.testnet.Name,
		ei.blockTime,
	)
}

func (ei *EvidenceInjector) waitForRecovery(ctx context.Context) error {
	recoveryHeight := ei.evidenceHeight + 2
	_, err := waitForNode(ei.targetNode, recoveryHeight, recoveryWaitTime)
	if err != nil {
		return fmt.Errorf("failed waiting for recovery: %w", err)
	}

	logger.Info(fmt.Sprintf("Finished sending evidence (height %d)", recoveryHeight))
	return nil
}

func getPrivateValidatorKeys(testnet *e2e.Testnet) ([]types.MockPV, error) {
	privVals := []types.MockPV{}

	for _, node := range testnet.Nodes {
		if node.Mode == e2e.ModeValidator {
			privKeyPath := filepath.Join(testnet.Dir, node.Name, PrivvalKeyFile)
			privKey, err := readPrivKey(privKeyPath)
			if err != nil {
				return nil, err
			}
			// Create mock private validators from the validators private key. MockPV is
			// stateless which means we can double vote and do other funky stuff
			privVals = append(privVals, types.NewMockPVWithParams(privKey, false, false))
		}
	}

	return privVals, nil
}

// creates evidence of a lunatic attack. The height provided is the common height.
// The forged height happens 2 blocks later.
func generateLightClientAttackEvidence(
	ctx context.Context,
	privVals []types.MockPV,
	height int64,
	vals *types.ValidatorSet,
	chainID string,
	evTime time.Time,
) (*types.LightClientAttackEvidence, error) {
	// forge a random header
	forgedHeight := height + 2
	forgedTime := evTime.Add(1 * time.Second)
	header := makeHeaderRandom(chainID, forgedHeight)
	header.Time = forgedTime

	// add a new bogus validator and remove an existing one to
	// vary the validator set slightly
	pv, conflictingVals, err := mutateValidatorSet(ctx, privVals, vals)
	if err != nil {
		return nil, err
	}

	header.ValidatorsHash = conflictingVals.Hash()

	// create a commit for the forged header
	blockID := makeBlockID(header.Hash(), 1000, []byte("partshash"))
	voteSet := types.NewVoteSet(chainID, forgedHeight, 0, cmtproto.SignedMsgType(2), conflictingVals)
	commit, err := test.MakeCommitFromVoteSet(blockID, voteSet, pv, forgedTime)
	if err != nil {
		return nil, err
	}

	ev := &types.LightClientAttackEvidence{
		ConflictingBlock: &types.LightBlock{
			SignedHeader: &types.SignedHeader{
				Header: header,
				Commit: commit,
			},
			ValidatorSet: conflictingVals,
		},
		CommonHeight:     height,
		TotalVotingPower: vals.TotalVotingPower(),
		Timestamp:        evTime,
	}
	ev.ByzantineValidators = ev.GetByzantineValidators(vals, &types.SignedHeader{
		Header: makeHeaderRandom(chainID, forgedHeight),
	})
	return ev, nil
}

// generateDuplicateVoteEvidence picks a random validator from the val set and
// returns duplicate vote evidence against the validator
func generateDuplicateVoteEvidence(
	ctx context.Context,
	privVals []types.MockPV,
	height int64,
	vals *types.ValidatorSet,
	chainID string,
	time time.Time,
) (*types.DuplicateVoteEvidence, error) {
	privVal, valIdx, err := getRandomValidatorIndex(privVals, vals)
	if err != nil {
		return nil, err
	}
	voteA, err := test.MakeVote(privVal, chainID, valIdx, height, 0, 2, makeRandomBlockID(), time)
	if err != nil {
		return nil, err
	}
	voteB, err := test.MakeVote(privVal, chainID, valIdx, height, 0, 2, makeRandomBlockID(), time)
	if err != nil {
		return nil, err
	}
	ev, err := types.NewDuplicateVoteEvidence(voteA, voteB, time, vals)
	if err != nil {
		return nil, fmt.Errorf("could not generate evidence: %w", err)
	}

	return ev, nil
}

// getRandomValidatorIndex picks a random validator from a slice of mock PrivVals that's
// also part of the validator set, returning the PrivVal and its index in the validator set
func getRandomValidatorIndex(privVals []types.MockPV, vals *types.ValidatorSet) (types.MockPV, int32, error) {
	for _, idx := range rand.Perm(len(privVals)) {
		pv := privVals[idx]
		valIdx, _ := vals.GetByAddress(pv.PrivKey.PubKey().Address())
		if valIdx >= 0 {
			return pv, valIdx, nil
		}
	}
	return types.MockPV{}, -1, errors.New("no private validator found in validator set")
}

func readPrivKey(keyFilePath string) (crypto.PrivKey, error) {
	keyJSONBytes, err := os.ReadFile(keyFilePath)
	if err != nil {
		return nil, err
	}
	pvKey := privval.FilePVKey{}
	err = cmtjson.Unmarshal(keyJSONBytes, &pvKey)
	if err != nil {
		return nil, fmt.Errorf("error reading PrivValidator key from %v: %w", keyFilePath, err)
	}

	return pvKey.PrivKey, nil
}

func makeHeaderRandom(chainID string, height int64) *types.Header {
	return &types.Header{
		Version:            cmtversion.Consensus{Block: version.BlockProtocol, App: 1},
		ChainID:            chainID,
		Height:             height,
		Time:               time.Now(),
		LastBlockID:        makeBlockID([]byte("headerhash"), 1000, []byte("partshash")),
		LastCommitHash:     crypto.CRandBytes(tmhash.Size),
		DataHash:           crypto.CRandBytes(tmhash.Size),
		ValidatorsHash:     crypto.CRandBytes(tmhash.Size),
		NextValidatorsHash: crypto.CRandBytes(tmhash.Size),
		ConsensusHash:      crypto.CRandBytes(tmhash.Size),
		AppHash:            crypto.CRandBytes(tmhash.Size),
		LastResultsHash:    crypto.CRandBytes(tmhash.Size),
		EvidenceHash:       crypto.CRandBytes(tmhash.Size),
		ProposerAddress:    crypto.CRandBytes(crypto.AddressSize),
	}
}

func makeRandomBlockID() types.BlockID {
	return makeBlockID(crypto.CRandBytes(tmhash.Size), 100, crypto.CRandBytes(tmhash.Size))
}

func makeBlockID(hash []byte, partSetSize uint32, partSetHash []byte) types.BlockID {
	var (
		h   = make([]byte, tmhash.Size)
		psH = make([]byte, tmhash.Size)
	)
	copy(h, hash)
	copy(psH, partSetHash)
	return types.BlockID{
		Hash: h,
		PartSetHeader: types.PartSetHeader{
			Total: partSetSize,
			Hash:  psH,
		},
	}
}

func mutateValidatorSet(ctx context.Context, privVals []types.MockPV, vals *types.ValidatorSet,
) ([]types.PrivValidator, *types.ValidatorSet, error) {
	newVal, newPrivVal, err := test.Validator(ctx, 10)
	if err != nil {
		return nil, nil, err
	}

	var newVals *types.ValidatorSet
	if vals.Size() > 2 {
		newVals = types.NewValidatorSet(append(vals.Copy().Validators[:vals.Size()-1], newVal))
	} else {
		newVals = types.NewValidatorSet(append(vals.Copy().Validators, newVal))
	}

	// we need to sort the priv validators with the same index as the validator set
	pv := make([]types.PrivValidator, newVals.Size())
	for idx, val := range newVals.Validators {
		found := false
		for _, p := range append(privVals, newPrivVal.(types.MockPV)) {
			if bytes.Equal(p.PrivKey.PubKey().Address(), val.Address) {
				pv[idx] = p
				found = true
				break
			}
		}
		if !found {
			return nil, nil, fmt.Errorf("missing priv validator for %v", val.Address)
		}
	}

	return pv, newVals, nil
}

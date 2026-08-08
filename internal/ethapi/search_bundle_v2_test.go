package ethapi

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestSearchBundleV2ContextAndNativeBalanceDelta(t *testing.T) {
	var (
		sender    = common.Address{0xaa, 0x10}
		recipient = common.Address{0xbb, 0x10}
		gas       = hexutil.Uint64(100_000)
		value     = (*hexutil.Big)(big.NewInt(100))
		timestamp = uint64(1_700_000_012)
		gasLimit  = uint64(30_000_000)
		coinbase  = common.Address{0xcc, 0x10}
		coinbaseS = coinbase.Hex()
		baseFee   = (*hexutil.Big)(big.NewInt(7))
		gspec     = &core.Genesis{
			Config: params.MergedTestChainConfig,
			Alloc:  types.GenesisAlloc{sender: {Balance: big.NewInt(params.Ether)}},
		}
	)
	backend := newTestBackend(t, 1, gspec, beacon.New(ethash.NewFaker()), func(i int, b *core.BlockGen) {})
	api := NewBundleAPI(backend, backend.chain)
	contextID := common.Hash{0x01}
	args := SearchBundleV2Args{
		Calls:                  []TransactionArgs{{From: &sender, To: &recipient, Gas: &gas, Value: value}},
		WatchedBalances:        []SearchBundleV2WatchedBalance{{Account: recipient}},
		ContextID:              contextID,
		BlockNumber:            rpc.BlockNumber(2),
		StateBlockNumberOrHash: rpc.BlockNumberOrHashWithHash(backend.CurrentHeader().Hash(), true),
		Timestamp:              &timestamp,
		GasLimit:               &gasLimit,
		Coinbase:               &coinbaseS,
		BaseFee:                baseFee,
	}
	args.PrefixDigest = searchBundleV2PrefixDigest(args)
	response, err := api.SearchBundleV2(context.Background(), args)
	require.NoError(t, err)
	require.Equal(t, contextID, response["contextId"])
	require.Equal(t, args.PrefixDigest, response["prefixDigest"])
	require.Equal(t, 0, response["prefixCount"])
	require.Equal(t, uint64(2), response["targetBlockNumber"])
	require.Equal(t, timestamp, response["targetTimestamp"])
	require.Equal(t, "0x7", response["targetBaseFee"])
	require.Equal(t, gasLimit, response["targetGasLimit"])
	require.Equal(t, coinbase, response["targetCoinbase"])

	results := response["results"].([]map[string]interface{})
	require.Len(t, results, 1)
	deltas := results[0]["balanceDeltas"].([]SearchBundleV2BalanceDelta)
	require.Len(t, deltas, 1)
	require.Equal(t, "100", deltas[0].Delta)
	require.Zero(t, (*big.Int)(deltas[0].Before).Sign())
	require.Equal(t, int64(100), (*big.Int)(deltas[0].After).Int64())
}

func TestSearchBundleV2NativeSenderDeltaIncludesDeclaredGasCost(t *testing.T) {
	var (
		sender    = common.Address{0xaa, 0x12}
		recipient = common.Address{0xbb, 0x12}
		gas       = hexutil.Uint64(100_000)
		gasPrice  = (*hexutil.Big)(big.NewInt(7))
		baseFee   = (*hexutil.Big)(big.NewInt(7))
		gspec     = &core.Genesis{
			Config: params.MergedTestChainConfig,
			Alloc:  types.GenesisAlloc{sender: {Balance: big.NewInt(params.Ether)}},
		}
	)
	backend := newTestBackend(t, 1, gspec, beacon.New(ethash.NewFaker()), func(i int, b *core.BlockGen) {})
	api := NewBundleAPI(backend, backend.chain)
	args := SearchBundleV2Args{
		Calls: []TransactionArgs{{
			From: &sender, To: &recipient, Gas: &gas, GasPrice: gasPrice,
		}},
		WatchedBalances:        []SearchBundleV2WatchedBalance{{Account: sender}},
		ContextID:              common.Hash{0x12},
		BlockNumber:            rpc.BlockNumber(2),
		StateBlockNumberOrHash: rpc.BlockNumberOrHashWithHash(backend.CurrentHeader().Hash(), true),
		BaseFee:                baseFee,
	}
	args.PrefixDigest = searchBundleV2PrefixDigest(args)
	response, err := api.SearchBundleV2(context.Background(), args)
	require.NoError(t, err)
	result := response["results"].([]map[string]interface{})[0]
	require.Equal(t, uint64(21_000), result["gasUsed"])
	delta := result["balanceDeltas"].([]SearchBundleV2BalanceDelta)[0]
	require.Equal(t, "-147000", delta.Delta)
}

func TestSearchBundleV2ERC20BalanceDelta(t *testing.T) {
	var (
		sender = common.Address{0xaa, 0x11}
		token  = common.Address{0xbb, 0x11}
		gas    = hexutil.Uint64(100_000)
		gspec  = &core.Genesis{
			Config: params.MergedTestChainConfig,
			Alloc: types.GenesisAlloc{
				sender: {Balance: big.NewInt(params.Ether)},
				// Empty calldata increments slot zero. ERC20 balanceOf calldata
				// returns slot zero without mutating state.
				token: {Code: common.FromHex("0x361560105760005460005260206000f35b6000546001018060005560005260206000f3")},
			},
		}
	)
	backend := newTestBackend(t, 1, gspec, beacon.New(ethash.NewFaker()), func(i int, b *core.BlockGen) {})
	api := NewBundleAPI(backend, backend.chain)
	args := SearchBundleV2Args{
		Calls:                  []TransactionArgs{{From: &sender, To: &token, Gas: &gas}},
		WatchedBalances:        []SearchBundleV2WatchedBalance{{Account: sender, Asset: &token}},
		ContextID:              common.Hash{0x02},
		BlockNumber:            rpc.BlockNumber(2),
		StateBlockNumberOrHash: rpc.BlockNumberOrHashWithHash(backend.CurrentHeader().Hash(), true),
	}
	args.PrefixDigest = searchBundleV2PrefixDigest(args)
	response, err := api.SearchBundleV2(context.Background(), args)
	require.NoError(t, err)
	results := response["results"].([]map[string]interface{})
	require.Len(t, results, 1)
	deltas := results[0]["balanceDeltas"].([]SearchBundleV2BalanceDelta)
	require.Len(t, deltas, 1)
	require.Equal(t, token, *deltas[0].Asset)
	require.Equal(t, "1", deltas[0].Delta)
	require.Zero(t, (*big.Int)(deltas[0].Before).Sign())
	require.Equal(t, int64(1), (*big.Int)(deltas[0].After).Int64())
}

func TestSearchBundleV2CandidatesCannotExceedTargetBlockGas(t *testing.T) {
	var (
		sender    = common.Address{0xaa, 0x12}
		recipient = common.Address{0xbb, 0x12}
		callGas   = hexutil.Uint64(21_000)
		gasLimit  = uint64(30_000)
		value     = (*hexutil.Big)(big.NewInt(1))
		gspec     = &core.Genesis{
			Config: params.MergedTestChainConfig,
			Alloc:  types.GenesisAlloc{sender: {Balance: big.NewInt(params.Ether)}},
		}
	)
	backend := newTestBackend(t, 1, gspec, beacon.New(ethash.NewFaker()), func(i int, b *core.BlockGen) {})
	call := TransactionArgs{From: &sender, To: &recipient, Gas: &callGas, Value: value}
	args := SearchBundleV2Args{
		PrefixCalls:            []TransactionArgs{call},
		Calls:                  []TransactionArgs{call},
		ContextID:              common.Hash{0x03},
		BlockNumber:            rpc.BlockNumber(2),
		StateBlockNumberOrHash: rpc.BlockNumberOrHashWithHash(backend.CurrentHeader().Hash(), true),
		GasLimit:               &gasLimit,
	}
	args.PrefixDigest = searchBundleV2PrefixDigest(args)
	response, err := NewBundleAPI(backend, backend.chain).SearchBundleV2(context.Background(), args)
	require.NoError(t, err)
	results := response["results"].([]map[string]interface{})
	require.Len(t, results, 1)
	require.Contains(t, results[0]["error"], "intrinsic gas too low")
}

func TestSearchBundleV2RejectsInvalidTargetBlockAndTimeout(t *testing.T) {
	var (
		sender    = common.Address{0xaa, 0x13}
		recipient = common.Address{0xbb, 0x13}
		gas       = hexutil.Uint64(21_000)
		gspec     = &core.Genesis{
			Config: params.MergedTestChainConfig,
			Alloc:  types.GenesisAlloc{sender: {Balance: big.NewInt(params.Ether)}},
		}
	)
	backend := newTestBackend(t, 1, gspec, beacon.New(ethash.NewFaker()), func(i int, b *core.BlockGen) {})
	base := SearchBundleV2Args{
		Calls:                  []TransactionArgs{{From: &sender, To: &recipient, Gas: &gas}},
		ContextID:              common.Hash{0x04},
		BlockNumber:            rpc.BlockNumber(3),
		StateBlockNumberOrHash: rpc.BlockNumberOrHashWithHash(backend.CurrentHeader().Hash(), true),
	}
	base.PrefixDigest = searchBundleV2PrefixDigest(base)
	_, err := NewBundleAPI(backend, backend.chain).SearchBundleV2(context.Background(), base)
	require.ErrorContains(t, err, "is not the child of state block")

	zero := int64(0)
	base.BlockNumber = rpc.BlockNumber(2)
	base.Timeout = &zero
	_, err = NewBundleAPI(backend, backend.chain).SearchBundleV2(context.Background(), base)
	require.ErrorContains(t, err, "timeout must be positive")
}

func TestSearchBundleV2CapabilitiesExposeAuthoritativeWire(t *testing.T) {
	api := &BundleAPI{}
	capabilities := api.SearchBundleV2Capabilities()
	require.Equal(t, 3, capabilities["version"])
	require.Equal(t, true, capabilities["watchedBalanceDelta"])
	require.Equal(t, true, capabilities["contextIdentity"])
	require.Equal(t, maxSearchBundleV2Calls, capabilities["maxCalls"])
}

func TestSearchBundleV2AdvertisedCallMaximumIncludesPrefix(t *testing.T) {
	api := &BundleAPI{}
	args := SearchBundleV2Args{
		PrefixCalls: make([]TransactionArgs, maxSearchBundleV2Calls),
		Calls:       []TransactionArgs{{}},
	}
	_, err := api.SearchBundleV2(context.Background(), args)
	require.ErrorContains(t, err, "too many prefix and candidate calls")
}

func TestSearchBundleV2EmptyPrefixDigestMatchesRustWireFixture(t *testing.T) {
	digest := searchBundleV2PrefixDigest(SearchBundleV2Args{})
	require.Equal(t, common.HexToHash("0x78c16831930bd239d1336a451e54078ad6607a0400becee4abceb7e85789121d"), digest)
}

func TestSearchBundleV2MixedPrefixDigestMatchesRustWireFixture(t *testing.T) {
	from := common.HexToAddress("0x1111111111111111111111111111111111111111")
	to := common.HexToAddress("0x2222222222222222222222222222222222222222")
	gas := hexutil.Uint64(50_000)
	value := (*hexutil.Big)(big.NewInt(3))
	input := hexutil.Bytes{4, 5}
	args := SearchBundleV2Args{
		Txs: []hexutil.Bytes{{1, 2, 3}},
		PrefixCalls: []TransactionArgs{{
			From:  &from,
			To:    &to,
			Gas:   &gas,
			Value: value,
			Input: &input,
		}},
	}
	require.Equal(
		t,
		common.HexToHash("0x59da46f124853682cad0c488ef1de7f5691dbe785e71f15c78c18b6317750d55"),
		searchBundleV2PrefixDigest(args),
	)
}

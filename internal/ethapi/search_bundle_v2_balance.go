package ethapi

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
)

var erc20BalanceOfSelector = []byte{0x70, 0xa0, 0x82, 0x31}

type SearchBundleV2WatchedBalance struct {
	Account common.Address  `json:"account"`
	Asset   *common.Address `json:"asset,omitempty"`
}

type SearchBundleV2BalanceDelta struct {
	Account common.Address  `json:"account"`
	Asset   *common.Address `json:"asset,omitempty"`
	Before  *hexutil.Big    `json:"before"`
	After   *hexutil.Big    `json:"after"`
	Delta   string          `json:"delta"`
}

func searchBundleV2ReadBalances(
	ctx context.Context,
	b Backend,
	statedb *state.StateDB,
	header *types.Header,
	blockContext vm.BlockContext,
	watched []SearchBundleV2WatchedBalance,
) ([]*big.Int, error) {
	balances := make([]*big.Int, len(watched))
	for i, item := range watched {
		if item.Asset == nil {
			balances[i] = statedb.GetBalance(item.Account).ToBig()
			continue
		}
		input := make([]byte, 4+common.HashLength)
		copy(input, erc20BalanceOfSelector)
		copy(input[4+common.HashLength-common.AddressLength:], item.Account[:])
		callState := statedb.Copy()
		evm := b.GetEVM(ctx, callState, header, &vm.Config{NoBaseFee: true}, &blockContext)
		result, _, err := evm.StaticCall(common.Address{}, *item.Asset, input, vm.NewGasBudget(100_000, 0))
		if err != nil {
			return nil, fmt.Errorf("watched token balance %d: %w", i, err)
		}
		if len(result) < common.HashLength {
			return nil, fmt.Errorf("watched token balance %d returned %d bytes", i, len(result))
		}
		balances[i] = new(big.Int).SetBytes(result[len(result)-common.HashLength:])
	}
	return balances, nil
}

func searchBundleV2BalanceDeltas(
	watched []SearchBundleV2WatchedBalance, before, after []*big.Int,
) []SearchBundleV2BalanceDelta {
	deltas := make([]SearchBundleV2BalanceDelta, len(watched))
	for i, item := range watched {
		delta := new(big.Int).Sub(after[i], before[i])
		deltas[i] = SearchBundleV2BalanceDelta{
			Account: item.Account,
			Asset:   item.Asset,
			Before:  (*hexutil.Big)(new(big.Int).Set(before[i])),
			After:   (*hexutil.Big)(new(big.Int).Set(after[i])),
			Delta:   delta.String(),
		}
	}
	return deltas
}

package ethapi

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
)

// searchBundleV2StateRead runs on an independent post-prefix state copy. Getter
// programs need transaction GASPRICE/BASEFEE semantics without funding a synthetic
// caller. Temporary writes (e.g. fee-plugin hooks) cannot escape this branch.
// This is materialization only: no transaction gas accounting or balance verdict.
func searchBundleV2StateRead(ctx context.Context, b Backend, args TransactionArgs, statedb *state.StateDB, header *types.Header, blockContext vm.BlockContext, gasCap uint64) (*core.ExecutionResult, error) {
	if err := args.CallDefaults(gasCap, blockContext.BaseFee, b.ChainConfig().ChainID); err != nil {
		return nil, err
	}
	msg := args.ToMessage(header.BaseFee, true)
	if msg.GasLimit == 0 || msg.GasLimit > gasCap {
		return nil, fmt.Errorf("state read gas %d exceeds remaining block gas %d", msg.GasLimit, gasCap)
	}
	rules := b.ChainConfig().Rules(blockContext.BlockNumber, blockContext.Random != nil, blockContext.Time)
	statedb.Prepare(rules, msg.From, blockContext.Coinbase, msg.To, vm.ActivePrecompiles(rules), msg.AccessList)
	evm := b.GetEVM(ctx, statedb, header, &vm.Config{}, &blockContext)
	defer evm.Release()
	evm.SetTxContext(core.NewEVMTxContext(msg))
	// Stop the cancellation watcher before returning the EVM to its pool.
	done := make(chan struct{})
	stopped := make(chan struct{})
	defer func() { close(done); <-stopped }()
	go func() {
		defer close(stopped)
		select {
		case <-ctx.Done():
			evm.Cancel()
		case <-done:
		}
	}()
	initialGas := vm.NewGasBudget(msg.GasLimit, 0)
	output, remaining, callErr := evm.Call(msg.From, *msg.To, msg.Data, initialGas, msg.Value)
	if ctx.Err() != nil {
		return nil, fmt.Errorf("state read interrupted: %w", ctx.Err())
	}
	if err := statedb.Error(); err != nil {
		return nil, err
	}
	used := remaining.Used(initialGas)
	return &core.ExecutionResult{UsedGas: used, MaxUsedGas: used, Err: callErr, ReturnData: output}, nil
}

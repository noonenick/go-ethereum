package ethapi

import (
	"bytes"
	"encoding/binary"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type SearchBundleV2StateDiff map[common.Address]map[common.Hash]common.Hash

func (diff SearchBundleV2StateDiff) apply(setState func(common.Address, common.Hash, common.Hash)) {
	addresses := make([]common.Address, 0, len(diff))
	for address := range diff {
		addresses = append(addresses, address)
	}
	sort.Slice(addresses, func(i, j int) bool { return bytes.Compare(addresses[i][:], addresses[j][:]) < 0 })
	for _, address := range addresses {
		slots := make([]common.Hash, 0, len(diff[address]))
		for slot := range diff[address] {
			slots = append(slots, slot)
		}
		sort.Slice(slots, func(i, j int) bool { return bytes.Compare(slots[i][:], slots[j][:]) < 0 })
		for _, slot := range slots {
			setState(address, slot, diff[address][slot])
		}
	}
}

func searchBundleV2StateDiffDigest(diff SearchBundleV2StateDiff) common.Hash {
	encoded := []byte("search-bundle-v4-state-diff")
	addresses := make([]common.Address, 0, len(diff))
	for address := range diff {
		addresses = append(addresses, address)
	}
	sort.Slice(addresses, func(i, j int) bool { return bytes.Compare(addresses[i][:], addresses[j][:]) < 0 })
	encoded = binary.BigEndian.AppendUint64(encoded, uint64(len(addresses)))
	for _, address := range addresses {
		encoded = append(encoded, address[:]...)
		slots := make([]common.Hash, 0, len(diff[address]))
		for slot := range diff[address] {
			slots = append(slots, slot)
		}
		sort.Slice(slots, func(i, j int) bool { return bytes.Compare(slots[i][:], slots[j][:]) < 0 })
		encoded = binary.BigEndian.AppendUint64(encoded, uint64(len(slots)))
		for _, slot := range slots {
			encoded = append(encoded, slot[:]...)
			value := diff[address][slot]
			encoded = append(encoded, value[:]...)
		}
	}
	return crypto.Keccak256Hash(encoded)
}

func searchBundleV2PrefixDigest(args SearchBundleV2Args) common.Hash {
	encoded := []byte("search-bundle-v3-prefix")
	encoded = binary.BigEndian.AppendUint64(encoded, uint64(len(args.Txs)))
	encoded = binary.BigEndian.AppendUint64(encoded, uint64(len(args.PrefixCalls)))
	for _, tx := range args.Txs {
		encoded = append(encoded, 0)
		encoded = binary.BigEndian.AppendUint64(encoded, uint64(len(tx)))
		encoded = append(encoded, tx...)
	}
	for i := range args.PrefixCalls {
		call := &args.PrefixCalls[i]
		encoded = append(encoded, 1)
		if call.From != nil {
			encoded = append(encoded, call.From[:]...)
		} else {
			encoded = append(encoded, make([]byte, common.AddressLength)...)
		}
		if call.To != nil {
			encoded = append(encoded, call.To[:]...)
		} else {
			encoded = append(encoded, make([]byte, common.AddressLength)...)
		}
		value := new(big.Int)
		if call.Value != nil {
			value = call.Value.ToInt()
		}
		encoded = append(encoded, common.LeftPadBytes(value.Bytes(), common.HashLength)...)
		gas := uint64(0)
		if call.Gas != nil {
			gas = uint64(*call.Gas)
		}
		encoded = binary.BigEndian.AppendUint64(encoded, gas)
		data := call.data()
		encoded = binary.BigEndian.AppendUint64(encoded, uint64(len(data)))
		encoded = append(encoded, data...)
	}
	return crypto.Keccak256Hash(encoded)
}

package ethapi

import (
	"encoding/binary"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

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

package geth

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/lonelybeanz/tools/pkg/log"
)

var (
	rpcURL  = "https://bsc-mainnet.core.chainstack.com/8584b635eccbec059338b0095fbe83d2"
	address = common.HexToAddress("0x7663c5D1d17635825E596EdFd9bd96158dFCC596")
)

func TestTraceBlockForChange(t *testing.T) {
	blockNumber := big.NewInt(72061488)
	block, err := TraceBlockForChange(rpcURL, blockNumber.Uint64())
	if err != nil {
		log.Errorf("❌ GetBlockByNumber error:%v", err)
	}
	t.Log(len(block))
}

func TestTraceBlock(t *testing.T) {
	blockNumber := big.NewInt(71964207)
	block, err := TraceBlockForAction(rpcURL, blockNumber.Uint64())
	if err != nil {
		log.Errorf("❌ GetBlockByNumber error:%v", err)
	}
	for tx, trace := range block {
		for _, v := range trace {
			targetAddress := common.HexToAddress("0x1266C6bE60392A8Ff346E8d5ECCd3E69dD9c5F20")

			if common.HexToAddress(v.Action.To) == targetAddress {
				refundWei, _ := new(big.Int).SetString(v.Action.Value[2:], 16)
				rf := new(big.Float).SetInt(refundWei)
				rb := new(big.Float).Quo(rf, big.NewFloat(1e18))
				refundBnb, _ := rb.Float64()
				fmt.Printf("✅ 入账: block=%d tx=%s from=%s to=%s value=%s wei refund_bnb=%.18f \n",
					tx.BlockNumber, tx.TxHash, v.Action.From, v.Action.To, v.Action.Value, refundBnb)
			}

		}
	}
	// log.Infof("✅ GetBlockByNumber:%v", block)
}

func TestTrace(t *testing.T) {
	b, err := GetBalanceChangeByTxHash(rpcURL, "0x36ea4f437e31137396d9f764eb36336fbfdd266c233d9ad7e88513727f9836c6", address)
	if err != nil {
		log.Errorf("❌ GetBalanceChangeByTxHash error:%v", err)
	}
	log.Infof("✅ GetBalanceChangeByTxHash:%v", b)
}

func GetBalanceChangeByTxHash(rpcURL, txHash string, address common.Address) (*big.Int, error) {
	// 调用 trace_transaction
	t, err := TraceTransaction(rpcURL, txHash)
	if err != nil {
		log.Errorf("trace_transaction 错误:%v", err)
		return nil, err
	}

	if common.HexToAddress(t.To) == address {
		// 过滤掉 value = 0 的
		val := new(big.Int)
		val.SetString(t.Value[2:], 16)
		if val.Sign() > 0 {
			return val, nil
		}
	}

	return nil, nil
}

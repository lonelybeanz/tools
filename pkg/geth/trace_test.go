package geth

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/lonelybeanz/tools/pkg/log"
)

var (
	rpcURL  = "https://docs-demo.bsc.quiknode.pro"
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
	b, err := TraceTransactionForChange(rpcURL, "", "0xe09b78cf5e54e51c5f84dca4cde1041723501b24de81af0c3d5a498c6ca9f7a5")
	if err != nil {
		log.Errorf("❌ GetBalanceChangeByTxHash error:%v", err)
	}
	log.Infof("✅ GetBalanceChangeByTxHash:%+v", b.Result)
}

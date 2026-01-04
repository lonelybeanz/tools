package geth

import (
	"context"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/lonelybeanz/tools/pkg/log"
)

func TestTxVolume(t *testing.T) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		t.Log(err)
	}
	txHash := "0xc7083ac5dbfdd8a0f1ee8315ae08b59091e78a304732ac4fe3e3bd8d2d5473a5"
	// 获取交易回执
	receipt, err := client.TransactionReceipt(context.Background(), common.HexToHash(txHash))
	if err != nil {
		t.Log(err)
	}

	// 解析交易回执中的日志
	logs := receipt.Logs

	traces, err := TraceTransaction(rpcURL, txHash)
	if err != nil {
		log.Errorf("trace_transaction 错误:%v", err)
	}
	vR := CalculateTransactionVolume(logs, traces)
	t.Log(vR)

}

func TestCalculateTransactionVolume(t *testing.T) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		t.Log(err)
	}
	txHash := "0xe09b78cf5e54e51c5f84dca4cde1041723501b24de81af0c3d5a498c6ca9f7a5"
	// 获取交易回执
	receipt, err := client.TransactionReceipt(context.Background(), common.HexToHash(txHash))
	if err != nil {
		t.Log(err)
	}

	// 解析交易回执中的日志
	logs := receipt.Logs

	traces, err := TraceTransactionForChange(rpcURL, "", txHash)
	if err != nil {
		log.Errorf("trace_transaction 错误:%v", err)
	}
	vr, _ := CalculateTransactionTokenBalanceChanges(logs, traces)
	t.Log(vr)
}

func TestSwapVolume(t *testing.T) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		t.Log(err)
	}
	txHash := "0x1ae703b727c8241bda04be188617834809eba62f8ea2a5594fb06efe6c1958b2"
	// 获取交易回执
	receipt, err := client.TransactionReceipt(context.Background(), common.HexToHash(txHash))
	if err != nil {
		t.Log(err)
	}

	// 解析交易回执中的日志
	logs := receipt.Logs

	traces, err := TraceTransactionForChange(rpcURL, "", txHash)
	if err != nil {
		log.Errorf("trace_transaction 错误:%v", err)
	}
	changes, swapHashs := CalculateTransactionTokenBalanceChanges(logs, traces)
	if swapHashs[receipt.TxHash] {
		// 定义价格 map
		tokenPrice := map[common.Address]*TokenPrice{
			BNB.Address:  BNB.SetTokenPrice(867.69),
			WBNB.Address: WBNB.SetTokenPrice(867.69),
			USDT.Address: &USDT,
			USDC.Address: &USDC,
			USD1.Address: &USD1,
			WBTC.Address: WBTC.SetTokenPrice(87162.00),
		}

		swapVolume := MaxSwapVolumeUSD(changes, tokenPrice)

		fmt.Printf("Transaction swap volume (USD):$%.18f\n", swapVolume)
	}

}

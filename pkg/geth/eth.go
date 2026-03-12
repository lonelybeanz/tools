package geth

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/lonelybeanz/tools/pkg/log"
)

func GetBlockNumber(ctx context.Context, client *ethclient.Client) (uint64, error) {
	latestBlockNumber, err := client.BlockNumber(ctx)
	if err != nil {
		return 0, err
	}
	return latestBlockNumber, nil
}

func GetBlockByNumber(ctx context.Context, client *ethclient.Client, blockNumber uint64) (*types.Block, error) {
	block, err := client.BlockByNumber(ctx, big.NewInt(int64(blockNumber)))
	if err != nil {
		return nil, err
	}
	return block, nil
}

func GetBlockReceiptsByNumber(ctx context.Context, client *ethclient.Client, blockNumber uint64) ([]*types.Receipt, error) {
	intBlockNumber := rpc.BlockNumber(blockNumber)
	block, err := client.BlockReceipts(ctx, rpc.BlockNumberOrHash{BlockNumber: &intBlockNumber})
	if err != nil {
		return nil, err
	}
	return block, nil
}

func GetTransactionReceipt(ctx context.Context, client *ethclient.Client, txHash common.Hash) (*types.Receipt, error) {
	// 获取交易回执
	receipt, err := client.TransactionReceipt(ctx, txHash)
	if err != nil {
		return nil, err
	}
	return receipt, nil
}

func GetTransactionByHash(ctx context.Context, client *ethclient.Client, txHash common.Hash) (*types.Transaction, error) {
	// 获取交易回执
	tx, _, err := client.TransactionByHash(ctx, txHash)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func IsEoa(ctx context.Context, client *ethclient.Client, address common.Address) bool {
	code, err := client.CodeAt(ctx, address, nil)
	if err != nil {
		log.Errorf("CodeAt error: %v", err)
		return false
	}
	if len(code) > 0 {
		return false
	}
	return true
}

func GetTokenInWithLogs(ctx context.Context, client *ethclient.Client, sercherAddresses []string, startBlock, endBlock uint64) ([]types.Log, error) {
	var matches []common.Hash
	for _, addr := range sercherAddresses {
		// 注意：Topic 中的地址必须补全为 32 字节 (padding)
		matches = append(matches, common.HexToHash(addr))
	}

	transferSig := crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

	query := ethereum.FilterQuery{
		// Addresses: addresses,
		FromBlock: big.NewInt(int64(startBlock)),
		ToBlock:   big.NewInt(int64(endBlock)),
		Topics: [][]common.Hash{
			{transferSig}, // Topic0: 方法签名
			nil,           // Topic1: From (任何地址)
			matches,       // Topic2: To (或者是你)
			// 注意：如果要查 From 是你，需要组合查询或者分开两次查，
			// 标准 RPC 在同一位置是 OR，不同位置是 AND。
			// 技巧：你可以查两次，或者查 topic1=[我] 和 topic2=[我] 的并集。
		},
	}
	logs, err := client.FilterLogs(ctx, query)
	if err != nil {
		return nil, err
	}
	return logs, nil
}

func GetTokenOutWithLogs(ctx context.Context, client *ethclient.Client, sercherAddresses []string, startBlock, endBlock uint64) ([]types.Log, error) {
	var matches []common.Hash
	for _, addr := range sercherAddresses {
		// 注意：Topic 中的地址必须补全为 32 字节 (padding)
		matches = append(matches, common.HexToHash(addr))
	}

	transferSig := crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

	query := ethereum.FilterQuery{
		// Addresses: addresses,
		FromBlock: big.NewInt(int64(startBlock)),
		ToBlock:   big.NewInt(int64(endBlock)),
		Topics: [][]common.Hash{
			{transferSig}, // Topic0: 方法签名
			matches,       // Topic1: From
			nil,           // Topic2: To
			// 注意：如果要查 From 是你，需要组合查询或者分开两次查，
			// 标准 RPC 在同一位置是 OR，不同位置是 AND。
			// 技巧：你可以查两次，或者查 topic1=[我] 和 topic2=[我] 的并集。
		},
	}
	logs, err := client.FilterLogs(ctx, query)
	if err != nil {
		return nil, err
	}
	return logs, nil
}

func GetBalanceAt(ctx context.Context, client *ethclient.Client, address string, blockNumber uint64) (*big.Int, error) {
	return client.BalanceAt(ctx, common.HexToAddress(address), big.NewInt(int64(blockNumber)))
}

func GetTokenBalanceAt(ctx context.Context, client *ethclient.Client, address, token string, blockNumber uint64) (*big.Int, error) {
	if token == "" || token == "0x0000000000000000000000000000000000000000" {
		return GetBalanceAt(ctx, client, address, blockNumber)
	}
	return GetERC20BalanceAt(ctx, client, address, token, blockNumber)
}

func GetBalanceNow(ctx context.Context, client *ethclient.Client, address string) (*big.Int, error) {
	return client.BalanceAt(ctx, common.HexToAddress(address), nil)
}

func GetBalanceChange(ctx context.Context, client *ethclient.Client, address string, blockBegin uint64, blockEnd uint64) (*big.Int, error) {
	blockBegin = blockBegin - 1
	balanceBegin, err := client.BalanceAt(ctx, common.HexToAddress(address), big.NewInt(int64(blockBegin)))
	if err != nil {
		return nil, err
	}
	log.Debugf("before balance:%s", balanceBegin.String())
	balanceEnd, err := client.BalanceAt(ctx, common.HexToAddress(address), big.NewInt(int64(blockEnd)))
	if err != nil {
		return nil, err
	}
	log.Debugf("after balance:%s", balanceEnd.String())
	return balanceEnd.Sub(balanceEnd, balanceBegin), nil
}

func GetERC20BalanceChange(ctx context.Context, client *ethclient.Client, user, token string, blockBegin uint64, blockEnd uint64) (*big.Int, error) {
	blockBegin = blockBegin - 1
	balanceBegin, err := GetERC20BalanceAt(ctx, client, user, token, blockBegin)
	if err != nil {
		return nil, err
	}
	log.Debugf("before balance:%s", balanceBegin.String())
	balanceEnd, err := GetERC20BalanceAt(ctx, client, user, token, blockEnd)
	if err != nil {
		return nil, err
	}
	log.Debugf("after balance:%s", balanceEnd.String())
	return balanceEnd.Sub(balanceEnd, balanceBegin), nil
}

func GetERC20BalanceAt(ctx context.Context, client *ethclient.Client, user, token string, blockNumber uint64) (*big.Int, error) {
	userAddress := common.HexToAddress(user)
	tokenAddress := common.HexToAddress(token)
	// balanceOf 方法签名 70a08231
	data := append(common.FromHex("0x70a08231"), common.LeftPadBytes(userAddress.Bytes(), 32)...)

	// balance before
	msg := ethereum.CallMsg{To: &tokenAddress, Data: data}
	balBefore, err := client.CallContract(ctx, msg, big.NewInt(int64(blockNumber)))
	if err != nil {
		return nil, err
	}
	before := new(big.Int).SetBytes(balBefore)
	return before, nil

	// // balance after
	// balAfter, err := client.CallContract(context.Background(), msg, blockNumber)
	// if err != nil {
	// 	return nil, err
	// }
	// after := new(big.Int).SetBytes(balAfter)

	// diff := new(big.Int).Sub(after, before)
	// return diff.Abs(diff), nil
}

func GetChain(ctx context.Context, client *ethclient.Client) string {
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return ""
	}
	return chainID.String() // currently only support bsc
}

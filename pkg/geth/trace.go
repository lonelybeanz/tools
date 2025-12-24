package geth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type TraceAction struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Value string `json:"value"`
}

type TxRecord struct {
	BlockNumber uint64         `json:"blockNumber"`
	TxHash      common.Hash    `json:"txHash"`
	TxFrom      common.Address `json:"txFrom"`
	TxTo        common.Address `json:"txTo"`
}

type ActionResult struct {
	Action          TraceAction `json:"action"`
	TransactionHash string      `json:"transactionHash"`
	BlockNumber     uint64      `json:"blockNumber"`
	TxFrom          string      `json:"txFrom"`
	TxTo            string      `json:"txTo"`
}

func callRPC(rpcURL, method string, params []interface{}) ([]byte, error) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})

	resp, err := http.Post(rpcURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

type RPCTraceResult struct {
	Result *TraceCall `json:"result"` // 指针，因为可能为空或出错
	Error  string     `json:"error,omitempty"`
}

func TraceTransaction(rpcURL, txHash string) (*TraceCall, error) {
	type tracerObject struct {
		Tracer  string `json:"tracer"`
		Timeout string `json:"timeout"`
	}
	tracer := tracerObject{
		Tracer:  "callTracer",
		Timeout: "5s",
	}
	resp, err := callRPC(rpcURL, "debug_traceTransaction", []interface{}{txHash, tracer})
	if err != nil {
		return nil, err
	}

	var result RPCTraceResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	if result.Error != "" {
		return nil, fmt.Errorf("API error: %v", result.Error)
	}

	return result.Result, nil
}

type BlockTraceResult struct {
	TxHash string `json:"txHash"`
	Result struct {
		Calls []TraceCall `json:"calls"`
		From  string      `json:"from"`
		To    string      `json:"to"`
	} `json:"result"`
}

func TraceBlock(rpcURL string, blockNumber uint64) ([]BlockTraceResult, error) {
	type tracerObject struct {
		Tracer  string `json:"tracer"`
		Timeout string `json:"timeout"`
	}
	tracer := tracerObject{
		Tracer:  "callTracer",
		Timeout: "5s",
	}
	resp, err := callRPC(rpcURL, "debug_traceBlockByNumber", []interface{}{hexutil.EncodeUint64(blockNumber), tracer})
	if err != nil {
		return nil, err
	}

	var result struct {
		Result []BlockTraceResult `json:"result"`
		Error  interface{}        `json:"error"`
	}
	// fmt.Println(string(resp))
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	if result.Error != nil {
		return nil, fmt.Errorf("API error: %v", result.Error)
	}

	return result.Result, nil
}

func TraceBlockForAction(rpcURL string, blockNumber uint64) (map[TxRecord][]ActionResult, error) {
	type tracerObject struct {
		Tracer  string `json:"tracer"`
		Timeout string `json:"timeout"`
	}
	tracer := tracerObject{
		Tracer:  "callTracer",
		Timeout: "5s",
	}
	resp, err := callRPC(rpcURL, "debug_traceBlockByNumber", []interface{}{hexutil.EncodeUint64(blockNumber), tracer})
	if err != nil {
		return nil, err
	}

	type TxResult struct {
		TxHash string `json:"txHash"`
		Result struct {
			Calls []TraceCall `json:"calls"`
			From  string      `json:"from"`
			To    string      `json:"to"`
		} `json:"result"`
	}

	var result struct {
		Result []TxResult  `json:"result"`
		Error  interface{} `json:"error"`
	}
	// fmt.Println(string(resp))
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	if result.Error != nil {
		return nil, fmt.Errorf("API error: %v", result.Error)
	}

	traceResultMap := make(map[TxRecord][]ActionResult)

	for _, r := range result.Result {
		traceResult := make([]ActionResult, 0)
		for _, cs := range r.Result.Calls {
			allCalls := cs.getAllCalls()
			for _, c := range allCalls {
				traceResult = append(traceResult, ActionResult{
					Action: TraceAction{
						From:  c.From,
						To:    c.To,
						Value: c.Value,
					},
					BlockNumber:     blockNumber,
					TransactionHash: r.TxHash,
					TxFrom:          r.Result.From,
					TxTo:            r.Result.To,
				})
			}
		}
		traceResultMap[TxRecord{
			BlockNumber: blockNumber,
			TxHash:      common.HexToHash(r.TxHash),
			TxFrom:      common.HexToAddress(r.Result.From),
			TxTo:        common.HexToAddress(r.Result.To),
		}] = traceResult
	}

	return traceResultMap, nil
}

// PrestateResponse 对应 RPC 返回的最外层
type PrestateResponse struct {
	Result []PrestateTxResult `json:"result"`
	Error  interface{}        `json:"error"`
}

// PrestateTxResult 对应每笔交易的结果
type PrestateTxResult struct {
	TxHash string `json:"txHash"`
	Result struct {
		Pre  map[string]AccountState `json:"pre"`
		Post map[string]AccountState `json:"post"`
	} `json:"result"`
}

// AccountState 对应账户状态
// 这里的字段都是指针或 string，因为它们可能不存在
type AccountState struct {
	Balance string `json:"balance,omitempty"`
	Nonce   uint64 `json:"nonce,omitempty"`
	// 我们不解析 storage，因为它太深且跟原生代币余额无关
	// 定义为 RawMessage 以便忽略它而不报错
	Storage json.RawMessage `json:"storage,omitempty"`
}

type BalanceChangeResult struct {
	TxHash      string   // 交易哈希
	Address     string   // 发生变化的地址
	DeltaAmount *big.Int // 变化的金额 (正数表示入账，负数表示出账)
	Type        string   // "IN" 或 "OUT"
	RawPre      *big.Int // 原始 Pre 余额
	RawPost     *big.Int // 原始 Post 余额

}

func TraceBlockForChange(rpcURL string, blockNumber uint64) ([]PrestateTxResult, error) {
	type tracerConfigObject struct {
		OnlyTopCall bool `json:"onlyTopCall"`
		DiffMode    bool `json:"diffMode"`
	}

	type tracerObject struct {
		Tracer       string             `json:"tracer"`
		Timeout      string             `json:"timeout"`
		TracerConfig tracerConfigObject `json:"tracerConfig"`
	}

	tracerConfig := tracerConfigObject{
		OnlyTopCall: false,
		DiffMode:    true,
	}
	tracer := tracerObject{
		Tracer:       "prestateTracer",
		Timeout:      "5s",
		TracerConfig: tracerConfig,
	}

	resp, err := callRPC(rpcURL, "debug_traceBlockByNumber", []interface{}{hexutil.EncodeUint64(blockNumber), tracer})
	if err != nil {
		return nil, err
	}

	var result PrestateResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	if result.Error != nil {
		return nil, fmt.Errorf("API error: %v", result.Error)
	}
	return result.Result, nil

}

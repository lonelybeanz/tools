package types

type TokenAmt struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Code   string `json:"code"`
	Amount string `json:"amount"`
}

type SwapTransactionEvent struct {
	EventIndex      int      `json:"eventIndex"`
	Sender          string   `json:"sender"`
	Receiver        string   `json:"receiver"`
	InToken         TokenAmt `json:"inToken"`
	OutToken        TokenAmt `json:"outToken"`
	PoolAddress     string   `json:"poolAddress"`
	MarketProgramId string   `json:"market"`
}

type TransferEvent struct {
	EventIndex int      `json:"eventIndex"`
	Token      TokenAmt `json:"token"`
}

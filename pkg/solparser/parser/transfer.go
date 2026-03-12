package parser

type TransferEvent struct {
	EventIndex int    `json:"eventIndex"`
	Type       string `json:"type"`
	From       string `json:"from"`
	To         string `json:"to"`
	Token      string `json:"token"`
	Amount     string `json:"amount"`
}

type Transfer struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Token  string `json:"token"`
	Amount string `json:"amount"`
}

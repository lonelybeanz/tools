package geth

import "github.com/ethereum/go-ethereum/common"

var (
	BNB = TokenPrice{
		Chain:   "56",
		Address: common.HexToAddress("0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE"),
		Symbol:  "BNB",
		Decimal: 18,
	}
	WBNB = TokenPrice{
		Chain:   "56",
		Address: common.HexToAddress("0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c"),
		Symbol:  "WBNB",
		Decimal: 18,
	}
	USDT = TokenPrice{
		Chain:   "56",
		Address: common.HexToAddress("0x55d398326f99059fF775485246999027B3197955"),
		Symbol:  "USDT",
		Decimal: 18,
		Price:   1.00,
	}
	USDC = TokenPrice{
		Chain:   "56",
		Address: common.HexToAddress("0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d"),
		Symbol:  "USDC",
		Decimal: 18,
		Price:   1.00,
	}
	USD1 = TokenPrice{
		Chain:   "56",
		Address: common.HexToAddress("0x8d0D000Ee44948FC98c9B98A4FA4921476f08B0d"),
		Symbol:  "USD1",
		Decimal: 18,
		Price:   1.00,
	}
	WBTC = TokenPrice{
		Chain:   "56",
		Address: common.HexToAddress("0x0555E30da8f98308EdB960aa94C0Db47230d2B9c"),
		Symbol:  "WBTC",
		Decimal: 8,
	}
)

type TokenPrice struct {
	Chain   string
	Address common.Address
	Symbol  string
	Decimal int
	Price   float64
}

func (t *TokenPrice) SetTokenPrice(price float64) *TokenPrice {
	t.Price = price
	return t
}

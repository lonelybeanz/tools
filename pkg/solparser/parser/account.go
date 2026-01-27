package parser

import (
	"context"
	"fmt"
	"time"

	"github.com/avast/retry-go"
	"github.com/decert-me/solana-go-sdk/common"
	"github.com/decert-me/solana-go-sdk/program/token"
	"github.com/gagliardetto/solana-go"
	"github.com/lonelybeanz/tools/pkg/solparser/consts"
)

type AccountInfo struct {
	IsATA   bool   `json:"isATA"`
	Address string `json:"address"`
	Owner   string `json:"owner"`
	Token   string `json:"token"`
}

func (s *SolParser) updateAccountCache(isAta bool, address, owner, token string) {
	if s.accountCache == nil {
		s.accountCache = make(map[string]*AccountInfo)
	}
	accountInfo, ok := s.accountCache[address]
	if !ok {
		accountInfo = &AccountInfo{
			Address: address,
			IsATA:   isAta,
			Token:   token,
			Owner:   owner,
		}
		s.accountCache[address] = accountInfo
	} else {

		accountInfo.IsATA = isAta
		if owner != "" {
			accountInfo.Owner = owner
		}
		if token != "" {
			accountInfo.Token = token
		}
	}
}

func (s *SolParser) updateTokenAccountCache(mint, tokenAccount string) {
	if s.tokenAccountCache == nil {
		s.tokenAccountCache = make(map[string][]string)
	}
	s.tokenAccountCache[mint] = append(s.tokenAccountCache[mint], tokenAccount)

	for account, accountInfo := range s.accountCache {
		if account == tokenAccount {
			accountInfo.IsATA = true
			accountInfo.Token = mint
			s.accountCache[account] = accountInfo
		}
	}
}

// 清理当前交易缓存
func (s *SolParser) clearAccountCache() {
	s.accountCache = defaultCache
	s.tokenAccountCache = make(map[string][]string)
}

// Helper function to fill token amounts
func (s *SolParser) fillTokenAmounts(transEvent *TransferEvent) error {
	if err := s.FillTokenAmtWithTransferIx(transEvent); err != nil {
		return fmt.Errorf("filling in token amount: %w", err)
	}
	return nil
}

func (s *SolParser) FillTokenAmtWithTransferIx(transEvent *TransferEvent) error {

	// 遵循以“所有者”为中心的模型来追踪资金流动

	// 场景1: Token Program (e.g. SPL Token, WSOL)
	// 对于SPL代币转账，我们将ATA地址解析为其所有者地址。
	if transEvent.Type == "tokenTransfer" {
		sourceInfo, _ := s.GetTokenAccountInfoByTokenAccount(transEvent.From)
		destInfo, _ := s.GetTokenAccountInfoByTokenAccount(transEvent.To)

		var token string
		if sourceInfo != nil {
			token = sourceInfo.Mint.String()
			s.updateAccountCache(true, transEvent.From, sourceInfo.Owner.String(), sourceInfo.Mint.String())
		}

		if destInfo != nil {
			token = destInfo.Mint.String()
			s.updateAccountCache(true, transEvent.To, destInfo.Owner.String(), destInfo.Mint.String())
		}

		transEvent.Token = token

		s.updateTokenAccountCache(token, transEvent.From)
		s.updateTokenAccountCache(token, transEvent.To)

		return nil
	}

	return nil
}

func (s *SolParser) RetryGetTokenAccountInfoByTokenAccount(tokenAccount string) (*token.TokenAccount, error) {
	var tokenInfo *token.TokenAccount
	var err error
	err = retry.Do(func() error {
		tokenInfo, err = s.GetTokenAccountInfoByTokenAccount(tokenAccount)
		if err == nil {
			return nil
		}
		return err
	}, retry.Attempts(3), retry.Delay(1*time.Second), retry.LastErrorOnly(true), retry.DelayType(func(n uint, err error, config *retry.Config) time.Duration {
		return retry.BackOffDelay(n, err, config)
	}))
	return tokenInfo, err

}

func (s *SolParser) GetTokenAccountInfoByTokenAccount(tokenAccount string) (*token.TokenAccount, error) {

	if accountInfo, ok := s.accountCache[tokenAccount]; ok {
		if accountInfo.Owner != "" && accountInfo.Token != "" {
			t := &token.TokenAccount{
				Mint:  common.PublicKeyFromString(accountInfo.Token),
				Owner: common.PublicKeyFromString(accountInfo.Owner),
			}
			return t, nil
		}
		if !accountInfo.IsATA {
			t := &token.TokenAccount{
				Mint:  common.PublicKeyFromString(consts.SOL),
				Owner: common.PublicKeyFromString(tokenAccount),
			}
			return t, nil
		}

	}

	ctx := context.Background()
	accountInfo, e := s.cli.GetAccountInfo(ctx, solana.MustPublicKeyFromBase58(tokenAccount))
	if e != nil {
		return nil, e
	}
	if accountInfo.Value != nil && accountInfo.Value.Owner == solana.SystemProgramID {
		uint64One := uint64(1)
		// 账户本身就是一个 lamport 账户，非Token账户
		t := &token.TokenAccount{
			Mint:            common.PublicKeyFromString(consts.SOL),
			Owner:           common.PublicKeyFromString(tokenAccount),
			Amount:          accountInfo.Value.Lamports,
			Delegate:        nil,
			State:           1, //tokenAccount.Initialized
			IsNative:        &uint64One,
			DelegatedAmount: 0,
			CloseAuthority:  nil,
		}
		return t, nil
	}
	t, err2 := token.TokenAccountFromData(accountInfo.GetBinary())
	if err2 != nil {
		return nil, fmt.Errorf("error decoding token account data: %v", err2)
	}

	return &t, nil
}

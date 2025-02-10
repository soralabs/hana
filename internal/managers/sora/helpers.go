package sora_manager

import (
	"fmt"

	"github.com/soralabs/hana/internal/dexscreener"
	"github.com/soralabs/zen/cache"
)

func (s *SoraManager) getSoraTokenData() (string, error) {
	cacheKey := cache.CacheKey("sora_token_data")
	cacheValue, exists := s.cache.Get(cacheKey)
	if exists {
		return cacheValue.(string), nil
	}

	data, err := dexscreener.GetPairInformation(s.Ctx, SoraMintAddress)
	if err != nil {
		return "", fmt.Errorf("failed to get pair information: %w", err)
	}

	if len(data) == 0 {
		return "", fmt.Errorf("no data found")
	}

	tokenInfo := data[0]
	summary := fmt.Sprintf("Sora trading summary: Price: %s USD (%s native). Volumes: 24h: %.2f USD, 1h: %.2f USD. Market metrics: Cap: %.2f USD, FDV: %.2f USD, Liquidity: %.2f USD. Transactions: 24h - %d buys, %d sells; 1h - %d buys, %d sells. Price changes: 5m: %.2f%%, 1h: %.2f%%, 6h: %.2f%%, 24h: %.2f%%.",
		tokenInfo.PriceUsd,
		tokenInfo.PriceNative,
		tokenInfo.Volume.H24,
		tokenInfo.Volume.H1,
		tokenInfo.MarketCap,
		tokenInfo.Fdv,
		tokenInfo.Liquidity.Usd,
		tokenInfo.Txns.H24.Buys,
		tokenInfo.Txns.H24.Sells,
		tokenInfo.Txns.H1.Buys,
		tokenInfo.Txns.H1.Sells,
		tokenInfo.PriceChange.M5,
		tokenInfo.PriceChange.H1,
		tokenInfo.PriceChange.H6,
		tokenInfo.PriceChange.H24)

	s.cache.Set(cacheKey, summary)

	return summary, nil
}

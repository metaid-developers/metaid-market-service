package common

import (
	"encoding/hex"
	"errors"
	"strings"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
)

// DeriveIdCoinLpVaultChild derives the LP vault child key for a tick from the
// account xpub (m/84'/0'/1000'): the child path is the UTF-8 bytes of
// UPPER(tick), each byte a non-hardened index. Spec: docs/idcoin-floor-liquidity-spec.md §2.
// Returns the compressed public key hex and the P2WPKH address.
func DeriveIdCoinLpVaultChild(accountXpub, tick string, params *chaincfg.Params) (string, string, error) {
	if params == nil {
		params = &chaincfg.MainNetParams
	}
	tick = strings.ToUpper(strings.TrimSpace(tick))
	if tick == "" {
		return "", "", errors.New("tick is empty")
	}
	if strings.TrimSpace(accountXpub) == "" {
		return "", "", errors.New("lp vault xpub is empty")
	}
	key, err := hdkeychain.NewKeyFromString(accountXpub)
	if err != nil {
		return "", "", err
	}
	for _, b := range []byte(tick) {
		key, err = key.Derive(uint32(b))
		if err != nil {
			return "", "", err
		}
	}
	pubKey, err := key.ECPubKey()
	if err != nil {
		return "", "", err
	}
	compressed := pubKey.SerializeCompressed()
	addr, err := btcutil.NewAddressWitnessPubKeyHash(btcutil.Hash160(compressed), params)
	if err != nil {
		return "", "", err
	}
	return hex.EncodeToString(compressed), addr.EncodeAddress(), nil
}

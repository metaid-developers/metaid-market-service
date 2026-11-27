package mrc20_op_service

import (
	"errors"
	"metaid-market-service/common"
	"metaid-market-service/conf"
	"metaid-market-service/major"
	"metaid-market-service/models"
	"metaid-market-service/service/man_service"
	"strconv"
	"strings"

	"github.com/spf13/viper"
	"gorm.io/gorm"
)

// idCoinsLaunchEnabled gates self-hosted launch; default false keeps the
// maintenance behaviour.
func idCoinsLaunchEnabled() bool {
	return viper.GetBool("idcoins.launch_enabled")
}

func idCoinsLpVaultXpub() string {
	return strings.TrimSpace(viper.GetString("idcoins.lp_vault_xpub"))
}

// deriveIdCoinLpVault derives the LP vault pubkey+address for a tick from the
// configured account xpub. Fails closed when the xpub is not configured.
func deriveIdCoinLpVault(tick string) (string, string, error) {
	xpub := idCoinsLpVaultXpub()
	if xpub == "" {
		return "", "", errors.New("idcoins lp vault xpub is not configured")
	}
	return common.DeriveIdCoinLpVaultChild(xpub, tick, common.GetNetParams(conf.Net, "btc"))
}

// checkIdCoinLaunchLiquidity enforces the optional idcoins.min_liquidity_per_mint
// floor (0 = off).
func checkIdCoinLaunchLiquidity(liquidityPerMint int64) error {
	if minLiquidity := viper.GetInt64("idcoins.min_liquidity_per_mint"); minLiquidity > 0 && liquidityPerMint < minLiquidity {
		return errors.New("liquidityPerMint must be at least " + strconv.FormatInt(minLiquidity, 10))
	}
	return nil
}

// checkIdCoinsDeployer ports the legacy deployer checks
// (common.GetPlatformKeyForIdCoinLimit): platform.idcoins.limit_deploys caps
// finished deploys platform-wide, platform.idcoins.mint_min_pins requires the
// issuer to have at least that many creator pins.
func checkIdCoinsDeployer(issuerAddress string) (string, bool, error) {
	var (
		limitDeployer int64 = viper.GetInt64("platform.idcoins.limit_deploys")
		mintMinPins   int64 = viper.GetInt64("platform.idcoins.mint_min_pins")
	)
	totalDeploy, _ := models.IdCoinsDeployOrderModelDao().Count(&models.IdCoinsDeployOrderModel{InscribeState: models.InscribeStateFinish})
	if limitDeployer > 0 && totalDeploy >= limitDeployer {
		return "deployer limit", false, nil
	}
	if mintMinPins > 0 {
		total, err := fetchCreatorPinsCount(issuerAddress)
		if err != nil {
			return "", false, err
		}
		if total < mintMinPins {
			return "creator pins not enough", false, nil
		}
	}
	return "", true, nil
}

func fetchCreatorPinsCount(address string) (int64, error) {
	pins, err := man_service.FetchPinsByPath(conf.Net, "creator", address, "", 0, 1)
	if err != nil {
		if strings.Contains(err.Error(), "no data found") {
			return 0, nil
		}
		return 0, err
	}
	if pins == nil {
		return 0, nil
	}
	return pins.Total, nil
}

// listSelfDeployedCoins returns confirmed self-hosted deploys whose tickSign
// verifies (legacy or platform pubkey) and whose payTo equals the LP vault
// derived from the configured xpub. Deploy rows failing either check are not
// part of the coin set.
func listSelfDeployedCoins() ([]common.IdCoinSnapshotCoin, error) {
	out := make([]common.IdCoinSnapshotCoin, 0)
	db := major.GetSqlDB()
	if db == nil {
		return out, nil
	}
	list := make([]*models.IdCoinsDeployOrderModel, 0)
	err := db.Model(&models.IdCoinsDeployOrderModel{}).
		Where("inscribeState = ? AND deployType = ?", int64(models.InscribeStateFinish), models.DeployTypeIdCoins).
		Order("timestamp asc").Find(&list).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	xpub := idCoinsLpVaultXpub()
	for _, entity := range list {
		if entity.TickId == "" || entity.Tick == "" {
			continue
		}
		if findIdCoinSnapshotByTick(entity.Tick) != nil {
			continue
		}
		if common.CheckIdCoins(entity.Tick, entity.Metadata, entity.Timestamp) != "id-coins" {
			continue
		}
		if xpub == "" {
			continue
		}
		_, vaultAddr, err := common.DeriveIdCoinLpVaultChild(xpub, entity.Tick, common.GetNetParams(conf.Net, "btc"))
		if err != nil || vaultAddr == "" {
			continue
		}
		if !strings.EqualFold(entity.PayCheckAddress, vaultAddr) {
			continue
		}
		out = append(out, common.IdCoinSnapshotCoin{Tick: entity.Tick, TickId: entity.TickId})
	}
	return out, nil
}

// mergedIdCoinSet is the full coin set for all id-coins endpoints and mint
// gating: the 32-coin snapshot plus confirmed self-deployed coins.
func mergedIdCoinSet() []common.IdCoinSnapshotCoin {
	set := common.IdCoinsSnapshot()
	deployed, err := listSelfDeployedCoins()
	if err != nil {
		return set
	}
	return append(set, deployed...)
}

func findMergedIdCoinByTickId(tickId string) *common.IdCoinSnapshotCoin {
	if coin := findIdCoinSnapshotByTickId(tickId); coin != nil {
		return coin
	}
	deployed, err := listSelfDeployedCoins()
	if err != nil {
		return nil
	}
	for i := range deployed {
		if deployed[i].TickId == tickId {
			return &deployed[i]
		}
	}
	return nil
}

func findMergedIdCoinByTick(tick string) *common.IdCoinSnapshotCoin {
	if coin := findIdCoinSnapshotByTick(tick); coin != nil {
		return coin
	}
	deployed, err := listSelfDeployedCoins()
	if err != nil {
		return nil
	}
	for i := range deployed {
		if strings.EqualFold(deployed[i].Tick, tick) {
			return &deployed[i]
		}
	}
	return nil
}

// idCoinTickTaken reports whether UPPER(tick) is already used: the 32-coin
// snapshot, a confirmed self-hosted deploy, or an existing MAN tick.
func idCoinTickTaken(tick string) (bool, error) {
	upper := strings.ToUpper(strings.TrimSpace(tick))
	if upper == "" {
		return true, errors.New("tick is empty")
	}
	if findIdCoinSnapshotByTick(upper) != nil {
		return true, nil
	}
	deployed, err := listSelfDeployedCoins()
	if err != nil {
		return false, err
	}
	for _, coin := range deployed {
		if strings.EqualFold(coin.Tick, upper) {
			return true, nil
		}
	}
	info, err := man_service.FetchMrc20TickInfo("", upper)
	if err != nil && !strings.Contains(err.Error(), "no data found") && !strings.Contains(err.Error(), "not found") {
		return false, err
	}
	if info != nil && info.Mrc20Id != "" {
		return true, nil
	}
	return false, nil
}

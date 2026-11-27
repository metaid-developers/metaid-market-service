package mrc20_op_service

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"metaid-market-service/common"
	"metaid-market-service/conf"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/models"
	"metaid-market-service/service/common_service"
	"metaid-market-service/service/mrc20_service"
	"metaid-market-service/service/risk_service"
	"metaid-market-service/tool"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

// idCoinMintEnabled gates self-hosted minting: the coin must be one of the 32
// snapshot coins AND its tick must be listed in the idcoins.mint_enabled_ticks
// config (read through viper directly; empty means nothing is enabled).
func idCoinMintEnabled(tickId string) bool {
	coin := findMergedIdCoinByTickId(tickId)
	if coin == nil {
		return false
	}
	for _, t := range viper.GetStringSlice("idcoins.mint_enabled_ticks") {
		if strings.EqualFold(strings.TrimSpace(t), coin.Tick) {
			return true
		}
	}
	return false
}

// idCoinMintSweepFee is what the keeper additionally needs to sweep the payTo
// output of a legacy pool coin: ceil(networkFeeRate x 110) — one small P2WPKH
// input plus output overhead of the sweep transaction.
func idCoinMintSweepFee(networkFeeRate int64) int64 {
	fee := networkFeeRate * 110
	if fee < 0 {
		return 0
	}
	return fee
}

func IdCoinsMintPre(req *request.IdCoinsMintPreRequest, publicKey, ip string) (*respond.IdCoinsMintPreResp, error) {
	var (
		chain     string = "btc"
		orderId   string = ""
		mintOrder *models.IdCoinsMintOrderModel
		err       error

		totalFee              int64  = 0
		inscribeMinerFee      int64  = 0
		inscribeMinerGas      int64  = 0
		inscribeMinerOutValue int64  = 0
		mintMinerFee          int64  = 0
		mintMinerGas          int64  = 0
		mintMinerOutValue     int64  = 0
		serviceFee            int64  = common.GetPlatformServiceFeeConfigData().MintFee
		serviceAddress        string = common.GetPlatformServiceFeeConfigData().ServiceAddress

		mrc20Builder             *mrc20_service.Mrc20Builder
		mrc20OpRequest           *mrc20_service.Mrc20OpRequest
		payTos                   []*mrc20_service.PayTo    = make([]*mrc20_service.PayTo, 0)
		otherOuts                []*mrc20_service.OtherOut = make([]*mrc20_service.OtherOut, 0)
		tickId                   string                    = req.TickId
		payToAddress             string                    = ""
		payToAmount              int64                     = 0
		pinOutValue              int64                     = req.OutValue
		mrc20OutValue            int64                     = req.OutValue
		mrc20OutAddressList      []string                  = []string{req.OutAddress, req.OutAddress}
		address                  string                    = req.OutAddress
		feeRate                  int64                     = req.NetworkFeeRate
		revealInscribePrePsbtRaw string                    = ""
		revealMintPrePsbtRaw     string                    = ""
		revealInscribeAddress    string                    = ""
		revealMintAddress        string                    = ""
		revealInscribeInputIndex int64                     = 0
		revealInputIndex         int64                     = 1
		followMetaId             string                    = ""

		nowTime int64 = tool.MakeTimestamp()

		extra map[string]interface{} = make(map[string]interface{})

		tickInfo *common_service.TickInfo
	)

	if !idCoinMintEnabled(req.TickId) {
		return nil, errIdCoinsMaintenance
	}

	verified, err := common.CheckPublicKeyAddress(common.GetNetParams(conf.Net, chain), publicKey, address)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Check address err: %s. ", err.Error()))
	}
	if !verified {
		return nil, errors.New(fmt.Sprintf("Check address verified: %v. ", verified))
	}
	if err := risk_service.CheckAddress(chain, address); err != nil {
		return nil, err
	}

	if serviceFee >= 546 {
		otherOuts = append(otherOuts, &mrc20_service.OtherOut{
			Address: serviceAddress,
			Amount:  serviceFee,
		})
	}

	tickInfo, err = common_service.GetMrc20TickInfo(tickId, "")
	if err != nil {
		return nil, err
	}
	if tickInfo.PayCheck != nil && tickInfo.PayCheck.PayTo != "" && tickInfo.PayCheck.PayAmount != "" {
		payToAddress = tickInfo.PayCheck.PayTo
		payTo := &mrc20_service.PayTo{
			Address: tickInfo.PayCheck.PayTo,
		}
		payTo.Amount, _ = strconv.ParseInt(tickInfo.PayCheck.PayAmount, 10, 64)
		if payTo.Amount <= 546 {
			payTo.Amount = 546
		}
		// Legacy-pool coins (the 32 snapshot coins) are swept by the keeper
		// immediately, so the minter funds the sweep fee. Self-deployed coins
		// pay directly into their LP vault — no sweep, no sweepFee.
		if findIdCoinSnapshotByTickId(tickId) != nil {
			payTo.Amount += idCoinMintSweepFee(feeRate)
		}
		payToAmount = payTo.Amount
		payTos = append(payTos, payTo)
	}

	mintCountDe, _ := decimal.NewFromString(tickInfo.MintCount)
	totalMintedDe, _ := decimal.NewFromString(tickInfo.TotalMinted)
	if totalMintedDe.GreaterThanOrEqual(mintCountDe) {
		return nil, errors.New("the mint limit has been reached")
	}

	if tickInfo.PinCheck != nil {
		pinCheck := &common_service.PinCheck{}
		if err := tool.JsonToAny(tickInfo.PinCheck, pinCheck); err != nil {
			return nil, err
		}
		followMetaId, err = common_service.ExtractMetaIDFromPath(pinCheck.Path)
		if err != nil {
			return nil, err
		}
	}
	if followMetaId == "" {
		return nil, errors.New("followMetaId is empty")
	}

	payload := fmt.Sprintf(`{"id":"%s", "vout":"%d"}`, tickId, revealInputIndex+1)
	mrc20OpRequest = &mrc20_service.Mrc20OpRequest{
		Net:                 common.GetNetParams(conf.Net, chain),
		MetaIdFlag:          common.GetMetaIdFlag(),
		Op:                  "mint",
		OpPayload:           payload,
		MintPins:            nil,
		PayTos:              payTos,
		TransferMrc20s:      nil,
		Mrc20OutValue:       mrc20OutValue,
		Mrc20OutAddressList: mrc20OutAddressList,
		PinOutAddress:       address,
		PinOutValue:         pinOutValue,
		ChangeAddress:       address,
		OtherOuts:           otherOuts,
	}

	mrc20Builder, inscribeMinerFee, mintMinerFee, err = mrc20_service.IdCoinsMintBuilder(mrc20OpRequest, followMetaId, feeRate)
	if err != nil {
		return nil, err
	}
	revealMintPrePsbtRaw, err = mrc20Builder.RevealPsbtBuilder.ToString()
	if err != nil {
		return nil, err
	}
	revealInscribePrePsbtRaw, err = mrc20Builder.RevealInscribePsbtBuilder.ToString()
	if err != nil {
		return nil, err
	}
	revealInscribeAddress, err = common.PkScriptToAddress(conf.Net, chain, hex.EncodeToString(mrc20Builder.InscribeTxCtxData.CommitTxAddressPkScript))
	if err != nil {
		return nil, err
	}
	revealMintAddress, err = common.PkScriptToAddress(conf.Net, chain, hex.EncodeToString(mrc20Builder.TxCtxData.CommitTxAddressPkScript))
	if err != nil {
		return nil, err
	}
	totalFee = inscribeMinerFee + mintMinerFee
	inscribeMinerOutValue = mrc20OpRequest.PinOutValue
	inscribeMinerGas = inscribeMinerFee - inscribeMinerOutValue
	for _, outAddress := range mrc20OpRequest.Mrc20OutAddressList {
		_ = outAddress
		mintMinerOutValue += mrc20OpRequest.Mrc20OutValue
	}
	mintMinerGas = mintMinerFee - mintMinerOutValue - serviceFee - payToAmount

	orderId = fmt.Sprintf("id_coins_mint_%s_%s_%d", tickId, req.OutAddress, nowTime)
	orderId = hex.EncodeToString(tool.SHA256([]byte(orderId)))

	mintOrder, err = models.IdCoinsMintOrderModelDao().GetOne(&models.IdCoinsMintOrderModel{OrderId: orderId})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if mintOrder != nil && (mintOrder.InscribeState == models.InscribeStateFinish) {
		return nil, errors.New("mint order already exists")
	}

	if mintOrder == nil {
		mintOrder = &models.IdCoinsMintOrderModel{
			OrderId:                     orderId,
			InscribeState:               models.InscribeStatePending,
			Address:                     address,
			TickId:                      tickId,
			PayToAddress:                payToAddress,
			PayToAmount:                 payToAmount,
			MintPins:                    "",
			NetworkFeeRate:              feeRate,
			TotalFee:                    totalFee,
			InscribeMinerFee:            inscribeMinerFee,
			MintMinerFee:                mintMinerFee,
			ServiceFee:                  serviceFee,
			CommitTxRaw:                 "",
			PinAddress:                  mrc20Builder.PinAddress,
			PinPrivateKey:               mrc20Builder.PinPrivateKeyHex,
			RevealOutValue:              mrc20OutValue,
			RevealInscribeTxPrivateKey:  mrc20Builder.InscribeTxCtxData.RecoveryPrivateKeyHex,
			RevealMintTxPrivateKey:      mrc20Builder.TxCtxData.RecoveryPrivateKeyHex,
			RevealInscribeTxAddress:     revealInscribeAddress,
			RevealMintTxAddress:         revealMintAddress,
			RedeemScriptInscribe:        hex.EncodeToString(mrc20Builder.InscribeTxCtxData.InscriptionScript),
			ControlBlockWitnessInscribe: hex.EncodeToString(mrc20Builder.InscribeTxCtxData.ControlBlockWitness),
			RevealInscribeInputIndex:    revealInscribeInputIndex,
			RevealInscribePrePsbtRaw:    revealInscribePrePsbtRaw,
			RevealInscribeFinalPsbtRaw:  "",
			RedeemScriptMint:            hex.EncodeToString(mrc20Builder.TxCtxData.InscriptionScript),
			ControlBlockWitnessMint:     hex.EncodeToString(mrc20Builder.TxCtxData.ControlBlockWitness),
			RevealMintInputIndex:        revealInputIndex,
			RevealMintPrePsbtRaw:        revealMintPrePsbtRaw,
			RevealMintFinalPsbtRaw:      "",
			CommitTxId:                  "",
			RevealInscribeTxId:          "",
			RevealMintTxId:              "",
			BlockHeight:                 0,
			ConfirmationState:           models.ConfirmationStateUnconfirmed,
			Timestamp:                   nowTime,
			Version:                     0,
			CreateTime:                  nowTime,
			UpdateTime:                  0,
			State:                       models.STATE_EXIST,
		}
		err = models.IdCoinsMintOrderModelDao().Set(mintOrder)
		if err != nil {
			return nil, err
		}
	} else {
		mintOrder.Address = address
		mintOrder.PayToAddress = payToAddress
		mintOrder.PayToAmount = payToAmount
		mintOrder.NetworkFeeRate = feeRate
		mintOrder.TotalFee = totalFee
		mintOrder.InscribeMinerFee = inscribeMinerFee
		mintOrder.MintMinerFee = mintMinerFee
		mintOrder.ServiceFee = serviceFee
		mintOrder.PinAddress = mrc20Builder.PinAddress
		mintOrder.PinPrivateKey = mrc20Builder.PinPrivateKeyHex
		mintOrder.RevealOutValue = mrc20OutValue
		mintOrder.RevealInscribeTxAddress = revealInscribeAddress
		mintOrder.RevealMintTxAddress = revealMintAddress
		mintOrder.RevealInscribeTxPrivateKey = mrc20Builder.InscribeTxCtxData.RecoveryPrivateKeyHex
		mintOrder.RevealMintTxPrivateKey = mrc20Builder.TxCtxData.RecoveryPrivateKeyHex
		mintOrder.RedeemScriptInscribe = hex.EncodeToString(mrc20Builder.InscribeTxCtxData.InscriptionScript)
		mintOrder.ControlBlockWitnessInscribe = hex.EncodeToString(mrc20Builder.InscribeTxCtxData.ControlBlockWitness)
		mintOrder.RevealInscribeInputIndex = revealInscribeInputIndex
		mintOrder.RevealInscribePrePsbtRaw = revealInscribePrePsbtRaw
		mintOrder.RedeemScriptMint = hex.EncodeToString(mrc20Builder.TxCtxData.InscriptionScript)
		mintOrder.ControlBlockWitnessMint = hex.EncodeToString(mrc20Builder.TxCtxData.ControlBlockWitness)
		mintOrder.RevealMintInputIndex = revealInputIndex
		mintOrder.RevealMintPrePsbtRaw = revealMintPrePsbtRaw
		mintOrder.UpdateTime = nowTime
		err = models.IdCoinsMintOrderModelDao().Update(mintOrder)
		if err != nil {
			return nil, err
		}
	}

	return &respond.IdCoinsMintPreResp{
		OrderId:                mintOrder.OrderId,
		TotalFee:               mintOrder.TotalFee,
		RevealInscribeFee:      mintOrder.InscribeMinerFee,
		RevealInscribeGas:      inscribeMinerGas,
		RevealInscribeOutValue: inscribeMinerOutValue,
		RevealMintFee:          mintOrder.MintMinerFee,
		RevealMintGas:          mintMinerGas,
		RevealMintOutValue:     mintMinerOutValue,
		RevealInscribeAddress:  mintOrder.RevealInscribeTxAddress,
		RevealMintAddress:      revealMintAddress,
		ServiceFee:             mintOrder.ServiceFee,
		PayToAmount:            payToAmount,
		Extra:                  extra,
	}, nil
}

// validateIdCoinsMintCommitTx checks the user's commit transaction against the
// stored order: both reveal outputs must be present at the declared indexes,
// paying the exact stored amounts to the exact stored reveal addresses.
func validateIdCoinsMintCommitTx(net, chain string, commitTx *wire.MsgTx, mintOrder *models.IdCoinsMintOrderModel, inscribeIndex, mintIndex int64) error {
	if err := common.ValidateCommitOutput(net, chain, commitTx, inscribeIndex, mintOrder.RevealInscribeTxAddress, mintOrder.InscribeMinerFee); err != nil {
		return fmt.Errorf("commit inscribe output invalid: %w", err)
	}
	if err := common.ValidateCommitOutput(net, chain, commitTx, mintIndex, mintOrder.RevealMintTxAddress, mintOrder.MintMinerFee); err != nil {
		return fmt.Errorf("commit mint output invalid: %w", err)
	}
	if inscribeIndex == mintIndex {
		return errors.New("commit inscribe and mint output indexes must differ")
	}
	return nil
}

func IdCoinsMintCommit(req *request.IdCoinsMintCommitRequest, publicKey, ip string) (*respond.IdCoinsMintCommitResp, error) {
	var (
		chain                string = "btc"
		orderId              string = req.OrderId
		commitTxRaw          string = req.CommitTxRaw
		commitTxId           string = ""
		revealInscribeTxRaw  string = ""
		revealMintTxRaw      string = ""
		preInscribePsbtRaw   string = ""
		finalInscribePsbtRaw string = ""
		preMintPsbtRaw       string = ""
		finalMintPsbtRaw     string = ""

		address string = ""

		inscribeTxId string = ""
		mintTxId     string = ""
		mintOrder    *models.IdCoinsMintOrderModel
		err          error

		txRawByte                []byte
		commitTx                 *wire.MsgTx = wire.NewMsgTx(2)
		commitTxOutInscribeIndex int64       = req.CommitTxOutInscribeIndex
		commitTxOutMintIndex     int64       = req.CommitTxOutMintIndex
		revealInscribeTx         *wire.MsgTx = wire.NewMsgTx(2)
		revealInscribeTxRawByte  []byte
		revealMintTx             *wire.MsgTx = wire.NewMsgTx(2)
		revealMintTxRawByte      []byte

		inscribePsbtBuilder   *common.PsbtBuilder
		mintPsbtBuilder       *common.PsbtBuilder
		inscribeInSigner      *common.InputSign
		mintInSigner          *common.InputSign
		revealInscribeSigners []*common.InputSign = make([]*common.InputSign, 0)
		revealMintSigners     []*common.InputSign = make([]*common.InputSign, 0)
		inscribePkScript      string              = ""
		mintPkScript          string              = ""
		pinPkScript           string              = ""

		nowTime int64 = tool.MakeTimestamp()
	)

	mintOrder, err = models.IdCoinsMintOrderModelDao().GetOne(&models.IdCoinsMintOrderModel{OrderId: orderId})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if mintOrder == nil {
		return nil, errors.New("mint order not exists")
	}
	if mintOrder.InscribeState != models.InscribeStatePending {
		return nil, errors.New("mint order state error")
	}
	preInscribePsbtRaw = mintOrder.RevealInscribePrePsbtRaw
	preMintPsbtRaw = mintOrder.RevealMintPrePsbtRaw
	if preInscribePsbtRaw == "" || preMintPsbtRaw == "" {
		return nil, errors.New("mint order reveal psbt is empty")
	}

	if err := risk_service.CheckRawBtcTx(chain, commitTxRaw); err != nil {
		return nil, err
	}

	txRawByte, _ = hex.DecodeString(commitTxRaw)
	err = commitTx.Deserialize(bytes.NewReader(txRawByte))
	if err != nil {
		return nil, err
	}
	commitTxId = commitTx.TxHash().String()
	if len(commitTx.TxOut) <= int(commitTxOutInscribeIndex) || len(commitTx.TxOut) <= int(commitTxOutMintIndex) {
		return nil, errors.New("commitTxOutIndex error")
	}
	if err = validateIdCoinsMintCommitTx(conf.Net, chain, commitTx, mintOrder, commitTxOutInscribeIndex, commitTxOutMintIndex); err != nil {
		return nil, err
	}

	commitTxIn := commitTx.TxIn[0]
	utxoInfo := common.GetUtxoInfo(conf.Net, chain, commitTxIn.PreviousOutPoint.Hash.String(), int64(commitTxIn.PreviousOutPoint.Index))
	if utxoInfo == nil {
		return nil, errors.New("commitTx utxoInfo not exists")
	}
	address = utxoInfo.Address
	verified, err := common.CheckPublicKeyAddress(common.GetNetParams(conf.Net, chain), publicKey, address)
	if err != nil || !verified {
		return nil, errors.New("commitTx funding address does not belong to the authenticated public key")
	}

	// reveal-inscribe: spends commit output [inscribeIndex] through the taproot
	// inscription input, creates the /follow pin output.
	{
		inscribePsbtBuilder, err = common.NewPsbtBuilder(common.GetNetParams(conf.Net, chain), preInscribePsbtRaw)
		if err != nil {
			return nil, err
		}

		inscribePsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxIn[mintOrder.RevealInscribeInputIndex].PreviousOutPoint = wire.OutPoint{
			Hash:  commitTx.TxHash(),
			Index: uint32(commitTxOutInscribeIndex),
		}

		inscribePkScript, err = common.AddressToPkScript(conf.Net, chain, mintOrder.RevealInscribeTxAddress)
		if err != nil {
			return nil, err
		}
		inscribeInSigner = &common.InputSign{
			UtxoType:            common.Taproot,
			Index:               int(mintOrder.RevealInscribeInputIndex),
			PkScript:            inscribePkScript,
			RedeemScript:        mintOrder.RedeemScriptInscribe,
			ControlBlockWitness: mintOrder.ControlBlockWitnessInscribe,
			Amount:              uint64(mintOrder.InscribeMinerFee),
			SighashType:         txscript.SigHashAll,
			PriHex:              mintOrder.RevealInscribeTxPrivateKey,
		}
		revealInscribeSigners = append(revealInscribeSigners, inscribeInSigner)
		err = inscribePsbtBuilder.UpdateAndSignTaprootInput(revealInscribeSigners)
		if err != nil {
			return nil, err
		}
		finalInscribePsbtRaw, err = inscribePsbtBuilder.ToString()
		if err != nil {
			return nil, err
		}

		revealInscribeTxRaw, err = inscribePsbtBuilder.ExtractPsbtTransaction()
		if err != nil {
			return nil, err
		}
		revealInscribeTxRawByte, _ = hex.DecodeString(revealInscribeTxRaw)
		err = revealInscribeTx.Deserialize(bytes.NewReader(revealInscribeTxRawByte))
		if err != nil {
			return nil, err
		}
		inscribeTxId = revealInscribeTx.TxHash().String()
	}

	// reveal-mint: input 0 spends the pin output of reveal-inscribe (pin key,
	// witness), input [mintIndex of the taproot data input] spends commit
	// output [mintIndex]; inscribes the MRC20 mint.
	{
		mintPsbtBuilder, err = common.NewPsbtBuilder(common.GetNetParams(conf.Net, chain), preMintPsbtRaw)
		if err != nil {
			return nil, err
		}
		mintPkScript, err = common.AddressToPkScript(conf.Net, chain, mintOrder.RevealMintTxAddress)
		if err != nil {
			return nil, err
		}

		mintPsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxIn[0].PreviousOutPoint = wire.OutPoint{
			Hash:  revealInscribeTx.TxHash(),
			Index: uint32(0),
		}

		mintPsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxIn[mintOrder.RevealMintInputIndex].PreviousOutPoint = wire.OutPoint{
			Hash:  commitTx.TxHash(),
			Index: uint32(commitTxOutMintIndex),
		}

		pinPkScript, err = common.AddressToPkScript(conf.Net, chain, mintOrder.PinAddress)
		if err != nil {
			return nil, err
		}

		mintPinInSigner := &common.InputSign{
			UtxoType:    common.Witness,
			Index:       int(0),
			PkScript:    pinPkScript,
			Amount:      uint64(mintOrder.RevealOutValue),
			SighashType: txscript.SigHashAll,
			PriHex:      mintOrder.PinPrivateKey,
		}
		pinMintSigners := make([]*common.InputSign, 0)
		pinMintSigners = append(pinMintSigners, mintPinInSigner)
		err = mintPsbtBuilder.UpdateAndSignInput(pinMintSigners)
		if err != nil {
			return nil, err
		}

		mintInSigner = &common.InputSign{
			UtxoType:            common.Taproot,
			Index:               int(mintOrder.RevealMintInputIndex),
			PkScript:            mintPkScript,
			RedeemScript:        mintOrder.RedeemScriptMint,
			ControlBlockWitness: mintOrder.ControlBlockWitnessMint,
			Amount:              uint64(mintOrder.MintMinerFee),
			SighashType:         txscript.SigHashAll,
			PriHex:              mintOrder.RevealMintTxPrivateKey,
		}
		revealMintSigners = append(revealMintSigners, mintInSigner)
		err = mintPsbtBuilder.UpdateAndSignTaprootInput(revealMintSigners)
		if err != nil {
			return nil, err
		}
		finalMintPsbtRaw, err = mintPsbtBuilder.ToString()
		if err != nil {
			return nil, err
		}

		revealMintTxRaw, err = mintPsbtBuilder.ExtractPsbtTransaction()
		if err != nil {
			return nil, err
		}
		revealMintTxRawByte, _ = hex.DecodeString(revealMintTxRaw)
		err = revealMintTx.Deserialize(bytes.NewReader(revealMintTxRawByte))
		if err != nil {
			return nil, err
		}
		mintTxId = revealMintTx.TxHash().String()
	}

	mintOrder.PayUtxoId = fmt.Sprintf("%s_%d", mintTxId, 3)
	mintOrder.MintPins = fmt.Sprintf("%si0", inscribeTxId)
	mintOrder.Address = address
	mintOrder.CommitTxRaw = commitTxRaw
	mintOrder.RevealInscribeFinalPsbtRaw = finalInscribePsbtRaw
	mintOrder.RevealMintFinalPsbtRaw = finalMintPsbtRaw
	mintOrder.CommitTxId = commitTxId
	mintOrder.RevealInscribeTxId = inscribeTxId
	mintOrder.RevealMintTxId = mintTxId
	mintOrder.InscribeState = models.InscribeStatePaid
	mintOrder.UpdateTime = nowTime
	err = models.IdCoinsMintOrderModelDao().Update(mintOrder)
	if err != nil {
		return nil, err
	}

	err = models.IdCoinsMintOrderModelDao().UpdateEntityListForInscribing(
		mintOrder,
		[]string{commitTxRaw, revealInscribeTxRaw, revealMintTxRaw},
		nil)
	if err != nil {
		return nil, err
	}
	if err = BroadcastInscribeTxs(chain, []string{commitTxRaw, revealInscribeTxRaw, revealMintTxRaw}); err != nil {
		return nil, err
	}
	err = models.IdCoinsMintOrderModelDao().UpdateEntityListForInscribing(mintOrder, nil, nil)
	if err != nil {
		return nil, err
	}
	return &respond.IdCoinsMintCommitResp{
		OrderId:            mintOrder.OrderId,
		CommitTxId:         commitTxId,
		RevealInscribeTxId: inscribeTxId,
		RevealMintTxId:     mintTxId,
	}, nil
}

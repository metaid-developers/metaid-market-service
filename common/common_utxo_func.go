package common

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/txscript"
	"metaid-market-service/conf"
	"metaid-market-service/models"
	"metaid-market-service/redis"
	"sync"
)

const (
	maxLimit int64 = 500
)

var (
	unoccupiedUtxoLock *sync.RWMutex = new(sync.RWMutex)
	saveUtxoLock       *sync.RWMutex = new(sync.RWMutex)
)

func GetUnoccupiedUtxoList(limit int64, utxoType models.UtxoType) ([]*models.MarketUtxoModel, error) {
	var (
		redisKeyPrefix     string                    = ""
		sortIndexList      []int                     = make([]int, 0)
		utxoIdKeyList      []string                  = make([]string, 0)
		startIndex         int64                     = -1
		utxoList           []*models.MarketUtxoModel = make([]*models.MarketUtxoModel, 0)
		unoccupiedUtxoList []*models.MarketUtxoModel = make([]*models.MarketUtxoModel, 0)
		confirmStatus                                = models.ConfirmStatus(-1)
	)
	switch utxoType {
	case models.UtxoTypeDummy600:
		redisKeyPrefix = fmt.Sprintf("%s%s", redis.CacheGetUtxo_, redis.UtxoTypeDummy_)
		confirmStatus = models.Confirmed
		break
	case models.UtxoTypeDummy1200:
		redisKeyPrefix = fmt.Sprintf("%s%s", redis.CacheGetUtxo_, redis.UtxoTypeDummy1200_)
		confirmStatus = models.Confirmed
		break
	default:
		return nil, errors.New("Unoccupied-Utxo: wrong type")
	}
	unoccupiedUtxoLock.RLock()
	defer unoccupiedUtxoLock.RUnlock()

	utxoIdKeyList, sortIndexList, _ = redis.GetUtxoInfoKeyValueList(redisKeyPrefix)
	for _, v := range sortIndexList {
		if startIndex == -1 {
			startIndex = int64(v)
		} else if startIndex > int64(v) {
			startIndex = int64(v)
		}
	}
	fmt.Printf("Get utxoIdKeyList: %+v\n", utxoIdKeyList)
	fmt.Printf("Get sortIndexList: %+v\n", sortIndexList)

	//utxoList, _ = mongo_service.FindUtxoList(net, startIndex, maxLimit, perAmount, utxoType, confirmStatus, fromOrderId, networkFeeRate)

	utxoList, _ = models.MarketUtxoModelDao().GetDummyList(&models.MarketUtxoModel{
		ConfirmStatus: confirmStatus,
		UtxoType:      utxoType,
		UsedState:     models.UsedNo,
	}, 0, maxLimit)
	if len(utxoList) == 0 {
		return nil, errors.New("Unoccupied-Utxo: Empty utxo list. Please contact customer service and wait for the platform to add UTXO. ")
	}
	for _, v := range utxoList {
		has := false
		for _, utxoId := range utxoIdKeyList {
			if utxoId == v.UtxoId {
				has = true
				break
			}
		}
		if has {
			continue
		}
		unoccupiedUtxoList = append(unoccupiedUtxoList, v)
	}
	if int64(len(unoccupiedUtxoList)) < limit {
		fmt.Printf("Unoccupied-Utxo[%d]: Not enough - have[%d], need[%d]", utxoType, len(unoccupiedUtxoList), limit)
		return nil, errors.New(fmt.Sprintf("Unoccupied-Utxo[%d]: Not enough - have[%d], need[%d]. Please contact customer service and wait for the platform to add UTXO. ", utxoType, len(unoccupiedUtxoList), limit))
	}
	unoccupiedUtxoList = unoccupiedUtxoList[:limit]
	for _, v := range unoccupiedUtxoList {
		addr, err := btcutil.DecodeAddress(v.Address, GetNetParams(conf.Net))
		if err != nil {
			return nil, err
		}
		pkScriptByte, err := txscript.PayToAddrScript(addr)
		if err != nil {
			return nil, err
		}
		v.PkScript = hex.EncodeToString(pkScriptByte)
	}

	cacheUtxoList(unoccupiedUtxoList)
	return unoccupiedUtxoList, nil
}

func ReleaseUtxoList(utxoList []*models.MarketUtxoModel) {
	for _, v := range utxoList {
		cacheUtxoType := redis.UtxoTypeDummy_
		switch v.UtxoType {
		case models.UtxoTypeDummy600:
			cacheUtxoType = redis.UtxoTypeDummy_
			break
		case models.UtxoTypeDummy1200:
			cacheUtxoType = redis.UtxoTypeDummy1200_
			break
		default:
			continue
		}
		err := redis.UnSetUtxoInfo(cacheUtxoType, v.UtxoId)
		if err != nil {
			fmt.Printf("UnSetUtxoInfo err:%s\n", err.Error())
		}
	}
}

func cacheUtxoList(utxoList []*models.MarketUtxoModel) {
	for _, v := range utxoList {
		cacheUtxoType := redis.UtxoTypeDummy_
		switch v.UtxoType {
		case models.UtxoTypeDummy600:
			cacheUtxoType = redis.UtxoTypeDummy_
			break
		case models.UtxoTypeDummy1200:
			cacheUtxoType = redis.UtxoTypeDummy1200_
			break
		default:
			continue
		}
		_, err := redis.SetRedisUtxoInfo(cacheUtxoType, v.UtxoId, int(v.SortIndex))
		if err != nil {
			fmt.Printf("SetRedisUtxoInfo err:%s\n", err.Error())
		}
	}
}

//func GetSaveStartIndex(net string, utxoType models.UtxoType, perAmount int64) int64 {
//	saveUtxoLock.RLock()
//	t1 := tool.MakeTimestamp()
//	fmt.Println("[LOCK]-Save-utxo")
//	defer func() {
//		saveUtxoLock.RUnlock()
//		fmt.Printf("[UNLOCK]-Save-utxo-timeConsuming:%d\n", tool.MakeTimestamp()-t1)
//	}()
//	startIndex := int64(0)
//	latestUtxo, _ := mongo_service.GetLatestStartIndexUtxo(net, utxoType, perAmount)
//	if latestUtxo != nil {
//		startIndex = latestUtxo.SortIndex
//	}
//	return startIndex
//}
//
//func GetUtxoInfo(net, txId string, txIndex int64) *own_service.UtxoInfo {
//	var (
//		infoMap   map[string]*own_service.UtxoInfo
//		outPoints []string = []string{
//			fmt.Sprintf("%s:%d", txId, txIndex),
//		}
//	)
//	infoMap, err := own_service.CheckUtxoInfo(net, outPoints)
//	if err != nil {
//		return nil
//	}
//	if len(infoMap) == 0 {
//		return nil
//	}
//	if _, ok := infoMap[fmt.Sprintf("%s:%d", txId, txIndex)]; !ok {
//		return nil
//	}
//	return infoMap[fmt.Sprintf("%s:%d", txId, txIndex)]
//}

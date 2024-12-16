package common

import (
	"crypto/sha256"
	"encoding/hex"
	"metaid-market-service/conf"
	"metaid-market-service/controller/respond"
	"metaid-market-service/service/man_service"
)

func GetMetaIdFlag() string {
	if conf.Net == "testnet" {
		//return "testid"
		return "metaid"
	}
	return "metaid"
}

func FetchMetaIDUserInfo(address string) *respond.UserInfo {
	var (
		info *man_service.MetaIDUserInfo
	)
	info, _ = man_service.FetchMetaIDUserInfoInfo(conf.Net, address)
	if info == nil {
		return nil
	}
	return &respond.UserInfo{
		Name:   info.Name,
		Avatar: info.Avatar,
	}
}

func GenPopSummary(pop string) string {
	var (
		extractCount int    = conf.PopExtractCount
		extractStr   string = ""
		popSummary   string = ""
	)
	if len(pop) <= extractCount {
		return pop
	}
	extractStr = pop[:extractCount]
	popSummary = pop[extractCount:]
	for _, v := range extractStr {
		if v != '0' {
			return "--"
		}
	}
	if len(popSummary) > 14 {
		popSummary = popSummary[:14]
	}
	return popSummary
}

func GetMetaIdByAddress(address string) (metaId string) {
	hash := sha256.New()
	hash.Write([]byte(address))
	hashBytes := hash.Sum(nil)
	metaId = hex.EncodeToString(hashBytes)
	return
}

package common

import "strings"

// ID Coins issued through the retired orders.exchange partner. Identification
// used to hit the partner's coins-info API; since the partner shut down, it now
// falls back to this fixed snapshot (taken from the partner API on 2026-10-22,
// 32 coins in total). Coins deployed after conf.IdCoinsSignTimestamp keep being
// identified by the local tick-signature check in CheckIdCoins.
var idCoins = []struct {
	Tick   string
	TickId string
}{
	{"MetaWeb", "8652ad30333a1493c5b71bfba1f886e40bc7e66712a94a6147b345888e8b6bdai0"},
	{"Solty", "03955931a7ee393d0bda84f2373394c961435d924b2a6dfa83a04b0dd5d23fe8i0"},
	{"ALLIN", "064849ac21b645be10a92c3e3548ee49ec70a4f07518ca5b2d0abd6e87229823i0"},
	{"JOKES", "380b00054fec8c0f86d4f1e3c610331f150f192f32852d3e503e19a9daa5ee3di0"},
	{"SB", "51bef1812749aefdb5014cbcab6af90dfcd8df3433cf7c2cdf1d359c034af63ai0"},
	{"IDCOIN", "b87d217618b6089cc1bcf97f77435fab3a90eaf9995a5179e13ec07c7a6783dci0"},
	{"MCPer", "d8cae9d01a316dd4203cb03983f5c3b75271dc98043ed2ef05e97ad8a1bc988ei0"},
	{"a1", "3b8ab78e9cb60a76e222e645bc21bd210c64a2c55957d0feb9c6e316b2d1619ai0"},
	{"goudddd", "75edf141dac47c51ff9ab6aaadbaaeb33c4260bd0d5220a09e9a4a7344543049i0"},
	{"MOG", "ca13c4cc9a2dce56697c7e810555155976184a952d1c52ee7330bb33cd0218a5i0"},
	{"master", "52e30817db2e49054692d2f518293fb619bbb1f65fdc51e12e0eddee7a833969i0"},
	{"NB", "53fa15e55134849cee8f783be3dc1b75e746fe7f0e6da606c858c9340a769d50i0"},
	{"Jfifj", "1f5686fba67ef7de8be14228c8d14412ee6d4a6c2a3eb95eef5249b66608ed5ei0"},
	{"DeepSeek", "79a89376045b6a544e09ec75c1ebe1982b620c045e3c4bf9e8f20811c7dcff93i0"},
	{"PPMD", "e1d2988a4de708ba00d213a9781937561e87d9c1a5101d1617e5830a21cf9ffci0"},
	{"PIZZA", "58ce650d491a0d064df236611d6fe57abacd433a05c804ffffcb2b642a20f9bci0"},
	{"Law", "e155086e29bcf9f6c1b9677bfbda125968635f49ec068ca92545e5e8e572f54di0"},
	{"Husky", "36324761c93047e997c9bfc400b2f4f494c4b8e4d31a4dce46f9a91c839d7e09i0"},
	{"DaddyYang", "abc9f88ffe8932d359010d2b9d78aa107aa67beb531ec350ca8dbdb4dd5656c0i0"},
	{"Trump", "451c8914c982b2207eedbc8cc81f35e7f2e3c86c7a7d89f4d9bc76a6a99c50b3i0"},
	{"Aix-Winer", "6d21fd3d0dfff3fd0addb35cc66a0d41d304be115df36267b0a837fc3909f22ai0"},
	{"Metalet-Offical", "b192a0e8f79db3600bfad9ecc0a41bac19411c17fc55ca2c5f3a3fcf6d3449aci0"},
	{"burn", "3e0c368c32c6c9c01ad24d430a13fb1aaf1d8b4796d1691884059931c48fe1c6i0"},
	{"world", "a4ceedfbfa5389d1886a021aedf001cb4baa02a46492d1d5b1374b35a13de736i0"},
	{"INok", "8cf737ada879ae31d165890c8f34b7b1ab6b45a6bbd9deca043368d8a95a6adci0"},
	{"Satoshi", "ee0f88341b1fdb5dbe21e72afb5552b57a249310c80e3542d161a57ea972dc98i0"},
	{"SATS", "070007a28d02f2bc5f21b2c2bee5d6a90570d2b0c09adf3c78491d772ed9fbefi0"},
	{"ShaBi", "10f6968c183e2adfce72db327710f6b43edbd749c792c87b964f55df21d4f17ei0"},
	{"Ocean", "762632e1771d20bba386025c5f33e4362af6e9477e700d21ea265aeb510f9b58i0"},
	{"buzz", "83288c76bbca7c6a3fb72931c0c0c7bc14bc8e71b931e7ac8ddac19d1983e7ebi0"},
	{"Sunny", "6acbfc5b8506a319e0848a1fc802d273b4f313681e195ab43c2a8a6d86dc8184i0"},
	{"METAID", "8ce590918a3d493631ab9d9e3bbc89e322f3dd08354af87f97c07218818b78f4i0"},
}

var (
	idCoinsTickIdSet = makeIdCoinsTickIdSet()
	idCoinsTickNames = makeIdCoinsTickNames()
)

func makeIdCoinsTickIdSet() map[string]struct{} {
	set := make(map[string]struct{}, len(idCoins))
	for _, coin := range idCoins {
		set[coin.TickId] = struct{}{}
	}
	return set
}

func makeIdCoinsTickNames() map[string]struct{} {
	set := make(map[string]struct{}, len(idCoins))
	for _, coin := range idCoins {
		set[strings.ToUpper(coin.Tick)] = struct{}{}
	}
	return set
}

func IsIdCoinsTickId(tickId string) bool {
	_, ok := idCoinsTickIdSet[tickId]
	return ok
}

func IsIdCoinsTick(tick string) bool {
	_, ok := idCoinsTickNames[strings.ToUpper(tick)]
	return ok
}

func IdCoinsTickIdList() []string {
	tickIds := make([]string, 0, len(idCoins))
	for _, coin := range idCoins {
		tickIds = append(tickIds, coin.TickId)
	}
	return tickIds
}

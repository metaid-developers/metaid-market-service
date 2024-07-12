package wallet_service

import (
	"fmt"
	"testing"
)

func TestFetchBrc20PoolKey(t *testing.T) {
	//conf.InitDevConfig()
	var (
		host string = ""
		tick string = "dddd"
		//tick     string            = "xxxx"
		protocol string            = "brc20"
		headers  map[string]string = nil
	)
	res, err := FetchPoolKey(host, tick, protocol, headers)
	if err != nil {
		t.Errorf("FetchBrc20PoolKey() error = %v", err)
		return
	}
	fmt.Printf("res = %v\n", res)
	//segwitAddress1, err := create_key.GetSegwitAddressFromPublicKey(common.GetNetParams(net), res.Btc)
	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}
	//segwitAddress2, err := create_key.GetSegwitAddressFromPublicKey(common.GetNetParams(net), res.Protocol)
	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}
	//fmt.Printf("segwitAddress1 = %v\n", segwitAddress1)
	//tb1qzpvvfjsdvtcsq8zq6pfzkckc3xhfer87sjwx6l
	//tb1pzhd6849xw0akf58sv4mh9tlslxcp23rmtp49r0gl6ad0hjp2sn5qzj3a2a
	//fmt.Printf("segwitAddress2 = %v\n", segwitAddress2)
	//tb1q855q58757gcxfnw48n36cca925r4rnhjjaytnx
	//tb1pyd70e879wwwkfehwdp5z9mm9qj48x0tqlxdh5xk73ew6c7cz9zmqswvv2n
}

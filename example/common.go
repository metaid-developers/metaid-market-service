package example

import (
	"encoding/hex"
	"encoding/json"
	"log"
	"metaid-market-service/controller/auth"
	"metaid-market-service/tool"
	"os"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
)

var domain = os.Getenv("METAID_MARKET_EXAMPLE_BASE_URL")

func requireIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("METAID_MARKET_EXAMPLE_INTEGRATION") != "1" {
		t.Skip("network examples are disabled; set METAID_MARKET_EXAMPLE_INTEGRATION=1 to opt in")
	}
	if domain == "" {
		t.Fatal("METAID_MARKET_EXAMPLE_BASE_URL is required")
	}
}

func examplePrivateKey(role string) string {
	return os.Getenv("METAID_MARKET_EXAMPLE_PRIVATE_KEY_" + role)
}

func addHeaders(priKey string) map[string]string {
	privateKeyBytes, err := hex.DecodeString(priKey)
	if err != nil {
		log.Fatal(err)
	}
	privateKey, _ := btcec.PrivKeyFromBytes(privateKeyBytes)
	publicKey := hex.EncodeToString(privateKey.PubKey().SerializeCompressed())
	result, err := tool.PostUrl(domain+"/api/v1/auth/challenge", map[string]string{"publicKey": publicKey}, nil)
	if err != nil {
		log.Fatal(err)
	}
	var response struct {
		Code int `json:"code"`
		Data struct {
			Message string `json:"message"`
			Nonce   string `json:"nonce"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(result), &response); err != nil || response.Code != 0 {
		log.Fatalf("auth challenge failed: %s", result)
	}
	signature, err := auth.SignTextMessage(response.Data.Message, priKey)
	if err != nil {
		log.Fatal(err)
	}
	loginResult, err := tool.PostUrl(domain+"/api/v1/auth/login", map[string]string{
		"publicKey": publicKey,
		"signature": signature,
		"nonce":     response.Data.Nonce,
	}, nil)
	if err != nil {
		log.Fatal(err)
	}
	var loginResponse struct {
		Code int `json:"code"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(loginResult), &loginResponse); err != nil || loginResponse.Code != 0 {
		log.Fatalf("auth login failed: %s", loginResult)
	}
	return map[string]string{"X-Session": loginResponse.Data.Token}
}


func adminHeaders(priKey string) map[string]string {
	headers := addHeaders(priKey)
	headers["Authorization"] = "Bearer " + os.Getenv("METAID_MARKET_ADMIN_API_KEY")
	return headers
}

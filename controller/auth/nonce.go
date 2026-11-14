package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"metaid-market-service/redis"
	"metaid-market-service/tool"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
)

const nonceTTLSeconds = 300

type AuthChallenge struct {
	Message   string `json:"message"`
	Nonce     string `json:"nonce"`
	ExpiresAt int64  `json:"expiresAt"`
}

type NonceRepository interface {
	Save(challenge *AuthChallenge) error
	Consume(nonce string) (*AuthChallenge, error)
}

type redisNonceRepository struct{}

func (redisNonceRepository) Save(challenge *AuthChallenge) error {
	_, err := redis.GetRedisManager().Set(redis.DbUtxo(), "auth_nonce_"+challenge.Nonce, challenge, nonceTTLSeconds)
	return err
}

func (redisNonceRepository) Consume(nonce string) (*AuthChallenge, error) {
	value, err := redis.GetRedisManager().Consume(redis.DbUtxo(), "auth_nonce_"+nonce)
	if err != nil {
		return nil, err
	}
	bytes, ok := value.([]byte)
	if !ok || len(bytes) == 0 {
		return nil, errors.New("nonce not found")
	}
	var challenge AuthChallenge
	if err := json.Unmarshal(bytes, &challenge); err != nil {
		return nil, err
	}
	return &challenge, nil
}

func CreateChallenge(publicKey string) (*AuthChallenge, error) {
	return CreateChallengeWithRepository(publicKey, redisNonceRepository{})
}

func CreateChallengeWithRepository(publicKey string, repository NonceRepository) (*AuthChallenge, error) {
	publicKeyBytes, err := hex.DecodeString(publicKey)
	if err != nil {
		return nil, err
	}
	if _, err := btcec.ParsePubKey(publicKeyBytes); err != nil {
		return nil, err
	}
	nonceBytes := make([]byte, 24)
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, err
	}
	challenge := &AuthChallenge{
		Nonce:     hex.EncodeToString(nonceBytes),
		ExpiresAt: tool.MakeTimestamp() + nonceTTLSeconds*1000,
	}
	challenge.Message = fmt.Sprintf("metaid.market\naction:POST /api/v1/auth/login\nnonce:%s\nexpiresAt:%d", challenge.Nonce, challenge.ExpiresAt)
	if repository == nil {
		return nil, errors.New("nonce repository is required")
	}
	if err := repository.Save(challenge); err != nil {
		return nil, err
	}
	return challenge, nil
}

func consumeChallenge(repository NonceRepository, nonce string) (*AuthChallenge, error) {
	if repository == nil {
		return nil, errors.New("nonce repository is required")
	}
	challenge, err := repository.Consume(nonce)
	if err != nil {
		return nil, err
	}
	if challenge == nil || challenge.ExpiresAt < time.Now().UnixMilli() {
		return nil, errors.New("nonce is expired")
	}
	return challenge, nil
}

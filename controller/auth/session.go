package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"metaid-market-service/redis"
	"metaid-market-service/tool"
)

const sessionTTLSeconds = 3600

type AuthSession struct {
	Token     string `json:"token"`
	PublicKey string `json:"publicKey"`
	ExpiresAt int64  `json:"expiresAt"`
}

type SessionRepository interface {
	Create(session *AuthSession) error
	Get(token string) (*AuthSession, error)
	Delete(token string) error
}

type LoginRepository interface {
	NonceRepository
	SessionRepository
}

type redisSessionRepository struct{}

func (redisSessionRepository) Create(session *AuthSession) error {
	_, err := redis.GetRedisManager().Set(redis.DbUtxo(), "auth_session_"+session.Token, session, sessionTTLSeconds)
	return err
}

func (redisSessionRepository) Get(token string) (*AuthSession, error) {
	value, err := redis.GetRedisManager().Get(redis.DbUtxo(), "auth_session_"+token)
	if err != nil {
		return nil, err
	}
	bytes, ok := value.([]byte)
	if !ok || len(bytes) == 0 {
		return nil, errors.New("session not found")
	}
	var session AuthSession
	if err := json.Unmarshal(bytes, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (redisSessionRepository) Delete(token string) error {
	_, err := redis.GetRedisManager().Delete(redis.DbUtxo(), "auth_session_"+token)
	return err
}

type redisLoginRepository struct {
	redisNonceRepository
	redisSessionRepository
}

func LoginWithRepository(publicKey, signature, nonce string, repository LoginRepository) (*AuthSession, error) {
	challenge, err := consumeChallenge(repository, nonce)
	if err != nil {
		return nil, err
	}
	verified, err := VerifyTextSign(challenge.Message, signature, publicKey)
	if err != nil || !verified {
		return nil, errors.New("login signature verification failed")
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	session := &AuthSession{
		Token:     hex.EncodeToString(tokenBytes),
		PublicKey: publicKey,
		ExpiresAt: tool.MakeTimestamp() + sessionTTLSeconds*1000,
	}
	if err := repository.Create(session); err != nil {
		return nil, err
	}
	return session, nil
}

func ValidateSession(token string, repository SessionRepository) (*AuthSession, error) {
	if token == "" {
		return nil, errors.New("session is empty")
	}
	session, err := repository.Get(token)
	if err != nil {
		return nil, err
	}
	if session == nil || session.ExpiresAt < tool.MakeTimestamp() {
		return nil, errors.New("session is expired")
	}
	return session, nil
}

func RevokeSession(token string, repository SessionRepository) error {
	if token == "" {
		return nil
	}
	return repository.Delete(token)
}


func Login(publicKey, signature, nonce string) (*AuthSession, error) {
	return LoginWithRepository(publicKey, signature, nonce, redisLoginRepository{})
}

func Logout(token string) error {
	return RevokeSession(token, redisSessionRepository{})
}

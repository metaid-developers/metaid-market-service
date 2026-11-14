package auth

import (
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/gin-gonic/gin"
)

type memoryAuthRepository struct {
	nonces   map[string]*AuthChallenge
	sessions map[string]*AuthSession
}

func newMemoryAuthRepository() *memoryAuthRepository {
	return &memoryAuthRepository{nonces: make(map[string]*AuthChallenge), sessions: make(map[string]*AuthSession)}
}

func (r *memoryAuthRepository) Save(challenge *AuthChallenge) error {
	r.nonces[challenge.Nonce] = challenge
	return nil
}

func (r *memoryAuthRepository) Consume(nonce string) (*AuthChallenge, error) {
	challenge, ok := r.nonces[nonce]
	if !ok {
		return nil, errors.New("nonce not found")
	}
	delete(r.nonces, nonce)
	return challenge, nil
}

func (r *memoryAuthRepository) Create(session *AuthSession) error {
	r.sessions[session.Token] = session
	return nil
}

func (r *memoryAuthRepository) Get(token string) (*AuthSession, error) {
	session, ok := r.sessions[token]
	if !ok {
		return nil, errors.New("session not found")
	}
	return session, nil
}

func (r *memoryAuthRepository) Delete(token string) error {
	delete(r.sessions, token)
	return nil
}

func TestLoginNonceIsSingleUseAndSessionExpires(t *testing.T) {
	repository := newMemoryAuthRepository()
	privateKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	publicKey := hex.EncodeToString(privateKey.PubKey().SerializeCompressed())
	privateKeyHex := hex.EncodeToString(privateKey.Serialize())
	challenge, err := CreateChallengeWithRepository(publicKey, repository)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := SignTextMessage(challenge.Message, privateKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	session, err := LoginWithRepository(publicKey, signature, challenge.Nonce, repository)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if _, err := LoginWithRepository(publicKey, signature, challenge.Nonce, repository); err == nil {
		t.Fatal("nonce replay was accepted")
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(AuthSignMiddlewareWithRepository(repository))
	router.GET("/protected", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("X-Session", session.Token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("valid session rejected: %d", recorder.Code)
	}

	session.ExpiresAt = 0
	request = httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("X-Session", session.Token)
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expired session accepted: %d", recorder.Code)
	}
}

func TestLoginRejectsWrongSignature(t *testing.T) {
	repository := newMemoryAuthRepository()
	privateKey, _ := btcec.NewPrivateKey()
	otherKey, _ := btcec.NewPrivateKey()
	publicKey := hex.EncodeToString(privateKey.PubKey().SerializeCompressed())
	challenge, err := CreateChallengeWithRepository(publicKey, repository)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := SignTextMessage(challenge.Message, hex.EncodeToString(otherKey.Serialize()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoginWithRepository(publicKey, signature, challenge.Nonce, repository); err == nil {
		t.Fatal("signature from another key was accepted")
	}
}

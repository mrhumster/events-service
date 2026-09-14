package service_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mrhumster/events-service/config"
	"github.com/mrhumster/events-service/internal/service"
	"github.com/mrhumster/identity-service/pkg/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mustRSAKey returns a fresh 2048-bit key (fast enough for tests).
func mustRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func signAccessToken(t *testing.T, key *rsa.PrivateKey, userID, role string) string {
	t.Helper()
	claims := &dto.AccessClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "identity-service",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(key)
	require.NoError(t, err)
	return signed
}

func newTokenService(t *testing.T, publicKey *rsa.PublicKey) *service.TokenService {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-pem-file")
		pkix, err := x509.MarshalPKIXPublicKey(publicKey)
		require.NoError(t, err)
		pemKey := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pkix})
		_, _ = w.Write(pemKey)
	}))
	t.Cleanup(srv.Close)

	ts, err := service.NewTokenService(&config.JWT{AccessPublicKeyURL: srv.URL})
	require.NoError(t, err)
	return ts
}

func TestTokenService_ValidateAccessToken(t *testing.T) {
	key := mustRSAKey(t)
	ts := newTokenService(t, &key.PublicKey)

	userID := "f2e7c3f4-56c9-4a1d-9b3c-8c5d6e7f8a90"
	token := signAccessToken(t, key, userID, "member")

	claims, err := ts.ValidateAccessToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, "member", claims.Role)
}

func TestTokenService_RejectsBadlySignedToken(t *testing.T) {
	key := mustRSAKey(t)
	other := mustRSAKey(t)
	ts := newTokenService(t, &key.PublicKey)

	token := signAccessToken(t, other, "some-user", "member")
	_, err := ts.ValidateAccessToken(token)
	assert.Error(t, err)
}

func TestTokenService_RejectsGarbage(t *testing.T) {
	key := mustRSAKey(t)
	ts := newTokenService(t, &key.PublicKey)

	_, err := ts.ValidateAccessToken("not.a.token")
	assert.Error(t, err)
}

func TestTokenService_ErrorWhenPublicKeyURLMissing(t *testing.T) {
	_, err := service.NewTokenService(&config.JWT{})
	assert.Error(t, err)
}
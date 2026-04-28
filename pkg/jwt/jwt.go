package jwt

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

type TokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type Claims struct {
	UserID    int64  `json:"userId"`
	TokenType string `json:"tokenType,omitempty"`
	jwtlib.RegisteredClaims
}

type JWTManager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewJWTManager(secret string, a, r time.Duration) *JWTManager {
	return &JWTManager{secret: []byte(secret), accessTTL: a, refreshTTL: r}
}

func (j *JWTManager) RefreshTTL() time.Duration { return j.refreshTTL }

func (j *JWTManager) GenerateTokenPair(uid int64) (*TokenPair, error) {
	a, _, err := j.generate(uid, j.accessTTL, TokenTypeAccess)
	if err != nil {
		return nil, err
	}
	r, _, err := j.generate(uid, j.refreshTTL, TokenTypeRefresh)
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: a, RefreshToken: r}, nil
}

func (j *JWTManager) GenerateTokenPairWithJTI(uid int64, refreshJTI string) (*TokenPair, error) {
	a, _, err := j.generate(uid, j.accessTTL, TokenTypeAccess)
	if err != nil {
		return nil, err
	}
	r, _, err := j.generateWithJTI(uid, j.refreshTTL, TokenTypeRefresh, refreshJTI)
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: a, RefreshToken: r}, nil
}

func NewJTI() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func (j *JWTManager) generate(uid int64, ttl time.Duration, tokenType string) (string, string, error) {
	jti, err := NewJTI()
	if err != nil {
		return "", "", err
	}
	s, err := j.generateWithClaims(uid, ttl, tokenType, jti)
	return s, jti, err
}

func (j *JWTManager) generateWithJTI(uid int64, ttl time.Duration, tokenType, jti string) (string, string, error) {
	s, err := j.generateWithClaims(uid, ttl, tokenType, jti)
	return s, jti, err
}

func (j *JWTManager) generateWithClaims(uid int64, ttl time.Duration, tokenType, jti string) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:    uid,
		TokenType: tokenType,
		RegisteredClaims: jwtlib.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwtlib.NewNumericDate(now),
			ExpiresAt: jwtlib.NewNumericDate(now.Add(ttl)),
		},
	}
	t := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	return t.SignedString(j.secret)
}

func (j *JWTManager) Parse(tok string) (*Claims, error) {
	t, err := jwtlib.ParseWithClaims(tok, &Claims{}, func(t *jwtlib.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return j.secret, nil
	})
	if err != nil {
		return nil, err
	}
	c, ok := t.Claims.(*Claims)
	if !ok || !t.Valid {
		return nil, jwtlib.ErrTokenInvalidClaims
	}
	return c, nil
}

func (j *JWTManager) ParseWithType(tok, expected string) (*Claims, error) {
	c, err := j.Parse(tok)
	if err != nil {
		return nil, err
	}
	if c.TokenType == "" {
		if expected == TokenTypeAccess {
			return c, nil
		}
		return nil, errors.New("TOKEN_TYPE_MISMATCH")
	}
	if c.TokenType != expected {
		return nil, errors.New("TOKEN_TYPE_MISMATCH")
	}
	return c, nil
}

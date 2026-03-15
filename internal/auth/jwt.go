package auth

import (
    "time"
    "github.com/golang-jwt/jwt/v5"
)

type TokenPair struct {
    AccessToken string `json:"accessToken"`
    RefreshToken string `json:"refreshToken"`
}

type Claims struct {
    UserID int64 `json:"userId"`
    jwt.RegisteredClaims
}

type JWTManager struct {
    secret []byte
    accessTTL time.Duration
    refreshTTL time.Duration
}

func NewJWTManager(secret string, a, r time.Duration) *JWTManager {
    return &JWTManager{secret: []byte(secret), accessTTL: a, refreshTTL: r}
}

func (j *JWTManager) GenerateTokenPair(uid int64) (*TokenPair, error) {
    a, err := j.generate(uid, j.accessTTL)
    if err != nil { return nil, err }
    r, err := j.generate(uid, j.refreshTTL)
    if err != nil { return nil, err }
    return &TokenPair{a, r}, nil
}

func (j *JWTManager) generate(uid int64, ttl time.Duration) (string, error) {
    now := time.Now()
    claims := &Claims{
        UserID: uid,
        RegisteredClaims: jwt.RegisteredClaims{
            IssuedAt: jwt.NewNumericDate(now),
            ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
        },
    }
    t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return t.SignedString(j.secret)
}

func (j *JWTManager) Parse(tok string) (*Claims, error) {
    t, err := jwt.ParseWithClaims(tok, &Claims{}, func(t *jwt.Token)(interface{}, error){
        return j.secret, nil
    })
    if err != nil { return nil, err }
    if c, ok := t.Claims.(*Claims); ok && t.Valid { return c, nil }
    return nil, jwt.ErrTokenInvalidClaims
}

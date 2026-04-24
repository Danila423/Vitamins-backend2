package jwt

import (
	"strings"
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

func TestJWTManager_RoundTripAccessToken(t *testing.T) {
	mgr := NewJWTManager("secret-jwt-test", time.Minute, time.Hour)

	pair, err := mgr.GenerateTokenPair(42)
	if err != nil {
		t.Fatalf("generate pair: %v", err)
	}

	claims, err := mgr.ParseWithType(pair.AccessToken, TokenTypeAccess)
	if err != nil {
		t.Fatalf("parse access: %v", err)
	}
	if claims.UserID != 42 {
		t.Fatalf("uid: want 42, got %d", claims.UserID)
	}
	if claims.TokenType != TokenTypeAccess {
		t.Fatalf("token type: want %s, got %s", TokenTypeAccess, claims.TokenType)
	}
	if claims.ID == "" {
		t.Fatalf("expected jti to be populated")
	}
}

func TestJWTManager_AccessTokenRejectedAsRefresh(t *testing.T) {
	mgr := NewJWTManager("secret-jwt-test", time.Minute, time.Hour)
	pair, err := mgr.GenerateTokenPair(7)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := mgr.ParseWithType(pair.AccessToken, TokenTypeRefresh); err == nil {
		t.Fatalf("expected access token to be rejected when refresh expected")
	}
}

func TestJWTManager_RefreshTokenRejectedAsAccess(t *testing.T) {
	mgr := NewJWTManager("secret-jwt-test", time.Minute, time.Hour)
	pair, err := mgr.GenerateTokenPair(8)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := mgr.ParseWithType(pair.RefreshToken, TokenTypeAccess); err == nil {
		t.Fatalf("expected refresh token to be rejected when access expected")
	}
}

func TestJWTManager_LegacyTokenWithoutTypeAcceptedAsAccessOnly(t *testing.T) {
	mgr := NewJWTManager("secret-jwt-legacy", time.Minute, time.Hour)
	now := time.Now()
	legacyClaims := jwtlib.MapClaims{
		"userId": float64(11),
		"iat":    now.Unix(),
		"exp":    now.Add(time.Minute).Unix(),
	}
	tok := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, legacyClaims)
	signed, err := tok.SignedString([]byte("secret-jwt-legacy"))
	if err != nil {
		t.Fatalf("sign legacy: %v", err)
	}
	if _, err := mgr.ParseWithType(signed, TokenTypeAccess); err != nil {
		t.Fatalf("legacy must be accepted as access: %v", err)
	}
	if _, err := mgr.ParseWithType(signed, TokenTypeRefresh); err == nil {
		t.Fatalf("legacy must NOT be accepted as refresh")
	}
}

func TestJWTManager_RejectsNoneAlgorithm(t *testing.T) {
	mgr := NewJWTManager("doesnt-matter", time.Minute, time.Hour)

	header := `{"alg":"none","typ":"JWT"}`
	payload := `{"userId":99,"exp":` + itoa(time.Now().Add(time.Hour).Unix()) + `}`
	tok := b64url(header) + "." + b64url(payload) + "."

	if _, err := mgr.Parse(tok); err == nil {
		t.Fatalf("alg=none must be rejected")
	}
}

func TestJWTManager_RejectsWrongSecret(t *testing.T) {
	mgrA := NewJWTManager("secret-A", time.Minute, time.Hour)
	mgrB := NewJWTManager("secret-B", time.Minute, time.Hour)
	pair, err := mgrA.GenerateTokenPair(1)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := mgrB.Parse(pair.AccessToken); err == nil {
		t.Fatalf("token signed with secret-A must not parse under secret-B")
	}
}

func TestJWTManager_RejectsExpiredToken(t *testing.T) {
	mgr := NewJWTManager("secret-jwt-exp", -time.Minute, time.Hour)
	pair, err := mgr.GenerateTokenPair(1)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := mgr.Parse(pair.AccessToken); err == nil {
		t.Fatalf("expired access token must be rejected")
	}
}

func TestJWTManager_GenerateTokenPairWithJTIPreservesID(t *testing.T) {
	mgr := NewJWTManager("secret-jti", time.Minute, time.Hour)
	const jti = "deadbeefdeadbeefdeadbeefdeadbeef"
	pair, err := mgr.GenerateTokenPairWithJTI(5, jti)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	c, err := mgr.ParseWithType(pair.RefreshToken, TokenTypeRefresh)
	if err != nil {
		t.Fatalf("parse refresh: %v", err)
	}
	if c.ID != jti {
		t.Fatalf("jti mismatch: want %s, got %s", jti, c.ID)
	}
}

// helpers (intentionally tiny — avoid pulling extra deps in jwt unit tests)

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func b64url(s string) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	b := []byte(s)
	var out strings.Builder
	for i := 0; i < len(b); i += 3 {
		end := i + 3
		if end > len(b) {
			end = len(b)
		}
		chunk := b[i:end]
		var n uint32
		for j := 0; j < 3; j++ {
			n <<= 8
			if j < len(chunk) {
				n |= uint32(chunk[j])
			}
		}
		out.WriteByte(alphabet[(n>>18)&0x3F])
		out.WriteByte(alphabet[(n>>12)&0x3F])
		if len(chunk) > 1 {
			out.WriteByte(alphabet[(n>>6)&0x3F])
		}
		if len(chunk) > 2 {
			out.WriteByte(alphabet[n&0x3F])
		}
	}
	return out.String()
}

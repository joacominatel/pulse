package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// supabase jwt claims structure
// based on supabase auth token format
type SupabaseClaims struct {
	jwt.RegisteredClaims

	// aal is the authenticator assurance level
	AAL string `json:"aal,omitempty"`

	// role is the user's role (e.g., "authenticated", "anon")
	Role string `json:"role,omitempty"`

	// session_id is the unique session identifier
	SessionID string `json:"session_id,omitempty"`

	// email is the user's email address
	Email string `json:"email,omitempty"`

	// email_verified indicates if email is confirmed
	EmailVerified bool `json:"email_verified,omitempty"`

	// phone is the user's phone number
	Phone string `json:"phone,omitempty"`

	// phone_verified indicates if phone is confirmed
	PhoneVerified bool `json:"phone_verified,omitempty"`

	// app_metadata contains application-specific metadata
	AppMetadata map[string]any `json:"app_metadata,omitempty"`

	// user_metadata contains user-editable metadata
	UserMetadata map[string]any `json:"user_metadata,omitempty"`

	// is_anonymous indicates if the user is anonymous
	IsAnonymous bool `json:"is_anonymous,omitempty"`
}

// UserID returns the subject claim (user's UUID in supabase)
func (c *SupabaseClaims) UserID() string {
	return c.Subject
}

// IsAuthenticated returns true if the user has a valid authenticated role
func (c *SupabaseClaims) IsAuthenticated() bool {
	return c.Role == "authenticated"
}

// JWTValidator validates supabase auth tokens
type JWTValidator struct {
	secret []byte
}

// NewJWTValidator creates a new validator with the supabase jwt secret
func NewJWTValidator(secret string) *JWTValidator {
	return &JWTValidator{
		secret: []byte(secret),
	}
}

// common jwt validation errors
var (
	ErrMissingToken     = errors.New("missing authorization token")
	ErrInvalidToken     = errors.New("invalid token format")
	ErrTokenExpired     = errors.New("token has expired")
	ErrInvalidSignature = errors.New("invalid token signature")
	ErrInvalidClaims    = errors.New("invalid token claims")
)

// ValidateToken parses and validates a supabase jwt token
// returns the claims if valid, or an error if validation fails
func (v *JWTValidator) ValidateToken(tokenString string) (*SupabaseClaims, error) {
	if tokenString == "" {
		return nil, ErrMissingToken
	}

	// strip "Bearer " prefix if present
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	tokenString = strings.TrimSpace(tokenString)

	if tokenString == "" {
		return nil, ErrMissingToken
	}

	claims := &SupabaseClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		// validate the signing method is HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return v.secret, nil
	})

	if err != nil {
		// check for specific jwt errors
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		if errors.Is(err, jwt.ErrSignatureInvalid) {
			return nil, ErrInvalidSignature
		}
		if errors.Is(err, jwt.ErrTokenMalformed) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidClaims, err)
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	// validate essential claims
	if claims.Subject == "" {
		return nil, fmt.Errorf("%w: missing subject claim", ErrInvalidClaims)
	}

	// check expiration manually as extra safety
	if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
		return nil, ErrTokenExpired
	}

	return claims, nil
}

// ExtractBearerToken extracts the token from an Authorization header value
func ExtractBearerToken(authHeader string) string {
	if authHeader == "" {
		return ""
	}
	// handle "Bearer <token>" format
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return authHeader
}

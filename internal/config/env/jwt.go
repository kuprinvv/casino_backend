package env

import (
	"casino_backend/internal/config"
	"fmt"
	"os"
	"time"
)

const (
	refreshTokenDurationEnvName = "REFRESH_TOKEN_DURATION"
	accessTokenKeyEnvName       = "ACCESS_TOKEN"
	accessTokenDurationEnvName  = "ACCESS_TOKEN_DURATION"
)

type jwtConfig struct {
	refreshTokenDuration time.Duration
	accessTokenSecretKey string
	accessTokenDuration  time.Duration
}

func NewJWTConfig() (config.JWTConfig, error) {
	accessToken := os.Getenv(accessTokenKeyEnvName)
	if len(accessToken) == 0 {
		return nil, fmt.Errorf("access token secret key not found")
	}

	refreshTokenDuration := os.Getenv(refreshTokenDurationEnvName)
	if len(refreshTokenDuration) == 0 {
		return nil, fmt.Errorf("refresh token duration not found")
	}

	refreshTokenDurationParsed, err := time.ParseDuration(refreshTokenDuration)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token duration: %w", err)
	}

	accessTokenDuration := os.Getenv(accessTokenDurationEnvName)
	if len(accessTokenDuration) == 0 {
		return nil, fmt.Errorf("access token duration not found")
	}

	accessTokenDurationParsed, err := time.ParseDuration(accessTokenDuration)
	if err != nil {
		return nil, fmt.Errorf("invalid access token duration: %w", err)
	}

	return &jwtConfig{
		accessTokenSecretKey: accessToken,
		refreshTokenDuration: refreshTokenDurationParsed,
		accessTokenDuration:  accessTokenDurationParsed,
	}, nil
}

func (j *jwtConfig) AccessTokenSecretKey() []byte {
	return []byte(j.accessTokenSecretKey)
}

func (j *jwtConfig) RefreshTokenDuration() time.Duration {
	return j.refreshTokenDuration
}

func (j *jwtConfig) AccessTokenDuration() time.Duration {
	return j.accessTokenDuration
}

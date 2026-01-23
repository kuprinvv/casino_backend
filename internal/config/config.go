package config

import "time"

type LineConfig interface {
	SymbolWeights() map[string]int
	WildChance() float64
	FreeSpinsByScatter() map[int]int
	PayoutTable() map[string]map[int]int
}

type CascadeConfig interface {
	SymbolWeights() map[int]int
	BonusProbPerColumn() float64
	BonusAwards() map[int]int
	PayoutTable() map[int]int
}

type HTTPConfig interface {
	Address() string
}

type PGConfig interface {
	DSN() string
}

type JWTConfig interface {
	AccessTokenSecretKey() []byte
	AccessTokenDuration() time.Duration
	RefreshTokenDuration() time.Duration
}

package auth_service

import (
	"fmt"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
)

// CreateToken from Claims to JWT
func CreateToken(claims jwt.MapClaims, secret string, duration int) (string, error) {
	now := time.Now()
	if duration != 0 {
		claims["exp"] = now.Add(time.Duration(duration) * time.Hour).Unix()
	} else {
		claims["exp"] = now.Add(time.Hour).Unix() // 1 hour
	}
	// create the token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// Sign and get the complete encoded token as string
	return token.SignedString([]byte(secret))
}

// CreatePartialToken from Claims to JWT
func CreatePartialToken(claims jwt.MapClaims, secret string, minute int) (string, error) {
	now := time.Now()
	if minute != 0 {
		claims["exp"] = now.Add(time.Duration(minute) * time.Minute).Unix()
	} else {
		claims["exp"] = now.Add(time.Minute).Unix() // 1 minute
	}
	// create the token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// Sign and get the complete encoded token as string
	return token.SignedString([]byte(secret))
}

func CreatePreAuthTokenWithStage(userId uint64, stage PreAuthTokenStages, secret string, exp int) (string, error) {
	claims := jwt.MapClaims{
		"key":   fmt.Sprintf("%d", userId),
		"stage": stage,
	}

	return CreatePartialToken(claims, secret, exp)
}

package manage_token

/**
 * Manage Token
 *
 * This package keeps all user access tokens in memory.
 * At every request received token is matched against this list and user ID it has associated
 *
 */
import (
	"fmt"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/net/redis"
)

var conn *redis.Client

// Start Redis for token management
func Start(redisCfg redis.Config) error {
	conn = redis.NewClient(redisCfg)
	err := conn.Connect()
	if err != nil {
		return err
	}
	return nil
}

func Close() {
	conn.Disconnect()
}

// ValidateToken - check JWT token exists in Redis
func ValidateToken(tokenString string, userID uint64) (int, error) {
	tokenExists := 0

	key := fmt.Sprintf("Token:%d", userID)
	err := conn.Exec(&tokenExists, "SISMEMBER", key, tokenString)
	return tokenExists, err
}

// RememberToken - save JWT token to Redis
func RememberToken(tokenString string, userID uint64) error {
	key := fmt.Sprintf("Token:%d", userID)
	err := conn.Exec(nil, "SADD", key, tokenString)
	if err != nil {
		return err
	}

	return nil
}

// RemoveToken - removes token from Redis
func RemoveToken(tokenString string, userID uint64) error {
	key := fmt.Sprintf("Token:%d", userID)
	err := conn.Exec(nil, "SREM", key, tokenString)
	if err != nil {
		return err
	}

	return nil
}

// RemoveAllUserTokens - removes all tokens for user from Redis
func RemoveAllUserTokens(userID uint64) error {
	key := fmt.Sprintf("Token:%d", userID)
	err := conn.Exec(nil, "DEL", key)
	if err != nil {
		return err
	}

	return nil
}

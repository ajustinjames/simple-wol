package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// APIToken represents a long-lived API token usable for automation/integrations.
// The raw token value is never stored — only its SHA-256 hash.
type APIToken struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	UserID     int64      `json:"user_id"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

// generateAPIToken returns a high-entropy random token string, prefixed for
// easy identification (e.g. in logs or revoked-token lists).
func generateAPIToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return "wol_" + base64.RawURLEncoding.EncodeToString(bytes), nil
}

// hashAPIToken returns the hex-encoded SHA-256 digest of a raw token value.
// SHA-256 (rather than bcrypt) is appropriate here: API tokens are
// high-entropy random values, not low-entropy user-chosen passwords, so a
// fast cryptographic hash is sufficient and avoids unnecessary CPU cost on
// every authenticated request.
func hashAPIToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CreateAPIToken generates a new token, stores its hash, and returns the
// raw token value (shown to the user once) along with its DB record.
func CreateAPIToken(db *sql.DB, userID int64, name string) (string, APIToken, error) {
	raw, err := generateAPIToken()
	if err != nil {
		return "", APIToken{}, err
	}
	hash := hashAPIToken(raw)

	result, err := db.Exec("INSERT INTO api_tokens (name, token_hash, user_id) VALUES (?, ?, ?)", name, hash, userID)
	if err != nil {
		return "", APIToken{}, fmt.Errorf("create api token: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return "", APIToken{}, fmt.Errorf("create api token: %w", err)
	}

	tok := APIToken{ID: id, Name: name, UserID: userID, CreatedAt: time.Now()}
	return raw, tok, nil
}

// ListAPITokens returns all tokens (revoked and active) ordered by creation time.
func ListAPITokens(db *sql.DB) ([]APIToken, error) {
	rows, err := db.Query("SELECT id, name, user_id, created_at, last_used_at, revoked_at FROM api_tokens ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []APIToken
	for rows.Next() {
		var t APIToken
		if err := rows.Scan(&t.ID, &t.Name, &t.UserID, &t.CreatedAt, &t.LastUsedAt, &t.RevokedAt); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// RevokeAPIToken marks a token as revoked. It is idempotent: revoking an
// already-revoked token is not an error.
func RevokeAPIToken(db *sql.DB, id int64) error {
	result, err := db.Exec("UPDATE api_tokens SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL", time.Now(), id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		// Either the token doesn't exist or was already revoked.
		var exists bool
		if err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM api_tokens WHERE id = ?)", id).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("token not found")
		}
	}
	return nil
}

// ValidateAPIToken looks up a token by its raw value, ensuring it exists and
// has not been revoked. On success it updates last_used_at and returns the
// associated user ID.
func ValidateAPIToken(db *sql.DB, raw string) (int64, error) {
	hash := hashAPIToken(raw)
	var id, userID int64
	var revokedAt sql.NullTime
	err := db.QueryRow("SELECT id, user_id, revoked_at FROM api_tokens WHERE token_hash = ?", hash).Scan(&id, &userID, &revokedAt)
	if err != nil {
		return 0, fmt.Errorf("invalid token")
	}
	if revokedAt.Valid {
		return 0, fmt.Errorf("token revoked")
	}

	// Best-effort update; failure to record last-used should not block auth.
	db.Exec("UPDATE api_tokens SET last_used_at = ? WHERE id = ?", time.Now(), id)

	return userID, nil
}

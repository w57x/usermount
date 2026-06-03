package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func initDB() error {
	var err error
	db, err = sql.Open("sqlite", AppConfig.DBPath)
	if err != nil {
		return err
	}

	createTablesQuery := `
	CREATE TABLE IF NOT EXISTS invites (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code TEXT UNIQUE NOT NULL,
		email TEXT NOT NULL,
		used BOOLEAN NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'user',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err = db.Exec(createTablesQuery)
	return err
}

func hasAdmin() (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func listUsers() ([]User, error) {
	rows, err := db.Query("SELECT id, username, role, created_at FROM users ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func generateCode() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	hexStr := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s", hexStr[0:4], hexStr[4:8], hexStr[8:12]), nil
}

func createInvite(email string) (string, error) {
	code, err := generateCode()
	if err != nil {
		return "", err
	}
	_, err = db.Exec("INSERT INTO invites (code, email) VALUES (?, ?)", code, email)
	if err != nil {
		return "", err
	}
	return code, nil
}

type Invite struct {
	ID        int
	Code      string
	Email     string
	Used      bool
	CreatedAt time.Time
}

func getInviteByCode(code string) (*Invite, error) {
	row := db.QueryRow("SELECT id, code, email, used, created_at FROM invites WHERE code = ? AND created_at >= datetime('now', '-10 minutes')", code)
	var inv Invite
	err := row.Scan(&inv.ID, &inv.Code, &inv.Email, &inv.Used, &inv.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &inv, nil
}

// markInviteAsUsed uses atomic compare-and-swap to guarantee code can only be used once
func markInviteAsUsed(code string) (bool, error) {
	res, err := db.Exec("UPDATE invites SET used = 1 WHERE code = ? AND used = 0", code)
	if err != nil {
		return false, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

type User struct {
	ID           int
	Username     string
	PasswordHash string
	Role         string
	CreatedAt    time.Time
}

func getUser(username string) (*User, error) {
	row := db.QueryRow("SELECT id, username, password_hash, role, created_at FROM users WHERE username = ?", username)
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func createUserDb(username, passwordHash, role string) error {
	_, err := db.Exec("INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)", username, passwordHash, role)
	return err
}

func deleteUser(username string) error {
	_, err := db.Exec("DELETE FROM users WHERE username = ?", username)
	return err
}

func markInviteAsUnused(code string) error {
	_, err := db.Exec("UPDATE invites SET used = 0 WHERE code = ?", code)
	return err
}

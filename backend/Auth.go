package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const userIDKey contextKey = "userID"

func jwtSecret() []byte {
	s := os.Getenv("JWT_SECRET")
	if s == "" {
		s = "change-me-in-production"
	}
	return []byte(s)
}

func generateToken(userID string) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(72 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret())
}

// ── migrations called from InitDB ────────────────────────────────────────────

func migrateAuth(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id            TEXT     PRIMARY KEY,
			email         TEXT     NOT NULL UNIQUE,
			password_hash TEXT     NOT NULL,
			created_at    DATETIME NOT NULL DEFAULT (datetime('now'))
		);

		CREATE TABLE IF NOT EXISTS revoked_tokens (
			token      TEXT     PRIMARY KEY,
			revoked_at DATETIME NOT NULL
		);
	`)
	return err
}

// ── seed data ────────────────────────────────────────────────────────────────

var allDays = `[0,1,2,3,4,5,6]`

type seedTask struct {
	title     string
	area      string
	duration  int
	priority  int
	taskType  string
	subtype   string
	startTime *string
}

func ptr(s string) *string { return &s }

var seedTasks = []seedTask{
	// recurring time-bound
	{title: "Morning workout", area: "Health", duration: 30, priority: 2, taskType: "recurring", subtype: "time-bound", startTime: ptr("07:00")},
	{title: "Deep work block", area: "Work", duration: 90, priority: 1, taskType: "recurring", subtype: "time-bound", startTime: ptr("09:00")},
	{title: "Evening review", area: "Personal", duration: 15, priority: 3, taskType: "recurring", subtype: "time-bound", startTime: ptr("20:00")},
	// recurring flexible
	{title: "Read for 20 minutes", area: "Personal", duration: 20, priority: 3, taskType: "recurring", subtype: "flexible"},
	{title: "Inbox zero", area: "Work", duration: 10, priority: 2, taskType: "recurring", subtype: "flexible"},
}

func seedRoutine(db *sql.DB, userID string) error {
	stmt, err := db.Prepare(`
		INSERT INTO tasks (id, title, area, duration, priority, done, type, subtype, recurrence, start_time, user_id)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, t := range seedTasks {
		var st interface{}
		if t.startTime != nil {
			st = *t.startTime
		}
		_, err := stmt.Exec(
			uuid.New().String(),
			t.title,
			t.area,
			t.duration,
			t.priority,
			t.taskType,
			t.subtype,
			allDays,
			st,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// ── handlers ─────────────────────────────────────────────────────────────────

func handleSignup(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		body.Email = strings.TrimSpace(strings.ToLower(body.Email))
		if body.Email == "" || body.Password == "" {
			http.Error(w, "email and password required", http.StatusBadRequest)
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		userID := uuid.New().String()
		_, err = db.Exec(
			`INSERT INTO users (id, email, password_hash) VALUES (?, ?, ?)`,
			userID, body.Email, string(hash),
		)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE") {
				http.Error(w, "email already registered", http.StatusConflict)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if err := seedRoutine(db, userID); err != nil {
			// non-fatal: account created, seed failed
			_ = err
		}

		token, err := generateToken(userID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"token": token})
	}
}

func handleLogin(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		body.Email = strings.TrimSpace(strings.ToLower(body.Email))

		var userID, hash string
		err := db.QueryRow(
			`SELECT id, password_hash FROM users WHERE email = ?`, body.Email,
		).Scan(&userID, &hash)
		if err == sql.ErrNoRows {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)); err != nil {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}

		token, err := generateToken(userID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"token": token})
	}
}

func handleLogout(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "missing token", http.StatusBadRequest)
			return
		}
		raw := strings.TrimPrefix(authHeader, "Bearer ")

		_, err := db.Exec(
			`INSERT OR IGNORE INTO revoked_tokens (token, revoked_at) VALUES (?, ?)`,
			raw, time.Now().UTC().Format(time.RFC3339),
		)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "logged out"})
	}
}

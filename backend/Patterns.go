package main

import (
	"database/sql"
	"encoding/json"
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// ── models ───────────────────────────────────────────────────────────────────

type PatternEntry struct {
	TaskID       string  `json:"task_id"`
	Title        string  `json:"title"`
	Area         string  `json:"area"`
	FrequencyPct float64 `json:"frequency_pct"`
	DayOfWeek    int     `json:"day_of_week"`
}

type PatternsByDay struct {
	DayOfWeek int            `json:"day_of_week"`
	DayName   string         `json:"day_name"`
	Patterns  []PatternEntry `json:"patterns"`
}

var dayNames = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

// ── core analysis ─────────────────────────────────────────────────────────────

// weeksElapsed returns the number of full weeks since createdAt.
// Minimum 1 to avoid division by zero on brand-new accounts.
func weeksElapsed(createdAt string) int {
	t, err := time.Parse("2006-01-02T15:04:05Z", createdAt)
	if err != nil {
		// try sqlite default format
		t, err = time.Parse("2006-01-02 15:04:05", createdAt)
		if err != nil {
			return 1
		}
	}
	weeks := int(math.Floor(time.Since(t).Hours() / 168))
	if weeks < 1 {
		return 1
	}
	return weeks
}

type taskFrequency struct {
	taskID    string
	title     string
	area      string
	dayOfWeek int
	count     int
}

// analyzePatterns returns task frequencies per day for a user above the given threshold (0–1).
func analyzePatterns(db *sql.DB, userID string, threshold float64) ([]taskFrequency, error) {
	// get user signup date
	var createdAt string
	err := db.QueryRow(`SELECT created_at FROM users WHERE id = ?`, userID).Scan(&createdAt)
	if err != nil {
		return nil, err
	}
	weeks := weeksElapsed(createdAt)

	rows, err := db.Query(`
		SELECT cl.task_id, t.title, t.area, cl.day_of_week, COUNT(*) as completions
		FROM completion_log cl
		JOIN tasks t ON t.id = cl.task_id
		WHERE t.user_id = ?
		GROUP BY cl.task_id, cl.day_of_week
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []taskFrequency
	for rows.Next() {
		var f taskFrequency
		if err := rows.Scan(&f.taskID, &f.title, &f.area, &f.dayOfWeek, &f.count); err != nil {
			continue
		}
		freq := float64(f.count) / float64(weeks)
		if freq >= threshold {
			results = append(results, f)
		}
	}
	return results, nil
}

// ── handlers ─────────────────────────────────────────────────────────────────

func handleGetPatterns(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())

		var createdAt string
		err := db.QueryRow(`SELECT created_at FROM users WHERE id = ?`, userID).Scan(&createdAt)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		weeks := weeksElapsed(createdAt)

		rows, err := db.Query(`
			SELECT cl.task_id, t.title, t.area, cl.day_of_week, COUNT(*) as completions
			FROM completion_log cl
			JOIN tasks t ON t.id = cl.task_id
			WHERE t.user_id = ?
			GROUP BY cl.task_id, cl.day_of_week
		`, userID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		// group by day
		byDay := make(map[int][]PatternEntry)
		for rows.Next() {
			var taskID, title, area string
			var dayOfWeek, count int
			if err := rows.Scan(&taskID, &title, &area, &dayOfWeek, &count); err != nil {
				continue
			}
			pct := (float64(count) / float64(weeks)) * 100
			if pct < 75 {
				continue
			}
			if pct > 100 {
				pct = 100
			}
			byDay[dayOfWeek] = append(byDay[dayOfWeek], PatternEntry{
				TaskID:       taskID,
				Title:        title,
				Area:         area,
				FrequencyPct: math.Round(pct*10) / 10,
				DayOfWeek:    dayOfWeek,
			})
		}

		result := make([]PatternsByDay, 0, 7)
		for d := 0; d < 7; d++ {
			entries, ok := byDay[d]
			if !ok {
				entries = []PatternEntry{}
			}
			result = append(result, PatternsByDay{
				DayOfWeek: d,
				DayName:   dayNames[d],
				Patterns:  entries,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func handlePromotePattern(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())
		taskID := chi.URLParam(r, "id")

		// verify ownership
		var exists int
		err := db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE id = ? AND user_id = ?`, taskID, userID).Scan(&exists)
		if err != nil || exists == 0 {
			http.Error(w, "task not found", http.StatusNotFound)
			return
		}

		// find which days this task was completed in >= 75% of weeks
		var createdAt string
		db.QueryRow(`SELECT created_at FROM users WHERE id = ?`, userID).Scan(&createdAt)
		weeks := weeksElapsed(createdAt)

		rows, err := db.Query(`
			SELECT day_of_week, COUNT(*) as completions
			FROM completion_log
			WHERE task_id = ?
			GROUP BY day_of_week
		`, taskID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var days []int
		for rows.Next() {
			var day, count int
			if err := rows.Scan(&day, &count); err != nil {
				continue
			}
			if float64(count)/float64(weeks) >= 0.75 {
				days = append(days, day)
			}
		}
		if len(days) == 0 {
			http.Error(w, "no qualifying pattern found for this task", http.StatusUnprocessableEntity)
			return
		}

		recJSON, _ := json.Marshal(days)
		_, err = db.Exec(`
			UPDATE tasks SET type = 'recurring', recurrence = ? WHERE id = ? AND user_id = ?
		`, string(recJSON), taskID, userID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		row := db.QueryRow(`
			SELECT id, title, area, duration, priority, notes, done, created_at,
			       type, subtype, recurrence, start_time, user_id
			FROM tasks WHERE id = ?`, taskID)
		task, err := scanTask(row)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(task)
	}
}

// ── route registration ───────────────────────────────────────────────────────

func RegisterPatternRoutes(r chi.Router, db *sql.DB) {
	r.Get("/patterns", handleGetPatterns(db))
	r.Post("/patterns/{id}/promote", handlePromotePattern(db))
}

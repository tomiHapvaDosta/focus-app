package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ── model ────────────────────────────────────────────────────────────────────

type Task struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Area       string  `json:"area"`
	Duration   int     `json:"duration"`
	Priority   int     `json:"priority"`
	Notes      *string `json:"notes"`
	Done       bool    `json:"done"`
	CreatedAt  string  `json:"created_at"`
	Type       string  `json:"type"`
	Subtype    string  `json:"subtype"`
	Recurrence []int   `json:"recurrence"`
	StartTime  *string `json:"start_time"`
	UserID     string  `json:"user_id"`
}

type ConflictLog struct {
	ID           string `json:"id"`
	TaskID1      string `json:"task_id_1"`
	TaskID2      string `json:"task_id_2"`
	ConflictTime string `json:"conflict_time"`
	DayOfWeek    int    `json:"day_of_week"`
	LoggedAt     string `json:"logged_at"`
}

type Gap struct {
	From        string `json:"from"`
	To          string `json:"to"`
	MinutesFree int    `json:"minutes_free"`
	BestFit     *Task  `json:"best_fit"`
}

// ── helpers ──────────────────────────────────────────────────────────────────

func scanTask(row interface {
	Scan(dest ...any) error
}) (*Task, error) {
	var t Task
	var recurrenceRaw string
	var notes, startTime sql.NullString

	err := row.Scan(
		&t.ID, &t.Title, &t.Area, &t.Duration, &t.Priority,
		&notes, &t.Done, &t.CreatedAt,
		&t.Type, &t.Subtype, &recurrenceRaw, &startTime, &t.UserID,
	)
	if err != nil {
		return nil, err
	}

	if notes.Valid {
		t.Notes = &notes.String
	}
	if startTime.Valid {
		t.StartTime = &startTime.String
	}

	if err := json.Unmarshal([]byte(recurrenceRaw), &t.Recurrence); err != nil {
		t.Recurrence = []int{}
	}

	return &t, nil
}

func parseHHMM(s string) (int, int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format")
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return h, m, nil
}

func toMinutes(hhmm string) (int, error) {
	h, m, err := parseHHMM(hhmm)
	if err != nil {
		return 0, err
	}
	return h*60 + m, nil
}

func timesOverlap(start1 string, dur1 int, start2 string, dur2 int) bool {
	s1, err := toMinutes(start1)
	if err != nil {
		return false
	}
	s2, err := toMinutes(start2)
	if err != nil {
		return false
	}
	e1 := s1 + dur1
	e2 := s2 + dur2
	return s1 < e2 && s2 < e1
}

func detectConflict(db *sql.DB, userID, excludeID, startTime string, duration int, days []int) (*Task, error) {
	rows, err := db.Query(`
		SELECT id, title, area, duration, priority, notes, done, created_at,
		       type, subtype, recurrence, start_time, user_id
		FROM tasks
		WHERE user_id = ?
		  AND subtype = 'time-bound'
		  AND start_time IS NOT NULL
		  AND id != ?
		  AND done = 0
	`, userID, excludeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			continue
		}
		if t.StartTime == nil {
			continue
		}
		// check day overlap
		dayOverlap := false
		for _, d1 := range days {
			for _, d2 := range t.Recurrence {
				if d1 == d2 {
					dayOverlap = true
					break
				}
			}
			if dayOverlap {
				break
			}
		}
		if !dayOverlap {
			continue
		}
		if timesOverlap(startTime, duration, *t.StartTime, t.Duration) {
			return t, nil
		}
	}
	return nil, nil
}

func logConflict(db *sql.DB, taskID1, taskID2, conflictTime string, dayOfWeek int) error {
	_, err := db.Exec(`
		INSERT INTO conflict_log (id, task_id_1, task_id_2, conflict_time, day_of_week, logged_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, uuid.New().String(), taskID1, taskID2, conflictTime, dayOfWeek, time.Now().UTC().Format(time.RFC3339))
	return err
}

func getUserTasks(db *sql.DB, userID string, whereExtra string, args ...any) ([]*Task, error) {
	base := `
		SELECT id, title, area, duration, priority, notes, done, created_at,
		       type, subtype, recurrence, start_time, user_id
		FROM tasks
		WHERE user_id = ?`
	if whereExtra != "" {
		base += " AND " + whereExtra
	}

	fullArgs := append([]any{userID}, args...)
	rows, err := db.Query(base, fullArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// ── handlers ─────────────────────────────────────────────────────────────────

func handleGetToday(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())
		today := int(time.Now().Weekday()) // 0=Sun

		all, err := getUserTasks(db, userID, "done = 0")
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		var timeBound []*Task
		var flexible []*Task

		for _, t := range all {
			include := false
			if t.Type == "recurring" {
				for _, d := range t.Recurrence {
					if d == today {
						include = true
						break
					}
				}
			} else {
				// one-time: include if created today
				if strings.HasPrefix(t.CreatedAt, time.Now().Format("2006-01-02")) {
					include = true
				}
			}
			if !include {
				continue
			}
			if t.Subtype == "time-bound" {
				timeBound = append(timeBound, t)
			} else {
				flexible = append(flexible, t)
			}
		}

		sort.Slice(timeBound, func(i, j int) bool {
			if timeBound[i].StartTime == nil {
				return false
			}
			if timeBound[j].StartTime == nil {
				return true
			}
			return *timeBound[i].StartTime < *timeBound[j].StartTime
		})

		sort.Slice(flexible, func(i, j int) bool {
			if flexible[i].Priority != flexible[j].Priority {
				return flexible[i].Priority < flexible[j].Priority
			}
			return flexible[i].Duration < flexible[j].Duration
		})

		result := append(timeBound, flexible...)
		if result == nil {
			result = []*Task{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func handleCreateTask(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())

		var body struct {
			Title      string  `json:"title"`
			Area       string  `json:"area"`
			Duration   int     `json:"duration"`
			Priority   int     `json:"priority"`
			Notes      *string `json:"notes"`
			Type       string  `json:"type"`
			Subtype    string  `json:"subtype"`
			Recurrence []int   `json:"recurrence"`
			StartTime  *string `json:"start_time"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if body.Title == "" || body.Area == "" || body.Type == "" || body.Subtype == "" {
			http.Error(w, "title, area, type, and subtype are required", http.StatusBadRequest)
			return
		}
		if body.Recurrence == nil {
			body.Recurrence = []int{}
		}

		// conflict detection
		if body.Subtype == "time-bound" && body.StartTime != nil {
			conflict, err := detectConflict(db, userID, "", *body.StartTime, body.Duration, body.Recurrence)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if conflict != nil {
				today := int(time.Now().Weekday())
				_ = logConflict(db, "new", conflict.ID, *body.StartTime, today)
				http.Error(w, fmt.Sprintf("time conflict with task: %s", conflict.Title), http.StatusConflict)
				return
			}
		}

		recJSON, _ := json.Marshal(body.Recurrence)
		id := uuid.New().String()

		_, err := db.Exec(`
			INSERT INTO tasks (id, title, area, duration, priority, notes, type, subtype, recurrence, start_time, user_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, id, body.Title, body.Area, body.Duration, body.Priority, body.Notes,
			body.Type, body.Subtype, string(recJSON), body.StartTime, userID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		row := db.QueryRow(`
			SELECT id, title, area, duration, priority, notes, done, created_at,
			       type, subtype, recurrence, start_time, user_id
			FROM tasks WHERE id = ?`, id)
		task, err := scanTask(row)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(task)
	}
}

func handleUpdateTask(db *sql.DB) http.HandlerFunc {
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

		var body struct {
			Title      *string `json:"title"`
			Area       *string `json:"area"`
			Duration   *int    `json:"duration"`
			Priority   *int    `json:"priority"`
			Notes      *string `json:"notes"`
			Type       *string `json:"type"`
			Subtype    *string `json:"subtype"`
			Recurrence []int   `json:"recurrence"`
			StartTime  *string `json:"start_time"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		// fetch current
		row := db.QueryRow(`
			SELECT id, title, area, duration, priority, notes, done, created_at,
			       type, subtype, recurrence, start_time, user_id
			FROM tasks WHERE id = ?`, taskID)
		current, err := scanTask(row)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		// merge
		if body.Title != nil {
			current.Title = *body.Title
		}
		if body.Area != nil {
			current.Area = *body.Area
		}
		if body.Duration != nil {
			current.Duration = *body.Duration
		}
		if body.Priority != nil {
			current.Priority = *body.Priority
		}
		if body.Notes != nil {
			current.Notes = body.Notes
		}
		if body.Type != nil {
			current.Type = *body.Type
		}
		if body.Subtype != nil {
			current.Subtype = *body.Subtype
		}
		if body.Recurrence != nil {
			current.Recurrence = body.Recurrence
		}
		if body.StartTime != nil {
			current.StartTime = body.StartTime
		}

		// conflict detection if time-bound
		if current.Subtype == "time-bound" && current.StartTime != nil {
			conflict, err := detectConflict(db, userID, taskID, *current.StartTime, current.Duration, current.Recurrence)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if conflict != nil {
				today := int(time.Now().Weekday())
				_ = logConflict(db, taskID, conflict.ID, *current.StartTime, today)
				http.Error(w, fmt.Sprintf("time conflict with task: %s", conflict.Title), http.StatusConflict)
				return
			}
		}

		recJSON, _ := json.Marshal(current.Recurrence)

		_, err = db.Exec(`
			UPDATE tasks
			SET title=?, area=?, duration=?, priority=?, notes=?,
			    type=?, subtype=?, recurrence=?, start_time=?
			WHERE id = ? AND user_id = ?
		`, current.Title, current.Area, current.Duration, current.Priority, current.Notes,
			current.Type, current.Subtype, string(recJSON), current.StartTime,
			taskID, userID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		row = db.QueryRow(`
			SELECT id, title, area, duration, priority, notes, done, created_at,
			       type, subtype, recurrence, start_time, user_id
			FROM tasks WHERE id = ?`, taskID)
		updated, err := scanTask(row)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(updated)
	}
}

func handleDeleteTask(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())
		taskID := chi.URLParam(r, "id")

		res, err := db.Exec(`DELETE FROM tasks WHERE id = ? AND user_id = ?`, taskID, userID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			http.Error(w, "task not found", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func handleCompleteTask(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())
		taskID := chi.URLParam(r, "id")

		res, err := db.Exec(`UPDATE tasks SET done = 1 WHERE id = ? AND user_id = ?`, taskID, userID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			http.Error(w, "task not found", http.StatusNotFound)
			return
		}

		dayOfWeek := int(time.Now().Weekday())
		_, err = db.Exec(`
			INSERT INTO completion_log (id, task_id, completed_at, day_of_week)
			VALUES (?, ?, ?, ?)
		`, uuid.New().String(), taskID, time.Now().UTC().Format(time.RFC3339), dayOfWeek)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "completed"})
	}
}

func handleGetFlexible(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())

		tasks, err := getUserTasks(db, userID, "subtype = 'flexible' AND done = 0")
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		sort.Slice(tasks, func(i, j int) bool {
			if tasks[i].Priority != tasks[j].Priority {
				return tasks[i].Priority < tasks[j].Priority
			}
			return tasks[i].Duration < tasks[j].Duration
		})

		if tasks == nil {
			tasks = []*Task{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tasks)
	}
}

func handleGetGaps(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())
		today := int(time.Now().Weekday())

		all, err := getUserTasks(db, userID, "done = 0")
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		// collect today's time-bound tasks
		var timeBound []*Task
		var flexible []*Task
		for _, t := range all {
			onToday := false
			for _, d := range t.Recurrence {
				if d == today {
					onToday = true
					break
				}
			}
			if t.Type == "one-time" {
				onToday = true
			}
			if !onToday {
				continue
			}
			if t.Subtype == "time-bound" && t.StartTime != nil {
				timeBound = append(timeBound, t)
			} else if t.Subtype == "flexible" {
				flexible = append(flexible, t)
			}
		}

		sort.Slice(flexible, func(i, j int) bool {
			if flexible[i].Priority != flexible[j].Priority {
				return flexible[i].Priority < flexible[j].Priority
			}
			return flexible[i].Duration < flexible[j].Duration
		})

		// sort time-bound by start
		sort.Slice(timeBound, func(i, j int) bool {
			return *timeBound[i].StartTime < *timeBound[j].StartTime
		})

		// build blocks: [startMin, endMin]
		type block struct{ start, end int }
		var blocks []block
		for _, t := range timeBound {
			s, _ := toMinutes(*t.StartTime)
			blocks = append(blocks, block{s, s + t.Duration})
		}

		// day window: 06:00 to 23:00
		dayStart := 6 * 60
		dayEnd := 23 * 60

		var gaps []Gap
		cursor := dayStart

		addGap := func(from, to int) {
			if to-from < 5 {
				return
			}
			fromStr := fmt.Sprintf("%02d:%02d", from/60, from%60)
			toStr := fmt.Sprintf("%02d:%02d", to/60, to%60)
			minutesFree := to - from

			var bestFit *Task
			for _, f := range flexible {
				if f.Duration <= minutesFree {
					bestFit = f
					break
				}
			}

			gaps = append(gaps, Gap{
				From:        fromStr,
				To:          toStr,
				MinutesFree: minutesFree,
				BestFit:     bestFit,
			})
		}

		for _, b := range blocks {
			if b.start > cursor {
				addGap(cursor, b.start)
			}
			if b.end > cursor {
				cursor = b.end
			}
		}
		if cursor < dayEnd {
			addGap(cursor, dayEnd)
		}

		if gaps == nil {
			gaps = []Gap{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(gaps)
	}
}

func handleGetScheduleDay(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())
		dayParam := chi.URLParam(r, "day")
		day, err := strconv.Atoi(dayParam)
		if err != nil || day < 0 || day > 6 {
			http.Error(w, "day must be 0-6", http.StatusBadRequest)
			return
		}

		all, err := getUserTasks(db, userID, "subtype = 'time-bound' AND done = 0")
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		var result []*Task
		for _, t := range all {
			for _, d := range t.Recurrence {
				if d == day {
					result = append(result, t)
					break
				}
			}
		}

		sort.Slice(result, func(i, j int) bool {
			if result[i].StartTime == nil {
				return false
			}
			if result[j].StartTime == nil {
				return true
			}
			return *result[i].StartTime < *result[j].StartTime
		})

		if result == nil {
			result = []*Task{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func handleGetScheduleConflicts(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())

		// return conflicts where either task belongs to user
		rows, err := db.Query(`
			SELECT cl.id, cl.task_id_1, cl.task_id_2, cl.conflict_time, cl.day_of_week, cl.logged_at
			FROM conflict_log cl
			WHERE cl.task_id_1 IN (SELECT id FROM tasks WHERE user_id = ?)
			   OR cl.task_id_2 IN (SELECT id FROM tasks WHERE user_id = ?)
			ORDER BY cl.logged_at DESC
		`, userID, userID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var result []ConflictLog
		for rows.Next() {
			var c ConflictLog
			if err := rows.Scan(&c.ID, &c.TaskID1, &c.TaskID2, &c.ConflictTime, &c.DayOfWeek, &c.LoggedAt); err != nil {
				continue
			}
			result = append(result, c)
		}
		if result == nil {
			result = []ConflictLog{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func handleGetErrors(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())

		rows, err := db.Query(`
			SELECT cl.id, cl.task_id_1, cl.task_id_2, cl.conflict_time, cl.day_of_week, cl.logged_at
			FROM conflict_log cl
			WHERE cl.task_id_1 IN (SELECT id FROM tasks WHERE user_id = ?)
			   OR cl.task_id_2 IN (SELECT id FROM tasks WHERE user_id = ?)
			ORDER BY cl.logged_at DESC
		`, userID, userID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var result []ConflictLog
		for rows.Next() {
			var c ConflictLog
			if err := rows.Scan(&c.ID, &c.TaskID1, &c.TaskID2, &c.ConflictTime, &c.DayOfWeek, &c.LoggedAt); err != nil {
				continue
			}
			result = append(result, c)
		}
		if result == nil {
			result = []ConflictLog{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// ── route registration ───────────────────────────────────────────────────────

func RegisterTaskRoutes(r chi.Router, db *sql.DB) {
	r.Get("/tasks/today", handleGetToday(db))
	r.Get("/tasks/flexible", handleGetFlexible(db))
	r.Get("/tasks/gaps", handleGetGaps(db))
	r.Post("/tasks", handleCreateTask(db))
	r.Put("/tasks/{id}", handleUpdateTask(db))
	r.Delete("/tasks/{id}", handleDeleteTask(db))
	r.Post("/tasks/{id}/complete", handleCompleteTask(db))
	r.Get("/schedule/{day}", handleGetScheduleDay(db))
	r.Get("/schedule/conflicts", handleGetScheduleConflicts(db))
	r.Get("/errors", handleGetErrors(db))
}

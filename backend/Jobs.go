package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"time"
)

// StartPatternJob launches a weekly ticker that auto-promotes tasks
// completed in 90%+ of weeks since the user's signup date.
func StartPatternJob(db *sql.DB) {
	ticker := time.NewTicker(7 * 24 * time.Hour)
	go func() {
		// run once immediately on startup, then weekly
		runAutoPromote(db)
		for range ticker.C {
			runAutoPromote(db)
		}
	}()
}

func runAutoPromote(db *sql.DB) {
	log.Println("[jobs] running auto-promote pattern job")

	// collect all users
	rows, err := db.Query(`SELECT id FROM users`)
	if err != nil {
		log.Printf("[jobs] failed to query users: %v", err)
		return
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		userIDs = append(userIDs, id)
	}

	for _, userID := range userIDs {
		if err := autoPromoteForUser(db, userID); err != nil {
			log.Printf("[jobs] auto-promote failed for user %s: %v", userID, err)
		}
	}

	log.Println("[jobs] auto-promote complete")
}

func autoPromoteForUser(db *sql.DB, userID string) error {
	matches, err := analyzePatterns(db, userID, 0.90)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return nil
	}

	// group qualifying days per task
	daysByTask := make(map[string][]int)
	for _, m := range matches {
		daysByTask[m.taskID] = append(daysByTask[m.taskID], m.dayOfWeek)
	}

	for taskID, days := range daysByTask {
		// only promote tasks that are not already recurring
		var taskType string
		err := db.QueryRow(`SELECT type FROM tasks WHERE id = ? AND user_id = ?`, taskID, userID).Scan(&taskType)
		if err != nil {
			continue
		}
		if taskType == "recurring" {
			continue
		}

		recJSON, _ := json.Marshal(days)
		_, err = db.Exec(`
			UPDATE tasks SET type = 'recurring', recurrence = ?
			WHERE id = ? AND user_id = ?
		`, string(recJSON), taskID, userID)
		if err != nil {
			log.Printf("[jobs] failed to promote task %s: %v", taskID, err)
			continue
		}
		log.Printf("[jobs] promoted task %s for user %s on days %v", taskID, userID, days)
	}

	return nil
}

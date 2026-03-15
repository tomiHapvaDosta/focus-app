package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

func main() {
	db, err := InitDB("focus.db")
	if err != nil {
		log.Fatalf("init db: %v", err)
	}
	defer db.Close()

	if err := migrateAuth(db); err != nil {
		log.Fatalf("migrate auth: %v", err)
	}

	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"http://localhost:5173"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	r.Post("/auth/signup", handleSignup(db))
	r.Post("/auth/login", handleLogin(db))
	r.Post("/auth/logout", handleLogout(db))

	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware(db))
		// protected routes go here
	})

	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware(db))
		RegisterTaskRoutes(r, db)
	})

	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware(db))
		RegisterTaskRoutes(r, db)
		RegisterPatternRoutes(r, db)
	})

	StartPatternJob(db)
	fmt.Println("Backend running on http://localhost:8080")
	http.ListenAndServe(":8080", r)
}

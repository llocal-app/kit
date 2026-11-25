package main

import (
	"encoding/json"
	"net/http"

	"llocal.app/kit"
)

type note struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
	By   string `json:"by"`
}

func main() {
	app := kit.New()
	app.Get("/api/notes", listNotes(app))
	app.Post("/api/notes", addNote(app))
	app.Run()
}

func listNotes(app *kit.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := app.DB.QueryContext(r.Context(), `SELECT id, body, by_name FROM notes ORDER BY id DESC`)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		notes := []note{}
		for rows.Next() {
			var n note
			if err := rows.Scan(&n.ID, &n.Body, &n.By); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			notes = append(notes, n)
		}
		if err := rows.Err(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		kit.JSON(w, http.StatusOK, notes)
	}
}

func addNote(app *kit.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := kit.CurrentUser(r)
		var n note
		if err := json.NewDecoder(r.Body).Decode(&n); err != nil || n.Body == "" {
			http.Error(w, "body is required", http.StatusBadRequest)
			return
		}
		res, err := app.DB.ExecContext(r.Context(),
			`INSERT INTO notes (body, user_id, by_name) VALUES (?, ?, ?)`, n.Body, user.ID, user.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		n.ID, _ = res.LastInsertId()
		n.By = user.Name
		kit.JSON(w, http.StatusCreated, n)
	}
}

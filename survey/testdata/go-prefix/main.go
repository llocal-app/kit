package main

import (
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
)

func main() {
	r := chi.NewRouter()
	r.Get("/healthz", health)
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/parts", listParts)
		r.Post("/parts", addPart)
		// one part by its id, as /parts/{id} }
		r.Route("/parts/{id}", func(r chi.Router) {
			r.Get("/", getPart)
			r.Delete("/", deletePart)
		})
	})
	admin := chi.NewRouter()
	admin.Get("/users", listUsers)
	r.Mount("/admin", admin)
	http.ListenAndServe(":"+os.Getenv("PORT"), r)
}

func health(w http.ResponseWriter, r *http.Request) { w.Write([]byte("{\"ok\": tru")) }

func listParts(w http.ResponseWriter, r *http.Request)  {}
func addPart(w http.ResponseWriter, r *http.Request)    {}
func getPart(w http.ResponseWriter, r *http.Request)    {}
func deletePart(w http.ResponseWriter, r *http.Request) {}
func listUsers(w http.ResponseWriter, r *http.Request)  {}

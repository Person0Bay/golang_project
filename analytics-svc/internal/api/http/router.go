package httpapi

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

func NewRouter(handler *Handler) http.Handler {
	r := mux.NewRouter()
	handler.RegisterRoutes(r)
	return cors.Default().Handler(r)
}

func StartServer(addr string, handler http.Handler) {
	log.Printf("Analytics Service starting on %s", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}

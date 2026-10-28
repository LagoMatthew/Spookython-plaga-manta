package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"spookython-backend/internal/domain"
	"spookython-backend/internal/engine"
)

func simulateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req domain.SimulationRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	response := engine.RunFullSimulation(req)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func main() {
	http.HandleFunc("/simulate", simulateHandler)
	fmt.Println("🧟 Motor SEIR Manta activo en http://localhost:8080/simulate")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}

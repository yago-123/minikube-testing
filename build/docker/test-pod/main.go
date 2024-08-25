package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// Sample data to be returned as JSON
var sampleData = map[string]interface{}{
	"message": "Hello, world!",
	"status":  "success",
	"data": map[string]interface{}{
		"id":   1,
		"name": "Sample Item",
	},
}

// handler function to serve JSON response
func jsonHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(sampleData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	http.HandleFunc("/api", jsonHandler)
	port := "8080"
	log.Printf("Starting server on :%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

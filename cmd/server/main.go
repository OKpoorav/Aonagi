package main

import (
	"aonagi/internal/ai"
	"aonagi/internal/ws"
	"fmt"
	"net/http"
)

func main() {
	// Register the AI Observer callback to decouple ws from ai package
	ws.HubInstance.OnMessageBroadcast = ai.Observe

	http.HandleFunc("/ws", ws.Handler)
	fmt.Println("Server Running on port :8080")
	err := http.ListenAndServe(":8080", nil)

	if err != nil {
		fmt.Println("Error connecting", err)
		return
	}

}

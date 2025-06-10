package utils

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// CheckGRPCConnection checks connection to a gRPC endpoint.
func CheckGRPCConnection(ctx context.Context, conn *grpc.ClientConn) error {
	state := conn.GetState()
	log.Printf("Initial state: %v", state)

	// Wait for state change from current state
	if conn.WaitForStateChange(ctx, state) {
		newState := conn.GetState()
		log.Printf("New state: %v", newState)
	} else {
		log.Println("No state change occurred within the timeout period")
	}

	if conn.GetState() == connectivity.Ready {
		log.Println("Connection is ready")
		return nil
	}
	log.Println("Connection is not ready")
	return fmt.Errorf("connection is not ready")
}

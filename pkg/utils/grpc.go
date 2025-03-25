package utils

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// CheckGRPCConnection checks connection to a gRPC endpoint.
func CheckGRPCConnection(address string) error {
	options := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	conn, err := grpc.NewClient(address, options...)
	if err != nil {
		return fmt.Errorf("failed to connect to a3m server at %q: %w", address, err)
	}
	defer conn.Close()
	return nil
}

package driver

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
)

// NonBlockingGRPCServer is a non-blocking gRPC server
type NonBlockingGRPCServer struct {
	server *grpc.Server
	wg     sync.WaitGroup
}

// NewNonBlockingGRPCServer creates a new non-blocking gRPC server
func NewNonBlockingGRPCServer() *NonBlockingGRPCServer {
	return &NonBlockingGRPCServer{}
}

// Start starts the gRPC server
func (s *NonBlockingGRPCServer) Start(endpoint string, ids csi.IdentityServer, cs csi.ControllerServer, ns csi.NodeServer) error {
	s.wg.Add(1)
	go s.serve(endpoint, ids, cs, ns)
	return nil
}

// Stop stops the gRPC server
func (s *NonBlockingGRPCServer) Stop() {
	if s.server != nil {
		s.server.GracefulStop()
	}
	s.wg.Wait()
}

// serve starts serving gRPC requests
func (s *NonBlockingGRPCServer) serve(endpoint string, ids csi.IdentityServer, cs csi.ControllerServer, ns csi.NodeServer) {
	defer s.wg.Done()

	proto, addr, err := parseEndpoint(endpoint)
	if err != nil {
		klog.Fatalf("Failed to parse endpoint: %v", err)
	}

	if proto == "unix" {
		// Remove existing socket file
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			klog.Fatalf("Failed to remove existing socket file %s: %v", addr, err)
		}
	}

	listener, err := net.Listen(proto, addr)
	if err != nil {
		klog.Fatalf("Failed to listen on %s://%s: %v", proto, addr, err)
	}

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(logGRPC),
	}
	s.server = grpc.NewServer(opts...)

	if ids != nil {
		csi.RegisterIdentityServer(s.server, ids)
	}
	if cs != nil {
		csi.RegisterControllerServer(s.server, cs)
	}
	if ns != nil {
		csi.RegisterNodeServer(s.server, ns)
	}

	klog.Infof("Listening for connections on %s://%s", proto, addr)
	if err := s.server.Serve(listener); err != nil {
		klog.Fatalf("Failed to serve gRPC server: %v", err)
	}
}

// parseEndpoint parses the endpoint string
func parseEndpoint(endpoint string) (string, string, error) {
	if strings.HasPrefix(endpoint, "unix://") || strings.HasPrefix(endpoint, "tcp://") {
		s := strings.SplitN(endpoint, "://", 2)
		if len(s) != 2 {
			return "", "", fmt.Errorf("invalid endpoint: %s", endpoint)
		}
		return s[0], s[1], nil
	}
	// Default to unix socket
	return "unix", endpoint, nil
}

// logGRPC logs gRPC requests
func logGRPC(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	klog.V(4).Infof("gRPC call: %s", info.FullMethod)
	klog.V(5).Infof("gRPC request: %+v", req)

	resp, err := handler(ctx, req)

	if err != nil {
		klog.Errorf("gRPC error: %v", err)
	} else {
		klog.V(5).Infof("gRPC response: %+v", resp)
	}

	return resp, err
}

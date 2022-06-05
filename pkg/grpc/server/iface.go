/*
 * Copyright (c) The go-kit Authors
 */

package server

import "google.golang.org/grpc"

// serviceRegistry.RegisterService will be implemented by any grpc service
type serviceRegistry interface {
	RegisterService(*grpc.Server)
}

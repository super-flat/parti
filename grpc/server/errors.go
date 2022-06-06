/*
 * Copyright (c) The go-kit Authors
 */

package server

import "github.com/pkg/errors"

var (
	errMissingTraceURL                = errors.New("trace URL is not defined")
	errMissingServiceName             = errors.New("service name is not defined")
	errMsgTracerRegistrationFailure   = "unable to register the OTLP tracer"
	errMsgTracerDeregistrationFailure = "unable to deregister the OTLP tracer"
	errMsgListenerServiceFailure      = "grpc listener failed to serve"
	errMsgCannotUseSameBuilder        = errors.New("cannot use the same builder to build more than once")
)

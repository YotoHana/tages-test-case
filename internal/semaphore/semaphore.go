package semaphore

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	TooManyReqs = "too many concurent requests"
)

type Semaphore struct {
	slots chan struct{}
}


func (s *Semaphore) Acquire() {
	s.slots <- struct{}{}
}

func (s *Semaphore) TryAcquire() bool {
	select {
	case s.slots <- struct{}{}:
		return true
		
	default:
		return false
	}
}

func (s *Semaphore) Release() {
	<- s.slots
}

func NewSemaphore(limit int) *Semaphore {
	return &Semaphore{
		slots: make(chan struct{}, limit),
	}
}

func RateLimitStream(limiter *Semaphore) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler) error {
			if !limiter.TryAcquire() {
				return status.Error(codes.ResourceExhausted, TooManyReqs)
			}
			defer limiter.Release()

			return handler(srv, ss)
		}
}

func RateLimitUnary(limiter *Semaphore) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (resp any, err error) {
			if !limiter.TryAcquire() {
				return nil, status.Error(codes.ResourceExhausted, TooManyReqs)
			}
			defer limiter.Release()

			return handler(ctx, req)
		}
}
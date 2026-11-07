package rpc

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"

	grantv1 "github.com/Azure/fiberd/api/grant/v1"
	"github.com/Azure/fiberd/pkg/core"
)

// Miss policy: every miss carries a grantv1.Miss detail. Never a bare code.
//
//	core.Shed             -> ResourceExhausted + Miss{SHED, retry_after_s}
//	core.DeferredFallback -> Unavailable       + Miss{DEFERRED_FALLBACK, issuer}
//	core.NeedsTier        -> FailedPrecondition
//	core.Invalid          -> InvalidArgument (ErrAnonymousPark, ErrSessionRunning)
//	core.Unauthenticated  -> Unauthenticated
//	                         + WrongHome{home} when the cause is core.ErrWrongAudience and the home has a node id
//	core.NotFound         -> NotFound (ErrFiberUnknown, ErrSessionUnknown)
//	core.NoWorkspace      -> FailedPrecondition + NoWorkspace
//	core.OverBudget       -> ResourceExhausted, no Miss
//	core.Internal         -> Internal
func (s *Server) ToError(code core.StatusCode, err error) error {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	switch code {
	case core.OK:
		return nil
	case core.Shed:
		return withDetail(codes.ResourceExhausted, msg, &grantv1.Miss{
			Code:        grantv1.MissCode_SHED,
			RetryAfterS: uint32(s.retryAfter().Seconds()),
			Issuer:      s.Issuer,
		})
	case core.DeferredFallback:
		miss := &grantv1.Miss{Code: grantv1.MissCode_DEFERRED_FALLBACK, Issuer: s.Issuer}
		var rm *core.RemoteMiss
		if errors.As(err, &rm) {
			miss.PreferredHome = rm.PreferredHome
		}
		return withDetail(codes.Unavailable, msg, miss)
	case core.NeedsTier:
		return status.Error(codes.FailedPrecondition, msg)
	case core.Invalid:
		return status.Error(codes.InvalidArgument, msg)
	case core.Unauthenticated:
		if errors.Is(err, core.ErrWrongAudience) && s.Core.NodeID != "" {
			return withDetail(codes.Unauthenticated, msg, &grantv1.WrongHome{Home: s.Core.NodeID})
		}
		return status.Error(codes.Unauthenticated, msg)
	case core.NotFound:
		return status.Error(codes.NotFound, msg)
	case core.NoWorkspace:
		return withDetail(codes.FailedPrecondition, msg, &grantv1.NoWorkspace{})
	case core.OverBudget:
		return status.Error(codes.ResourceExhausted, msg)
	default:
		return status.Error(codes.Internal, msg)
	}
}

func withDetail(c codes.Code, msg string, d protoadapt.MessageV1) error {
	st, err := status.New(c, msg).WithDetails(d)
	if err != nil {
		// Cannot happen for a registered message type; fail loudly rather
		// than return a bare code that violates the protocol.
		panic("rpc: attach detail: " + err.Error())
	}
	return st.Err()
}

// detail is the first detail of type T a gRPC error carries.
func detail[T proto.Message](err error) (T, bool) {
	var none T
	st, ok := status.FromError(err)
	if !ok {
		return none, false
	}
	for _, d := range st.Details() {
		if m, ok := d.(T); ok {
			return m, true
		}
	}
	return none, false
}

// MissFromError extracts the Miss detail from a gRPC error, for clients
// and the conformance suite.
func MissFromError(err error) (*grantv1.Miss, bool) {
	return detail[*grantv1.Miss](err)
}

// WrongHomeFromError extracts the WrongHome detail from a gRPC error, for
// clients and the conformance suite. Its home is the node id of the home
// that refused the grant.
func WrongHomeFromError(err error) (*grantv1.WrongHome, bool) {
	return detail[*grantv1.WrongHome](err)
}

// IsNoWorkspace reports whether err is the refusal of a file call on a
// fiber whose grant has no workspace.
func IsNoWorkspace(err error) bool {
	if status.Code(err) != codes.FailedPrecondition {
		return false
	}
	_, ok := detail[*grantv1.NoWorkspace](err)
	return ok
}

// IsMiss reports whether err is one of the two miss outcomes.
func IsMiss(err error) bool {
	_, ok := MissFromError(err)
	return ok
}

var errNoDeadline = errors.New("rpc: deadline already passed")

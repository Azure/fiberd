package grant

import (
	"errors"
	"fmt"

	"github.com/Azure/fiberd/pkg/core"
)

// Token errors.
var (
	ErrNoKID         = errors.New("grant: token has no kid header")
	ErrClaimMismatch = errors.New("grant: registered claims disagree with the grant claim")
	// ErrWrongAudience is core's, so one sentinel covers the verifier's
	// and the home's refusals of a grant for another home.
	ErrWrongAudience  = core.ErrWrongAudience
	ErrWrongIssuer    = errors.New("grant: issuer mismatch")
	ErrMissingGrant   = errors.New("grant: token carries no grant claim")
	ErrKeyMismatchAlg = errors.New("grant: key algorithm does not match token")
	ErrTokenTooLarge  = fmt.Errorf("grant: token exceeds %d bytes", MaxTokenBytes)
	ErrLeaseTooLong   = errors.New("grant: lease longer than this home accepts")
	ErrEmptyGrant     = errors.New("grant: grant_uid is empty")
	ErrBadUID         = errors.New("grant: grant_uid must be a DNS-1123 label (lowercase alphanumerics and '-', at most 63)")
	ErrBadTenant      = errors.New("grant: tenant must be one path segment (alphanumerics, '.', '_' and '-', at most 253)")
)

// Key set errors.
var (
	ErrUnknownKey  = errors.New("grant: no key for kid")
	ErrNeverLoaded = errors.New("grant: key set never loaded")
	ErrJWKSStale   = errors.New("grant: key set older than the lease TTL")
)

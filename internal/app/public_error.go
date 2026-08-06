package app

import "github.com/jorgeluis594/happy-memory/internal/publicerror"

const storeBusyCode = publicerror.StoreBusyCode

// PublicError is the only error shape exposed by the application boundary.
type PublicError = publicerror.Error

func publicError(err error) PublicError {
	return publicerror.From(err)
}

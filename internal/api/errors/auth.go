package errors

import "errors"

var ErrAuthNotAuthenticated = errors.New("not authenticated")
var ErrAuthFailedLogout = errors.New("failed to logout")

var ErrAuthMissedStateParam = errors.New("state is not present in the query parameters")
var ErrAuthValidateBase64State = errors.New("failed to decode state")
var ErrAuthExchangeToken = errors.New("failed to exchange token")
var ErrAuthWrongCodeVerifier = errors.New("failed to extract code verifier")
var ErrAuthMissingDataForCodeVerifier = errors.New("missing data for code verifier")
var ErrAuthMissingRefreshToken = errors.New("refresh token is missing")

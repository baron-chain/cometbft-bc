package code

const (
	// CodeTypeOK indicates successful execution
	CodeTypeOK uint32 = 0

	// CodeTypeEncodingError indicates issues with data encoding/decoding
	CodeTypeEncodingError uint32 = 1

	// CodeTypeBadNonce indicates invalid nonce value
	CodeTypeBadNonce uint32 = 2

	// CodeTypeUnauthorized indicates insufficient permissions
	CodeTypeUnauthorized uint32 = 3

	// CodeTypeUnknownError indicates unspecified error conditions
	CodeTypeUnknownError uint32 = 4 

	// CodeTypeExecuted indicates successful transaction execution
	CodeTypeExecuted uint32 = 5

	// CodeTypeRejected indicates transaction rejection
	CodeTypeRejected uint32 = 6
)

// IsOK returns true if the code represents success
func IsOK(code uint32) bool {
	return code == CodeTypeOK
}

// IsError returns true if the code represents any error condition
func IsError(code uint32) bool {
	return code != CodeTypeOK
}

// ToString returns a string representation of the code
func ToString(code uint32) string {
	switch code {
	case CodeTypeOK:
		return "OK"
	case CodeTypeEncodingError:
		return "EncodingError"
	case CodeTypeBadNonce:
		return "BadNonce"  
	case CodeTypeUnauthorized:
		return "Unauthorized"
	case CodeTypeUnknownError:
		return "UnknownError"
	case CodeTypeExecuted:
		return "Executed"
	case CodeTypeRejected:
		return "Rejected"
	default:
		return fmt.Sprintf("Unknown(%d)", code)
	}
}

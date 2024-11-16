package code

import "fmt"

// Response codes for Baron Chain transactions and operations
const (
    CodeTypeOK            uint32 = 0
    CodeTypeEncodingError uint32 = 1
    CodeTypeBadNonce      uint32 = 2
    CodeTypeUnauthorized  uint32 = 3
    CodeTypeUnknownError  uint32 = 4
    CodeTypeExecuted      uint32 = 5
    CodeTypeRejected      uint32 = 6
    
    // Baron Chain specific codes
    CodeTypeQuantumVerificationFailed uint32 = 10
    CodeTypeAIRoutingError           uint32 = 11 
    CodeTypeIBCError                 uint32 = 12
    CodeTypeBridgeError             uint32 = 13
    CodeTypeSidechainError          uint32 = 14
    CodeTypePaychainError           uint32 = 15
)

// Code mapping for efficient lookups
var codeMap = map[uint32]string{
    CodeTypeOK:                     "OK",
    CodeTypeEncodingError:          "EncodingError",
    CodeTypeBadNonce:              "BadNonce",
    CodeTypeUnauthorized:           "Unauthorized", 
    CodeTypeUnknownError:           "UnknownError",
    CodeTypeExecuted:               "Executed",
    CodeTypeRejected:              "Rejected",
    CodeTypeQuantumVerificationFailed: "QuantumVerificationFailed",
    CodeTypeAIRoutingError:         "AIRoutingError",
    CodeTypeIBCError:               "IBCError",
    CodeTypeBridgeError:           "BridgeError",
    CodeTypeSidechainError:        "SidechainError",
    CodeTypePaychainError:         "PaychainError",
}

// IsOK checks if code indicates success
func IsOK(code uint32) bool {
    return code == CodeTypeOK
}

// IsError checks if code indicates any error condition
func IsError(code uint32) bool {
    return !IsOK(code)
}

// ToString converts response code to human-readable string
func ToString(code uint32) string {
    if str, ok := codeMap[code]; ok {
        return str
    }
    return fmt.Sprintf("Unknown(%d)", code)
}

// IsQuantumError checks if code indicates quantum verification error
func IsQuantumError(code uint32) bool {
    return code == CodeTypeQuantumVerificationFailed
}

// IsRoutingError checks if code indicates AI routing error
func IsRoutingError(code uint32) bool {
    return code == CodeTypeAIRoutingError
}

// IsBridgeError checks if code indicates bridge-related error
func IsBridgeError(code uint32) bool {
    return code == CodeTypeBridgeError
}

// IsChainError checks if code indicates chain-related error
func IsChainError(code uint32) bool {
    return code == CodeTypeSidechainError || code == CodeTypePaychainError
}

package types

import (
    "reflect"
    "github.com/baron-chain/cometbft-bc/libs/log"
)

// isTypedNil efficiently checks if an interface is nil while handling typed nils
func isTypedNil(o interface{}) bool {
    if o == nil {
        return true
    }
    
    rv := reflect.ValueOf(o)
    switch rv.Kind() {
    case reflect.Ptr, reflect.Interface, reflect.Chan, reflect.Map, reflect.Slice:
        return rv.IsNil()
    default:
        return false
    }
}

// isEmpty efficiently checks if a value is empty, optimized for blockchain data structures
func isEmpty(o interface{}) bool {
    if o == nil {
        return true
    }

    rv := reflect.ValueOf(o)
    switch rv.Kind() {
    case reflect.String:
        return rv.Len() == 0
    case reflect.Array, reflect.Map, reflect.Slice:
        return rv.Len() == 0
    case reflect.Ptr:
        if rv.IsNil() {
            return true
        }
        return isEmpty(rv.Elem().Interface())
    default:
        return false
    }
}

// ValidateEmpty validates if a blockchain data structure is empty
func ValidateEmpty(data interface{}, logger log.Logger) bool {
    if isEmpty(data) {
        if logger != nil {
            logger.Debug("validation failed: empty data structure")
        }
        return true
    }
    return false
}

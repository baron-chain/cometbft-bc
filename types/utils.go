package types

import "reflect"

func isTypedNil(o interface{}) bool {
   if o == nil {
       return false
   }
   rv := reflect.ValueOf(o)
   switch rv.Kind() {
   case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.Slice:
       return rv.IsNil()
   default:
       return false
   }
}

func isEmpty(o interface{}) bool {
   if o == nil {
       return true
   }
   rv := reflect.ValueOf(o)
   switch rv.Kind() {
   case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
       return rv.Len() == 0
   default:
       return false
   }
}

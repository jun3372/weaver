package reflection

import (
	"fmt"
	"reflect"
)

func Type[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

func ComponentName[T any]() string {
	t := Type[T]()
	return fmt.Sprintf("%s/%s", t.PkgPath(), t.Name())
}

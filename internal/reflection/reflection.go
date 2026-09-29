package reflection

import (
	"fmt"
	"reflect"
)

func Type[T any]() reflect.Type {
	return reflect.TypeFor[T]()
}

func ComponentName[T any]() string {
	t := Type[T]()
	return fmt.Sprintf("%s/%s", t.PkgPath(), t.Name())
}

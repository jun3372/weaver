package defaults

import (
	"reflect"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// BindFields 递归扫描 typ(struct 类型)的全部导出叶子字段,对每个叶子
// 调用 conf.BindEnv(viperKey, envVarName),使环境变量可覆盖配置文件。
//
//   - viperKey = topLevelKey + "." + fieldPath(保持原始大小写,如 "readTimeout")
//   - envName  = envPrefix + "_" + UPPERCASE(topLevelKey) + "_" +
//     UPPERCASE(fieldPath 中 "." 替换为 "_")
//
// 例如 BindFields(conf, "http", "WEAVER", t) 的 readTimeout 字段:
//
//	viperKey = "http.readTimeout"
//	envName  = "WEAVER_HTTP_READTIMEOUT"
//
// 嵌套 struct(含指针 struct 的 Elem)递归展开;time.Time 视为叶子;
// slice/map/非 struct 叶子直接绑定(不展开内部)。
func BindFields(conf *viper.Viper, topLevelKey string, envPrefix string, typ reflect.Type) {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return
	}
	bindStruct(conf, topLevelKey, topLevelKey, envPrefix, typ)
}

func bindStruct(conf *viper.Viper, topLevelKey, fieldPath string, envPrefix string, typ reflect.Type) {
	for f := range typ.Fields() {
		if !f.IsExported() {
			continue
		}

		ft := f.Type
		if ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}

		childPath := fieldPath + "." + f.Name

		// 嵌套 struct(非 time.Time):递归展开
		if ft.Kind() == reflect.Struct && ft != reflect.TypeFor[time.Time]() {
			bindStruct(conf, topLevelKey, childPath, envPrefix, ft)
			continue
		}

		// 叶子字段:绑定 env
		viperKey := childPath
		envName := envPrefix + "_" + strings.ToUpper(topLevelKey) + "_" +
			strings.ToUpper(strings.ReplaceAll(fieldPath, ".", "_")) + "_" +
			strings.ToUpper(f.Name)
		conf.BindEnv(viperKey, envName)
	}
}

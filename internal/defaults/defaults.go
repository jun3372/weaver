// Package defaults 提供 struct 字段的 default tag 解析与填充。
//
// 与 viper 配合使用,实现"环境变量 > 配置文件 > default tag"的优先级链:
// 先由 viper.UnmarshalKey 把 env/file 已配置的值解析进 struct,
// 再由 Apply 把 viper 中未设置且为零值的字段按 default tag 填充。
// 这样"用户显式写 0"不会被 default 覆盖(因为 IsSet 返回 true)。
package defaults

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// tagName 是默认值标签名。
const tagName = "default"

// Apply 按 default tag 填充 v(struct 指针)中的零值字段。
//
// 仅当字段同时满足以下条件时写入:
//   - 字段为零值
//   - 字段带 default tag
//   - isSet(viperKey) 返回 false(即 env/file/SetDefault 均未设置该 key)
//
// keyPrefix 是当前 struct 在 viper 中的 key 路径(如 "weaver.logger" 或 "http"),
// 用于拼接字段级 key 查询 isSet。viper key 按字段名小写计算(与 viper 内部行为一致)。
//
// 支持类型: string / bool / 各整数 / float32,64 / time.Duration /
// 嵌套 struct(递归) / 指针 struct(非 nil 时递归)。
// slice / map / chan / func 不支持,跳过。
// 解析失败返回 fmt.Errorf("field %s: %w", path, err)。
func Apply(v any, keyPrefix string, isSet func(string) bool) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("defaults.Apply: 需要非空指针,收到 %T", v)
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("defaults.Apply: 需要 struct 指针,收到 %T", v)
	}
	return applyStruct(rv, keyPrefix, isSet)
}

func applyStruct(s reflect.Value, keyPrefix string, isSet func(string) bool) error {
	t := s.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		fv := s.Field(i)
		if !fv.CanSet() {
			continue
		}

		// viper key 按字段名小写计算(与 viper 内部 ToLower 行为一致)
		viperKey := keyPrefix + "." + strings.ToLower(f.Name)

		// 嵌套 struct(非指针):递归,不查 isSet(因为父段可能整体缺失)
		if f.Type.Kind() == reflect.Struct && f.Type != reflect.TypeOf(time.Time{}) {
			if err := applyStruct(fv, viperKey, isSet); err != nil {
				return err
			}
			continue
		}

		// 指针 struct:非 nil 时递归,nil 时跳过(保持"可选段"语义)
		if f.Type.Kind() == reflect.Pointer && f.Type.Elem().Kind() == reflect.Struct {
			if !fv.IsNil() {
				if err := applyStruct(fv.Elem(), viperKey, isSet); err != nil {
					return err
				}
			}
			continue
		}

		// 叶子字段
		tag, ok := f.Tag.Lookup(tagName)
		if !ok {
			continue
		}
		// viper 已设置(含 env/file/SetDefault)→ 不覆盖
		if isSet != nil && isSet(viperKey) {
			continue
		}
		// 字段非零值 → 不覆盖
		if !fv.IsZero() {
			continue
		}
		if err := setField(fv, f.Type, tag, f.Name); err != nil {
			return err
		}
	}
	return nil
}

func setField(fv reflect.Value, ft reflect.Type, tag, name string) error {
	// time.Duration 特殊处理(底层 int64,但按 string 解析)
	if ft == reflect.TypeOf(time.Duration(0)) {
		d, err := time.ParseDuration(tag)
		if err != nil {
			return fmt.Errorf("field %s: 解析 duration %q: %w", name, tag, err)
		}
		fv.SetInt(int64(d))
		return nil
	}

	switch ft.Kind() {
	case reflect.String:
		fv.SetString(tag)
	case reflect.Bool:
		b, err := strconv.ParseBool(tag)
		if err != nil {
			return fmt.Errorf("field %s: 解析 bool %q: %w", name, tag, err)
		}
		fv.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(tag, 10, ft.Bits())
		if err != nil {
			return fmt.Errorf("field %s: 解析 int %q: %w", name, tag, err)
		}
		fv.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(tag, 10, ft.Bits())
		if err != nil {
			return fmt.Errorf("field %s: 解析 uint %q: %w", name, tag, err)
		}
		fv.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(tag, ft.Bits())
		if err != nil {
			return fmt.Errorf("field %s: 解析 float %q: %w", name, tag, err)
		}
		fv.SetFloat(n)
	default:
		// slice / map / chan / func 等不支持,静默跳过
	}
	return nil
}

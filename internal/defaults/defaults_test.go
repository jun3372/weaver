package defaults

import (
	"strings"
	"testing"
	"time"
)

func TestBasicTypes(t *testing.T) {
	type S struct {
		Name    string        `default:"hello"`
		Count   int           `default:"42"`
		Flag    bool          `default:"true"`
		Timeout time.Duration `default:"5s"`
		Ratio   float64       `default:"3.14"`
		Port    uint16        `default:"8080"`
	}
	var s S
	if err := Apply(&s, "root", nil); err != nil {
		t.Fatal(err)
	}
	if s.Name != "hello" {
		t.Errorf("Name = %q, want %q", s.Name, "hello")
	}
	if s.Count != 42 {
		t.Errorf("Count = %d, want 42", s.Count)
	}
	if !s.Flag {
		t.Error("Flag = false, want true")
	}
	if s.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", s.Timeout)
	}
	if s.Ratio != 3.14 {
		t.Errorf("Ratio = %f, want 3.14", s.Ratio)
	}
	if s.Port != 8080 {
		t.Errorf("Port = %d, want 8080", s.Port)
	}
}

func TestNonZeroNotOverridden(t *testing.T) {
	type S struct {
		Name  string `default:"hello"`
		Count int    `default:"42"`
	}
	s := S{Name: "world", Count: 99}
	if err := Apply(&s, "root", nil); err != nil {
		t.Fatal(err)
	}
	if s.Name != "world" {
		t.Errorf("Name = %q, want %q", s.Name, "world")
	}
	if s.Count != 99 {
		t.Errorf("Count = %d, want 99", s.Count)
	}
}

func TestMissingTagSkipped(t *testing.T) {
	type S struct {
		Name   string `default:"hello"`
		NoTag  string
		Count  int `default:"10"`
		NoTagI int
	}
	var s S
	if err := Apply(&s, "root", nil); err != nil {
		t.Fatal(err)
	}
	if s.Name != "hello" {
		t.Errorf("Name = %q, want %q", s.Name, "hello")
	}
	if s.NoTag != "" {
		t.Errorf("NoTag = %q, want empty", s.NoTag)
	}
	if s.Count != 10 {
		t.Errorf("Count = %d, want 10", s.Count)
	}
	if s.NoTagI != 0 {
		t.Errorf("NoTagI = %d, want 0", s.NoTagI)
	}
}

func TestNestedStruct(t *testing.T) {
	type Inner struct {
		Value string `default:"inner-default"`
	}
	type S struct {
		Name  string `default:"outer"`
		Inner Inner
	}
	var s S
	if err := Apply(&s, "root", nil); err != nil {
		t.Fatal(err)
	}
	if s.Name != "outer" {
		t.Errorf("Name = %q, want %q", s.Name, "outer")
	}
	if s.Inner.Value != "inner-default" {
		t.Errorf("Inner.Value = %q, want %q", s.Inner.Value, "inner-default")
	}
}

func TestPointerStructNonNil(t *testing.T) {
	type Inner struct {
		Value string `default:"ptr-default"`
	}
	type S struct {
		Inner *Inner
	}
	s := S{Inner: &Inner{}}
	if err := Apply(&s, "root", nil); err != nil {
		t.Fatal(err)
	}
	if s.Inner.Value != "ptr-default" {
		t.Errorf("Inner.Value = %q, want %q", s.Inner.Value, "ptr-default")
	}
}

func TestPointerStructNilSkipped(t *testing.T) {
	type Inner struct {
		Value string `default:"ptr-default"`
	}
	type S struct {
		Inner *Inner
	}
	var s S
	if err := Apply(&s, "root", nil); err != nil {
		t.Fatal(err)
	}
	if s.Inner != nil {
		t.Errorf("Inner = %v, want nil", s.Inner)
	}
}

func TestIsSetSkipsField(t *testing.T) {
	type S struct {
		Name  string `default:"hello"`
		Count int    `default:"42"`
	}
	var s S
	isSet := func(key string) bool {
		return key == "root.name"
	}
	if err := Apply(&s, "root", isSet); err != nil {
		t.Fatal(err)
	}
	if s.Name != "" {
		t.Errorf("Name = %q, want empty (isSet should skip)", s.Name)
	}
	if s.Count != 42 {
		t.Errorf("Count = %d, want 42", s.Count)
	}
}

func TestIllegalValueReturnsError(t *testing.T) {
	type S struct {
		Bad int `default:"not-a-number"`
	}
	var s S
	err := Apply(&s, "root", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Bad") {
		t.Errorf("error %q should contain field name 'Bad'", err.Error())
	}
}

func TestIllegalDurationReturnsError(t *testing.T) {
	type S struct {
		Timeout time.Duration `default:"not-a-duration"`
	}
	var s S
	err := Apply(&s, "root", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Timeout") {
		t.Errorf("error %q should contain field name 'Timeout'", err.Error())
	}
}

func TestNonPointerReturnsError(t *testing.T) {
	type S struct{}
	var s S
	err := Apply(s, "root", nil)
	if err == nil {
		t.Fatal("expected error for non-pointer, got nil")
	}
}

func TestNilPointerReturnsError(t *testing.T) {
	type S struct{}
	var s *S
	err := Apply(s, "root", nil)
	if err == nil {
		t.Fatal("expected error for nil pointer, got nil")
	}
}

func TestIntSizes(t *testing.T) {
	type S struct {
		A int8  `default:"127"`
		B int16 `default:"32767"`
		C int32 `default:"100000"`
		D int64 `default:"9999999999"`
	}
	var s S
	if err := Apply(&s, "root", nil); err != nil {
		t.Fatal(err)
	}
	if s.A != 127 {
		t.Errorf("A = %d, want 127", s.A)
	}
	if s.B != 32767 {
		t.Errorf("B = %d, want 32767", s.B)
	}
	if s.C != 100000 {
		t.Errorf("C = %d, want 100000", s.C)
	}
	if s.D != 9999999999 {
		t.Errorf("D = %d, want 9999999999", s.D)
	}
}

func TestUintSizes(t *testing.T) {
	type S struct {
		A uint8  `default:"255"`
		B uint16 `default:"65535"`
		C uint32 `default:"100000"`
		D uint64 `default:"9999999999"`
	}
	var s S
	if err := Apply(&s, "root", nil); err != nil {
		t.Fatal(err)
	}
	if s.A != 255 {
		t.Errorf("A = %d, want 255", s.A)
	}
	if s.B != 65535 {
		t.Errorf("B = %d, want 65535", s.B)
	}
	if s.C != 100000 {
		t.Errorf("C = %d, want 100000", s.C)
	}
	if s.D != 9999999999 {
		t.Errorf("D = %d, want 9999999999", s.D)
	}
}

func TestFloat32(t *testing.T) {
	type S struct {
		V float32 `default:"2.5"`
	}
	var s S
	if err := Apply(&s, "root", nil); err != nil {
		t.Fatal(err)
	}
	if s.V != 2.5 {
		t.Errorf("V = %f, want 2.5", s.V)
	}
}

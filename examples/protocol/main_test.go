package main

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestNegotiate(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    []byte
		wantErr bool
	}{
		{
			name:  "正确握手",
			input: []byte{5, 2, 0, 1},
			want:  []byte{5, 0xFF},
		},
		{
			name:    "版本错误",
			input:   []byte{4, 1, 0},
			wantErr: true,
		},
		{
			name:    "半包:头部截断",
			input:   []byte{5},
			wantErr: true,
		},
		{
			name:    "半包:methods 截断",
			input:   []byte{5, 2, 0},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := negotiate(bytes.NewReader(tt.input), &out)
			if (err != nil) != tt.wantErr {
				t.Fatalf("negotiate() err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !bytes.Equal(out.Bytes(), tt.want) {
				t.Fatalf("应答 = %v, want %v", out.Bytes(), tt.want)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	frame := func(magic uint16, payload []byte) []byte {
		out := make([]byte, 4+len(payload))
		binary.BigEndian.PutUint16(out[0:2], magic)
		binary.BigEndian.PutUint16(out[2:4], uint16(len(payload)))
		copy(out[4:], payload)
		return out
	}

	tests := []struct {
		name    string
		input   []byte
		want    []byte
		wantErr bool
	}{
		{name: "正常帧", input: frame(0xCAFE, []byte("hello")), want: []byte("hello")},
		{name: "空 payload", input: frame(0xCAFE, nil), want: []byte{}},
		{name: "短包", input: []byte{0xCA}, wantErr: true},
		{name: "magic 不匹配", input: frame(0xBEEF, []byte("x")), wantErr: true},
		{name: "len 不匹配", input: frame(0xCAFE, []byte("ab"))[:5], wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decode(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("decode() err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !bytes.Equal(got, tt.want) {
				t.Fatalf("payload = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	payload := []byte("round trip")
	got, err := decode(encode(payload))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("round trip = %q, want %q", got, payload)
	}
}

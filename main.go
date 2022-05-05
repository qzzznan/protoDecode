package main

import (
	"encoding/binary"
	"fmt"
	"google.golang.org/protobuf/proto"
	"math"
	"protoDecode/hello"
)

//go:generate protoc --go_out=. hello.proto

func main() {
	f := &hello.Fields{
		F1: map[int32]int32{4: 5, 6: 7, 8: 9},
		F2: 9999999,
		F3: -87878342,
		F4: 88.88,
		F5: "Hello World",
		F6: 66.66,
	}
	data, err := proto.Marshal(f)
	if err != nil {
		fmt.Println(err)
	}
	for _, v := range data {
		fmt.Printf("0b%08b %02d 0x%02X\n", v, v, v)
	}

	lst := make([]*Field, 0)
	var idx int
	for {
		fd := &Field{}
		n, err := ReadField(data[idx:], fd)
		if err != nil {
			fmt.Println("parse error", err)
			break
		}
		if n == 0 {
			break
		}
		lst = append(lst, fd)
		idx += n
	}
	for _, v := range lst {
		switch v.WireType {
		case _varint:
			a, _ := v.AsInt64()
			b, _ := v.AsSint64()
			fmt.Printf("Field:%d AsInt64:%d AsSint64:%d\n", v.FieldNum, a, b)

		case _64bit:
			d, _ := v.AsDouble()
			fmt.Printf("Field:%d AsDouble:%f\n", v.FieldNum, d)

		case _lengthDelimited:
			s, _ := v.AsString()
			f, _ := v.AsEmbedded()
			fmt.Printf("Field:%d AsString:%s EmbeddedLen:%d\n", v.FieldNum, s, len(f))

		case _32bit:
			f, _ := v.AsFloat()
			fmt.Printf("Field:%d AsFloat:%f\n", v.FieldNum, f)

		default:
			fmt.Println("unknown wire type")
		}
	}
}

type Field struct {
	FieldNum int
	WireType int
	Length   int
	data     []byte
}

func (f *Field) String() string {
	return fmt.Sprintf("FieldNum: %d WireType:%s DataLength:%d", f.FieldNum, WireType(f.WireType), f.Length)
}

func (f *Field) AsInt64() (int64, error) {
	if f.WireType != _varint {
		return 0, fmt.Errorf("not varint")
	}
	i, _ := varint(f.data)
	return i, nil
}

func (f *Field) AsSint64() (int64, error) {
	if f.WireType != _varint {
		return 0, fmt.Errorf("not varint")
	}
	i, _ := varint(f.data)
	return zzd64(i), nil
}

func (f *Field) AsString() (string, error) {
	if f.WireType != _lengthDelimited {
		return "", fmt.Errorf("not length delimited")
	}
	return string(f.data), nil
}

func (f *Field) AsEmbedded() ([]*Field, error) {
	if f.WireType != _lengthDelimited {
		return nil, fmt.Errorf("not length delimited")
	}
	lst := make([]*Field, 0)
	for idx := 0; ; {
		fd := &Field{}
		n, err := ReadField(f.data[idx:], fd)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			break
		}
		idx += n
		lst = append(lst, fd)
	}
	return lst, nil
}

func (f *Field) AsFloat() (float32, error) {
	if f.WireType != _32bit {
		return 0, fmt.Errorf("not 32bit")
	}
	b := binary.LittleEndian.Uint32(f.data)
	return math.Float32frombits(b), nil
}

func (f *Field) AsDouble() (float64, error) {
	if f.WireType != _64bit {
		return 0, fmt.Errorf("not 64bit")
	}
	b := binary.LittleEndian.Uint64(f.data)
	return math.Float64frombits(b), nil
}

func (f *Field) AsFixed32() (int32, error) {
	if f.WireType != _32bit {
		return 0, fmt.Errorf("not 32bit")
	}
	return int32(binary.LittleEndian.Uint32(f.data)), nil
}

func (f *Field) AsFixed64() (int64, error) {
	if f.WireType != _64bit {
		return 0, fmt.Errorf("not 64bit")
	}
	return int64(binary.LittleEndian.Uint64(f.data)), nil
}

func ReadField(src []byte, f *Field) (int, error) {
	if len(src) == 0 {
		return 0, nil
	}
	var n int
	fn, wire := tag(src[0])
	n += 1

	f.FieldNum = fn
	f.WireType = wire

	if wire == _varint {
		c := countVarintByte(src[n:])
		if c < 1 {
			return 0, fmt.Errorf("varint length < 1")
		}
		f.Length = c
		f.data = src[n : n+c]
		n += c
	} else if wire == _64bit {
		f.data = src[n : n+8]
		f.Length = 8
		n += 8
	} else if wire == _lengthDelimited {
		l, c := varint(src[n:])
		n += c
		f.Length = int(l)
		f.data = src[n : n+int(l)]
		n += int(l)
	} else if wire == _32bit {
		f.data = src[n : n+4]
		f.Length = 4
		n += 4
	} else {
		return -1, fmt.Errorf("unknown wire type: %d", wire)
	}

	return n, nil
}

func countVarintByte(src []byte) int {
	var n int
	for _, v := range src {
		if v&0x80 == 0 {
			return n + 1
		} else {
			n += 1
		}
	}
	return n
}

const (
	_varint          = 0
	_64bit           = 1
	_lengthDelimited = 2
	_32bit           = 5
)

func WireType(wire int) string {
	switch wire {
	case _varint:
		return "varint"
	case _64bit:
		return "64bit"
	case _lengthDelimited:
		return "lengthDelimited"
	case _32bit:
		return "32bit"
	default:
		return "unknown"
	}
}

func varint(arr []byte) (int64, int) {
	var x int64
	for c, v := range arr {
		if v&0x80 == 0 {
			x |= int64(v) << (c * 7)
			return x, c + 1
		} else {
			x |= int64(v&0x7F) << (c * 7)
		}
	}
	return 0, 0
}

func tag(b byte) (t int, f int) {
	t = int((b & 0b11111000) >> 3)
	f = int(b & 0b00000111)
	return
}

func zze32(i int32) int32 {
	return (i >> 31) ^ (i << 1)
}

func zzd32(i int32) int32 {
	return (i >> 1) ^ -(i & 1)
}

func zze64(i int64) int64 {
	return (i >> 63) ^ (i << 1)
}

func zzd64(i int64) int64 {
	return (i >> 1) ^ -(i & 1)
}

func unmarshal(d []byte) {
	var idx int
	for {
		if idx >= len(d) {
			break
		}
		_, wire := tag(d[idx])
		idx++
		switch wire {
		case _varint: // int32 int64 uint32 uint64 bool enum <sint32 sint64>
			_, n := varint(d[idx:])
			idx += n
		case _64bit: // fixed64 sfixed64 double (little-endian)
			_ = d[idx : idx+8]
			idx += 8

		case _lengthDelimited: // string bytes embedded message, packed repeated fields
			length, n := varint(d[idx:])
			idx += n
			_ = d[idx : idx+int(length)]
			idx += int(length)

		case _32bit: // fixed32 sfixed32 float  (little-endian)
			_ = d[idx : idx+4]
			idx += 4
		default:
			return
		}
	}
}

/*
  https://developers.google.com/protocol-buffers/docs/encoding
*/

package cell

import (
	"bytes"
	"testing"
)

// BenchmarkFixedCellEncode benchmarks encoding of fixed-size cells
func BenchmarkFixedCellEncode(b *testing.B) {
	cell := &Cell{
		CircID:  12345,
		Command: CmdPadding,
		Payload: make([]byte, 509),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := &bytes.Buffer{}
		err := cell.Encode(buf)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFixedCellDecode benchmarks decoding of fixed-size cells
func BenchmarkFixedCellDecode(b *testing.B) {
	// Create a valid encoded cell
	cell := &Cell{
		CircID:  12345,
		Command: CmdPadding,
		Payload: make([]byte, 509),
	}
	buf := &bytes.Buffer{}
	err := cell.Encode(buf)
	if err != nil {
		b.Fatal(err)
	}
	data := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		_, err := DecodeCell(reader)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRelayCellEncode benchmarks encoding of relay cells
func BenchmarkRelayCellEncode(b *testing.B) {
	relay := &RelayCell{
		Command:  RelayBegin,
		StreamID: 1,
		Digest:   [4]byte{0x01, 0x02, 0x03, 0x04},
		Length:   100,
		Data:     make([]byte, 100),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := relay.Encode()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRelayCellDecode benchmarks decoding of relay cells
func BenchmarkRelayCellDecode(b *testing.B) {
	// Create a valid encoded relay cell
	relay := &RelayCell{
		Command:  RelayBegin,
		StreamID: 1,
		Digest:   [4]byte{0x01, 0x02, 0x03, 0x04},
		Length:   100,
		Data:     make([]byte, 100),
	}
	data, err := relay.Encode()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := DecodeRelayCell(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCellEncodeParallel benchmarks parallel cell encoding
func BenchmarkCellEncodeParallel(b *testing.B) {
	cell := &Cell{
		CircID:  12345,
		Command: CmdPadding,
		Payload: make([]byte, 509),
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := &bytes.Buffer{}
			err := cell.Encode(buf)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkCellDecodeParallel benchmarks parallel cell decoding
func BenchmarkCellDecodeParallel(b *testing.B) {
	// Create a valid encoded cell
	cell := &Cell{
		CircID:  12345,
		Command: CmdPadding,
		Payload: make([]byte, 509),
	}
	buf := &bytes.Buffer{}
	err := cell.Encode(buf)
	if err != nil {
		b.Fatal(err)
	}
	data := buf.Bytes()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			reader := bytes.NewReader(data)
			_, err := DecodeCell(reader)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

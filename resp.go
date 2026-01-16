package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

const (
	STRING  = '+'
	ERROR   = '-'
	INTEGER = ':'
	BULK    = '$'
	ARRAY   = '*'
)

type Value struct {
	typ   string
	str   string
	num   int
	bulk  string
	array []Value
}

type Resp struct {
	reader *bufio.Reader
}

func NewResp(rd io.Reader) *Resp {
	return &Resp{reader: bufio.NewReader(rd)}
}

// readLine reads a RESP line and returns the line content (without CRLF)
// along with the total number of bytes consumed from the stream (including CRLF).
func (r *Resp) readLine() (line []byte, n int, err error) {
	for {
		b, err := r.reader.ReadByte()
		if err != nil {
			return nil, 0, err
		}

		// Taking `$5\r\n` as an example, n will be 4, not 2.
		// n counts all bytes read from the wire, including `\r\n`.
		// This is critical for RESP parsing, since higher-level parsers
		// need to know exactly how many bytes were consumed and where
		// the next value starts in the stream.
		n++
		line = append(line, b)

		// RESP lines are terminated by `\r\n`.
		// We cannot stop immediately on `\r`, because doing so would
		// leave the following `\n` unread in the buffer.
		// Instead, after reading one more byte, we check whether the
		// previous byte was `\r` (meaning the last two bytes are `\r\n`).
		if len(line) >= 2 && line[len(line)-2] == '\r' {
			break
		}
	}

	// Return the line without the trailing `\r\n`.
	return line[:len(line)-2], n, nil
}

// readInteger reads a RESP integer (e.g. `:1000\r\n`) from the stream.
// It returns the parsed integer value, the total number of bytes consumed (including CRLF).
func (r *Resp) readInteger() (x int, n int, err error) {
	line, n, err := r.readLine()
	if err != nil {
		return 0, 0, err
	}
	i64, err := strconv.ParseInt(string(line), 10, 64)
	if err != nil {
		return 0, 0, err
	}
	return int(i64), n, nil
}

// Read reads the next RESP value from the stream by inspecting
// the leading type byte and dispatching to the appropriate parser.
func (r *Resp) Read() (Value, error) {
	t, err := r.reader.ReadByte()
	if err != nil {
		return Value{}, err
	}

	switch t {
	case ARRAY:
		return r.readArray()
	case BULK:
		return r.readBulk()
	default:
		// RESP defines additional types (simple strings, errors, integers).
		// Returning an error here prevents silent protocol desync.
		return Value{}, fmt.Errorf("unknown RESP type byte: %q", t)
	}
}

// readArray parses a RESP array.
func (r *Resp) readArray() (Value, error) {
	v := Value{typ: "array"}

	// Read array length (number of elements)
	length, _, err := r.readInteger()
	if err != nil {
		return v, err
	}

	// Parse each element recursively using Read()
	v.array = make([]Value, length)
	for i := 0; i < length; i++ {
		val, err := r.Read()
		if err != nil {
			return v, err
		}
		v.array[i] = val
	}

	return v, nil
}

// readBulk parses a RESP bulk string.
func (r *Resp) readBulk() (Value, error) {
	v := Value{typ: "bulk"}

	// Read bulk length (number of bytes in payload)
	length, _, err := r.readInteger()
	if err != nil {
		return v, err
	}

	// Bulk strings are length-prefixed and may contain arbitrary bytes,
	// including '\r\n'. Therefore, readLine() cannot be used here.
	//
	// Example (13-byte payload containing CRLF):
	//   $13\r\n
	//   Hello\r\nWorld\r\n
	//
	// Using readLine() would incorrectly stop at the first '\r\n'
	// inside the payload and corrupt the stream.
	bulk := make([]byte, length)

	// Ensure the full payload is read
	if _, err := io.ReadFull(r.reader, bulk); err != nil {
		return v, err
	}

	v.bulk = string(bulk)

	// Manually consume the trailing '\r\n' that terminates the bulk string
	if _, _, err := r.readLine(); err != nil {
		return v, err
	}

	return v, nil
}

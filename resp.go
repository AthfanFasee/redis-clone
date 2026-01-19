package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

const (
	// Wire format prefixes
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

// READER

type Resp struct {
	reader *bufio.Reader
}

func NewResp(rd io.Reader) *Resp {
	return &Resp{reader: bufio.NewReader(rd)}
}

// readLine reads a RESP line and returns the line content (without CRLF)
// along with the total number of bytes consumed from the stream (including CRLF).
func (r *Resp) readLine() (line []byte, bytesRead int, err error) {
	for {
		currentByte, err := r.reader.ReadByte()
		if err != nil {
			return nil, 0, err
		}

		// Taking `$5\r\n` as an example, bytesRead will be 4, not 2.
		// bytesRead counts all bytes read from the wire, including `\r\n`.
		// This is critical for RESP parsing, since higher-level parsers
		// need to know exactly how many bytes were consumed and where
		// the next value starts in the stream.
		bytesRead++
		line = append(line, currentByte)

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
	return line[:len(line)-2], bytesRead, nil
}

// readInteger reads a RESP integer (e.g. `:1000\r\n`) from the stream.
// It returns the parsed integer value, the total number of bytes consumed (including CRLF).
func (r *Resp) readInteger() (x int, bytesRead int, err error) {
	line, bytesRead, err := r.readLine()
	if err != nil {
		return 0, 0, err
	}
	i64, err := strconv.ParseInt(string(line), 10, 64)
	if err != nil {
		return 0, 0, err
	}
	return int(i64), bytesRead, nil
}

// Read reads the next RESP value from the stream by inspecting
// the leading type byte and dispatching to the appropriate parser.
func (r *Resp) Read() (Value, error) {
	respType, err := r.reader.ReadByte()
	if err != nil {
		return Value{}, err
	}

	switch respType {
	case ARRAY:
		return r.readArray()
	case BULK:
		return r.readBulk()
	default:
		// RESP defines additional types (simple strings, errors, integers).
		// Returning an error here prevents silent protocol desync.
		return Value{}, fmt.Errorf("unknown RESP type byte: %q", string(respType))
	}
}

// readArray parses a RESP array.
func (r *Resp) readArray() (Value, error) {
	v := Value{typ: "array"}

	// Read array length (number of elements)
	arrayLength, _, err := r.readInteger()
	if err != nil {
		return v, err
	}

	// Parse each element recursively using Read()
	v.array = make([]Value, arrayLength)
	for i := 0; i < arrayLength; i++ {
		arrayElement, err := r.Read()
		if err != nil {
			return v, err
		}
		v.array[i] = arrayElement
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

// WRITER

type Writer struct {
	writer io.Writer
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{writer: w}
}

// Write serializes a Value into RESP format and writes it to the underlying writer.
func (w *Writer) Write(v Value) error {
	_, err := w.writer.Write(v.Marshal())
	return err
}

// Marshal converts a Value into its RESP wire representation.
// It returns the exact bytes that should be sent to the client.
func (v Value) Marshal() []byte {
	switch v.typ {
	case "array":
		return v.marshalArray()
	case "bulk":
		return v.marshalBulk()
	case "string":
		return v.marshalString()
	case "null":
		return v.marshallNull()
	case "error":
		return v.marshallError()
	default:
		// Unknown type hence return empty response
		return []byte{}
	}
}

// marshalString encodes a RESP simple string.
func (v Value) marshalString() []byte {
	var bytes []byte

	bytes = append(bytes, STRING)     // '+'
	bytes = append(bytes, v.str...)   // string content
	bytes = append(bytes, '\r', '\n') // CRLF terminator

	return bytes
}

// marshalBulk encodes a RESP bulk string.
func (v Value) marshalBulk() []byte {
	var bytes []byte

	bytes = append(bytes, BULK)                         // '$'
	bytes = append(bytes, strconv.Itoa(len(v.bulk))...) // length as ASCII digits
	bytes = append(bytes, '\r', '\n')                   // end of length line
	bytes = append(bytes, v.bulk...)                    // raw bulk data
	bytes = append(bytes, '\r', '\n')                   // CRLF after data

	return bytes
}

// marshalArray encodes a RESP array.
func (v Value) marshalArray() []byte {
	arrayLength := len(v.array)

	var bytes []byte
	bytes = append(bytes, ARRAY) // '*'
	// When you write `append(byteSlice, String...)``
	// Go automatically converts the string to []byte
	// So this is equvalent to `append(bytes, []byte(strconv.Itoa(arrayLength))...)`
	bytes = append(bytes, strconv.Itoa(arrayLength)...)
	bytes = append(bytes, '\r', '\n')

	// Marshal each element and append its bytes
	// Each array element knows how to convert itself into RESP bytes
	for i := 0; i < arrayLength; i++ {
		bytes = append(bytes, v.array[i].Marshal()...)
	}

	return bytes
}

// marshallError encodes a RESP error.
func (v Value) marshallError() []byte {
	var bytes []byte

	bytes = append(bytes, ERROR) // '-'
	bytes = append(bytes, v.str...)
	bytes = append(bytes, '\r', '\n')

	return bytes
}

// marshallNull encodes a RESP null bulk string.
// empty string ≠ null
// $-1\r\n = key does not exist
func (v Value) marshallNull() []byte {
	return []byte("$-1\r\n")
}

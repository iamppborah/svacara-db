package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

const (
	msgQuery  uint8 = 0x01
	msgResult uint8 = 0x10
	msgError  uint8 = 0x11
	msgReady  uint8 = 0x20
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: svacara-cli <host:port>\n")
		os.Exit(1)
	}

	addr := os.Args[1]
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Printf("SvacaraDB CLI connected to %s\n", addr)
	fmt.Println("Type SQL or commands: BEGIN, COMMIT, ABORT, EXIT")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("svacara> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.ToUpper(line) == "EXIT" || strings.ToUpper(line) == "QUIT" {
			break
		}

		payload := []byte(line)
		msg := make([]byte, 5+len(payload))
		msg[0] = msgQuery
		binary.BigEndian.PutUint32(msg[1:5], uint32(len(payload)))
		copy(msg[5:], payload)

		if _, err := conn.Write(msg); err != nil {
			fmt.Fprintf(os.Stderr, "write: %v\n", err)
			break
		}

		header := make([]byte, 5)
		if _, err := io.ReadFull(conn, header); err != nil {
			fmt.Fprintf(os.Stderr, "read: %v\n", err)
			break
		}

		msgType := header[0]
		length := binary.BigEndian.Uint32(header[1:5])
		body := make([]byte, length)
		if length > 0 {
			if _, err := io.ReadFull(conn, body); err != nil {
				fmt.Fprintf(os.Stderr, "read body: %v\n", err)
				break
			}
		}

		switch msgType {
		case msgResult:
			printResult(body)
		case msgError:
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", string(body))
		case msgReady:
			fmt.Println("OK")
		default:
			fmt.Printf("unknown response type: %d\n", msgType)
		}
	}
}

func printResult(data []byte) {
	if len(data) < 1 {
		return
	}
	pos := 0
	ncols := int(data[pos])
	pos++

	cols := make([]string, ncols)
	for i := 0; i < ncols; i++ {
		if pos+2 > len(data) {
			return
		}
		clen := int(binary.BigEndian.Uint16(data[pos:]))
		pos += 2
		cols[i] = string(data[pos : pos+clen])
		pos += clen
	}

	fmt.Println(strings.Join(cols, "\t"))
	fmt.Println(strings.Repeat("-", len(strings.Join(cols, "\t"))))

	for pos < len(data) {
		row := make([]string, ncols)
		for i := 0; i < ncols; i++ {
			if pos+2 > len(data) {
				return
			}
			clen := int(binary.BigEndian.Uint16(data[pos:]))
			pos += 2
			row[i] = string(data[pos : pos+clen])
			pos += clen
		}
		fmt.Println(strings.Join(row, "\t"))
	}
}

package protocol

import "encoding/binary"

const (
	MsgQuery   uint8 = 0x01
	MsgPrepare uint8 = 0x02
	MsgExecute uint8 = 0x03
	MsgBegin   uint8 = 0x04
	MsgCommit  uint8 = 0x05
	MsgAbort   uint8 = 0x06
	MsgResult  uint8 = 0x10
	MsgError   uint8 = 0x11
	MsgRowCount uint8 = 0x12
	MsgReady   uint8 = 0x20
)

func EncodeMessage(msgType uint8, payload []byte) []byte {
	buf := make([]byte, 5+len(payload))
	buf[0] = msgType
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(payload)))
	copy(buf[5:], payload)
	return buf
}

func DecodeMessage(data []byte) (uint8, []byte, bool) {
	if len(data) < 5 {
		return 0, nil, false
	}
	msgType := data[0]
	length := binary.BigEndian.Uint32(data[1:5])
	if len(data) < 5+int(length) {
		return 0, nil, false
	}
	return msgType, data[5 : 5+length], true
}

func EncodeString(s string) []byte {
	buf := make([]byte, 2+len(s))
	binary.BigEndian.PutUint16(buf[:2], uint16(len(s)))
	copy(buf[2:], s)
	return buf
}

func DecodeString(data []byte) (string, int) {
	if len(data) < 2 {
		return "", 0
	}
	length := binary.BigEndian.Uint16(data[:2])
	if len(data) < 2+int(length) {
		return "", 0
	}
	return string(data[2 : 2+length]), 2 + int(length)
}

func EncodeResult(cols []string, rows []map[string]string) []byte {
	var payload []byte
	payload = append(payload, byte(len(cols)))
	for _, c := range cols {
		payload = append(payload, EncodeString(c)...)
	}
	for _, row := range rows {
		for _, c := range cols {
			payload = append(payload, EncodeString(row[c])...)
		}
	}
	return EncodeMessage(MsgResult, payload)
}

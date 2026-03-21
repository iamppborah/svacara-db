package storage

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

const checksumOff = PageSize - 4

func checksum(page []byte) uint32 {
	return crc32.ChecksumIEEE(page[4:checksumOff])
}

func WriteMetaFull(page []byte, root uint64, flushed uint64, headPage, headSeq, tailPage, tailSeq uint64) {
	WriteMeta(page, root, flushed, headPage, headSeq, tailPage, tailSeq)
	binary.LittleEndian.PutUint32(page[checksumOff:], checksum(page))
}

func VerifyMeta(page []byte) bool {
	stored := binary.LittleEndian.Uint32(page[checksumOff:])
	computed := checksum(page)
	return stored == computed
}

func ReadMetaSafe(page []byte) (root uint64, flushed uint64, headPage, headSeq, tailPage, tailSeq uint64, ok bool) {
	if !VerifyMeta(page) {
		return 0, 0, 0, 0, 0, 0, false
	}
	return ReadMeta(page)
}

func (pm *PageMgr) Recover() error {
	meta := pm.Read(0)
	if !VerifyMeta(meta) {
		return fmt.Errorf("meta page checksum mismatch — possible corruption or incomplete write")
	}
	_, flushed, _, _, _, _, ok := ReadMeta(meta)
	if !ok {
		return fmt.Errorf("invalid meta page signature")
	}
	pm.ResetTo(flushed)
	return nil
}

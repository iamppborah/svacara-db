package storage

import (
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
)

const (
	PageSize   = 4096
	MaxChunkMB = 64
)

type PageMgr struct {
	fd      int
	file    *os.File
	mmap    mmapState
	page    pageState
}

type mmapState struct {
	total  int
	chunks [][]byte
}

type pageState struct {
	flushed uint64
	temp    [][]byte
	updates map[uint64][]byte
}

func NewPageMgr(file *os.File) *PageMgr {
	return &PageMgr{
		fd:   int(file.Fd()),
		file: file,
		page: pageState{
			updates: make(map[uint64][]byte),
		},
	}
}

func (pm *PageMgr) Read(ptr uint64) []byte {
	if node, ok := pm.page.updates[ptr]; ok {
		return node
	}
	start := uint64(0)
	for _, chunk := range pm.mmap.chunks {
		end := start + uint64(len(chunk))/PageSize
		if ptr < end {
			offset := PageSize * (ptr - start)
			return chunk[offset : offset+PageSize]
		}
		start = end
	}
	panic(fmt.Sprintf("bad ptr: %d", ptr))
}

func (pm *PageMgr) Alloc(page []byte) uint64 {
	ptr := pm.page.flushed + uint64(len(pm.page.temp))
	pm.page.temp = append(pm.page.temp, page)
	return ptr
}

func (pm *PageMgr) Write(ptr uint64) []byte {
	if node, ok := pm.page.updates[ptr]; ok {
		return node
	}
	existing := make([]byte, PageSize)
	copy(existing, pm.Read(ptr))
	pm.page.updates[ptr] = existing
	return existing
}

func (pm *PageMgr) Free(ptr uint64) {
	delete(pm.page.updates, ptr)
}

func (pm *PageMgr) ExtendMMap(size int) error {
	if size <= pm.mmap.total {
		return nil
	}
	alloc := pm.mmap.total
	if alloc < MaxChunkMB<<20 {
		alloc = MaxChunkMB << 20
	}
	for pm.mmap.total+alloc < size {
		alloc *= 2
	}
	chunk, err := syscall.Mmap(
		pm.fd, int64(pm.mmap.total), alloc,
		syscall.PROT_READ, syscall.MAP_SHARED,
	)
	if err != nil {
		return fmt.Errorf("mmap: %w", err)
	}
	pm.mmap.total += alloc
	pm.mmap.chunks = append(pm.mmap.chunks, chunk)
	return nil
}

func (pm *PageMgr) FlushPages() (int, error) {
	if len(pm.page.temp) == 0 && len(pm.page.updates) == 0 {
		return 0, nil
	}

	fileSize := int64(pm.page.flushed+uint64(len(pm.page.temp))) * PageSize
	if err := pm.ExtendMMap(int(fileSize)); err != nil {
		return 0, err
	}

	var writeCount int
	for ptr, page := range pm.page.updates {
		offset := int64(ptr) * PageSize
		n, err := pm.file.WriteAt(page, offset)
		if err != nil {
			return writeCount, fmt.Errorf("write page %d: %w", ptr, err)
		}
		if n != PageSize {
			return writeCount, fmt.Errorf("short write page %d: %d bytes", ptr, n)
		}
		writeCount++
	}

	if len(pm.page.temp) > 0 {
		offset := int64(pm.page.flushed) * PageSize
		totalBytes := 0
		for _, page := range pm.page.temp {
			totalBytes += len(page)
		}
		buf := make([]byte, totalBytes)
		pos := 0
		for _, page := range pm.page.temp {
			copy(buf[pos:], page)
			pos += len(page)
		}
		n, err := pm.file.WriteAt(buf, offset)
		if err != nil {
			return writeCount, fmt.Errorf("write temp pages: %w", err)
		}
		if n != totalBytes {
			return writeCount, fmt.Errorf("short temp write: %d of %d", n, totalBytes)
		}
		tempCount := len(pm.page.temp)
		pm.page.flushed += uint64(tempCount)
		pm.page.temp = pm.page.temp[:0]
		writeCount += tempCount
	}
	return writeCount, nil
}

func (pm *PageMgr) ClearUpdates() {
	pm.page.updates = make(map[uint64][]byte)
	pm.page.temp = pm.page.temp[:0]
}

func (pm *PageMgr) ResetTo(flushed uint64) {
	pm.page.flushed = flushed
	pm.page.temp = pm.page.temp[:0]
	pm.page.updates = make(map[uint64][]byte)
}

func (pm *PageMgr) Flushed() uint64 {
	return pm.page.flushed
}

func (pm *PageMgr) Close() error {
	for _, chunk := range pm.mmap.chunks {
		syscall.Munmap(chunk)
	}
	pm.mmap.chunks = nil
	return nil
}

const sigSize = 16

const (
	metaSig     = "SvacaraDB01\x00\x00\x00\x00\x00"
	metaOffRoot = sigSize
	metaOffUsed = sigSize + 8
	metaOffFree = sigSize + 16
	metaSize    = sigSize + 8 + 8 + 8 + 8 + 8 + 8
)

func writeMeta(page []byte, root uint64, flushed uint64, headPage, headSeq, tailPage, tailSeq uint64) {
	copy(page[:sigSize], []byte(metaSig))
	binary.LittleEndian.PutUint64(page[metaOffRoot:], root)
	binary.LittleEndian.PutUint64(page[metaOffUsed:], flushed)
	binary.LittleEndian.PutUint64(page[metaOffFree:], headPage)
	binary.LittleEndian.PutUint64(page[metaOffFree+8:], headSeq)
	binary.LittleEndian.PutUint64(page[metaOffFree+16:], tailPage)
	binary.LittleEndian.PutUint64(page[metaOffFree+24:], tailSeq)
}

func readMeta(page []byte) (root uint64, flushed uint64, headPage, headSeq, tailPage, tailSeq uint64, ok bool) {
	if string(page[:sigSize]) != metaSig {
		return 0, 0, 0, 0, 0, 0, false
	}
	root = binary.LittleEndian.Uint64(page[metaOffRoot:])
	flushed = binary.LittleEndian.Uint64(page[metaOffUsed:])
	headPage = binary.LittleEndian.Uint64(page[metaOffFree:])
	headSeq = binary.LittleEndian.Uint64(page[metaOffFree+8:])
	tailPage = binary.LittleEndian.Uint64(page[metaOffFree+16:])
	tailSeq = binary.LittleEndian.Uint64(page[metaOffFree+24:])
	return root, flushed, headPage, headSeq, tailPage, tailSeq, true
}

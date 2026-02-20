package interfaces

type Decompressor interface {
	Decompress([]byte) ([]byte, error)
	Close()
}

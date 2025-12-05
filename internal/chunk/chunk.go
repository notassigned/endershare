package chunk

const CHUNK_SIZE = 256 * 1024

type Chunk struct {
	Hash    [32]byte
	Content [CHUNK_SIZE]byte
}

func FileToChunks(file []byte)

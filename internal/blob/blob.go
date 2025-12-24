package blob

import (
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"

	"github.com/zeebo/xxh3"
)

type Store struct {
	Dir string
}

func (s Store) Open(id uint64) (*os.File, error) {
	return os.Open(filepath.Join(s.Dir, strconv.FormatUint(id, 10)))
}

func (s Store) Store(blob io.Reader) (id uint64, err error) {
	err = os.MkdirAll(s.Dir, 0777)
	if err != nil {
		return
	}
	tmpFilename := filepath.Join(s.Dir, strconv.FormatUint(rand.Uint64(), 10))
	f, err := os.Create(tmpFilename)
	if err != nil {
		return
	}
	defer f.Close()

	hasher := xxh3.New()
	tee := io.TeeReader(blob, hasher)

	_, err = io.Copy(f, tee)
	if err != nil {
		return
	}
	id = hasher.Sum64()
	f.Close()

	filename := filepath.Join(s.Dir, strconv.FormatUint(id, 10))
	err = os.Rename(tmpFilename, filename)
	if err != nil {
		return
	}
	return
}

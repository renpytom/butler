package archiver

import (
	"io"
	"os"
	"path/filepath"

	"github.com/itchio/wharf/pwr"
)

const (
	// ModeMask is or'd with files walked by butler
	ModeMask = 0666

	// LuckyMode is used when wiping in last-chance mode
	LuckyMode = 0777

	// DirMode is the default mode for directories created by butler
	DirMode = 0755
)

type ExtractResult struct {
	Dirs     int
	Files    int
	Symlinks int
}

type CompressResult struct {
	UncompressedSize int64
	CompressedSize   int64
}

func ExtractPath(archive string, destPath string, consumer *pwr.StateConsumer) (*ExtractResult, error) {
	var result *ExtractResult
	var err error

	stat, err := os.Lstat(archive)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(archive)
	if err != nil {
		return nil, err
	}

	defer func() {
		if cErr := file.Close(); cErr != nil && err == nil {
			err = cErr
		}
	}()

	result, err = Extract(file, stat.Size(), destPath, consumer)
	return result, err
}

func Extract(readerAt io.ReaderAt, size int64, destPath string, consumer *pwr.StateConsumer) (*ExtractResult, error) {
	return ExtractZip(readerAt, size, destPath, consumer)
}

func Mkdir(dstpath string) error {
	dirstat, err := os.Lstat(dstpath)
	if err != nil {
		// main case - dir doesn't exist yet
		err = os.MkdirAll(dstpath, DirMode)
		if err != nil {
			return err
		}
		return nil
	}

	if dirstat.IsDir() {
		// is already a dir, good!
	} else {
		// is a file or symlink for example, turn into a dir
		err = os.Remove(dstpath)
		if err != nil {
			return err
		}
		err = os.MkdirAll(dstpath, DirMode)
		if err != nil {
			return err
		}
	}

	return nil
}

func Symlink(linkname string, filename string, consumer *pwr.StateConsumer) error {
	consumer.Debugf("ln -s %s %s", linkname, filename)

	err := os.RemoveAll(filename)
	if err != nil {
		return err
	}

	err = os.Symlink(linkname, filename)
	if err != nil {
		return err
	}
	return nil
}

func CopyFile(filename string, mode os.FileMode, fileReader io.Reader, consumer *pwr.StateConsumer) error {
	consumer.Debugf("extract %s", filename)
	err := os.RemoveAll(filename)
	if err != nil {
		return err
	}

	dirname := filepath.Dir(filename)
	err = os.MkdirAll(dirname, LuckyMode)
	if err != nil {
		return err
	}

	writer, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer writer.Close()

	_, err = io.Copy(writer, fileReader)
	if err != nil {
		return err
	}

	err = os.Chmod(filename, mode)
	if err != nil {
		return err
	}
	return nil
}

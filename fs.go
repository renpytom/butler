package main

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/itchio/butler/comm"
	"github.com/itchio/wharf/archiver"
)

func mkdir(dir string) {
	comm.Debugf("mkdir -p %s", dir)

	must(os.MkdirAll(dir, archiver.DirMode))
}

func wipe(path string) {
	tries := 3
	sleepDuration := time.Second * 2

	for tries > 0 {
		err := tryWipe(path)
		if err == nil {
			break
		}

		comm.Logf("While removing: %s", err.Error())

		comm.Logf("Trying to brute-force permissions, who knows...")
		err = tryChmod(path)
		if err != nil {
			comm.Logf("While bruteforcing: %s", err.Error())
		}

		comm.Logf("Sleeping for a bit before we retry...")

		time.Sleep(sleepDuration)
		sleepDuration *= 2
	}
}

func tryWipe(path string) error {
	comm.Debugf("rm -rf %s", path)
	return os.RemoveAll(path)
}

func tryChmod(path string) error {
	// oh yeah?
	chmodAll := func(childpath string, f os.FileInfo, err error) error {
		if err != nil {
			// ignore walking errors
			return nil
		}

		// don't ignore chmodding errors
		return os.Chmod(childpath, os.FileMode(archiver.LuckyMode))
	}

	return filepath.Walk(path, chmodAll)
}

// Does not preserve users, nor permission, except the executable bit
func ditto(src string, dst string) {
	comm.Debugf("rsync -a %s %s", src, dst)

	totalSize := int64(0)
	doneSize := int64(0)
	oldProgress := 0.0

	inc := func(_ string, f os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		totalSize += f.Size()
		return nil
	}

	onFile := func(path string, f os.FileInfo, err error) error {
		if err != nil {
			comm.Logf("ignoring error %s", err.Error())
			return nil
		}

		rel, err := filepath.Rel(src, path)
		must(err)

		dstpath := filepath.Join(dst, rel)
		mode := f.Mode()

		switch {
		case mode.IsDir():
			dittoMkdir(dstpath)

		case mode.IsRegular():
			dittoReg(path, dstpath, os.FileMode(f.Mode()&archiver.LuckyMode|archiver.ModeMask))

		case (mode&os.ModeSymlink > 0):
			dittoSymlink(path, dstpath, f)
		}

		comm.Debug(rel)

		doneSize += f.Size()

		progress := float64(doneSize) / float64(totalSize)
		if progress-oldProgress > 0.01 {
			oldProgress = progress
			comm.Progress(progress)
		}

		return nil
	}

	rootinfo, err := os.Lstat(src)
	must(err)

	if rootinfo.IsDir() {
		totalSize = 0
		comm.Logf("Counting files in %s...", src)
		filepath.Walk(src, inc)

		comm.Logf("Mirroring...")
		filepath.Walk(src, onFile)
	} else {
		totalSize = rootinfo.Size()
		onFile(src, rootinfo, nil)
	}

	comm.EndProgress()
}

func dittoMkdir(dstpath string) {
	comm.Debugf("mkdir %s", dstpath)
	must(archiver.Mkdir(dstpath))
}

func dittoReg(srcpath string, dstpath string, mode os.FileMode) {
	comm.Debugf("cp -f %s %s", srcpath, dstpath)
	must(os.RemoveAll(dstpath))

	writer, err := os.OpenFile(dstpath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	must(err)
	defer writer.Close()

	reader, err := os.Open(srcpath)
	must(err)
	defer reader.Close()

	_, err = io.Copy(writer, reader)
	must(err)

	must(os.Chmod(dstpath, mode))
}

func dittoSymlink(srcpath string, dstpath string, f os.FileInfo) {
	must(os.RemoveAll(dstpath))

	linkname, err := os.Readlink(srcpath)
	must(err)

	comm.Debugf("ln -s %s %s", linkname, dstpath)
	must(os.Symlink(linkname, dstpath))
}

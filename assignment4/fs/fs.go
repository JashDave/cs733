package fs

import (
	_ "fmt"
	"sync"
	"time"
)

type FileInfo struct {
	filename   string
	contents   []byte
	version    int
	absexptime time.Time
	timer      *time.Timer
}

type FS struct {
	sync.RWMutex
	dir map[string]*FileInfo
}


type FileSystem struct {
	fs *FS
	gversion int // global version
}

func GetFileSystem(no_of_files int) *FileSystem{
	ret := new(FileSystem)
	ret.fs = &FS{dir: make(map[string]*FileInfo, no_of_files)}
	ret.gversion = 0 // global version
	return ret
}

func (fi *FileInfo) cancelTimer() {
	if fi.timer != nil {
		fi.timer.Stop()
		fi.timer = nil
	}
}


func (fsys *FileSystem) ProcessMsg(msg *Msg) *Msg {
	switch msg.Kind {
	case 'r':
		return fsys.processRead(msg)
	case 'w':
		return fsys.processWrite(msg)
	case 'c':
		return fsys.processCas(msg)
	case 'd':
		return fsys.processDelete(msg)
	}

	// Default: Internal error. Shouldn't come here since
	// the msg should have been validated earlier.
	return &Msg{Kind: 'I'}
}

func (fsys *FileSystem) processRead(msg *Msg) *Msg {
	fsys.fs.RLock()
	defer fsys.fs.RUnlock()
	if fi := fsys.fs.dir[msg.Filename]; fi != nil {
		remainingTime := 0
		if fi.timer != nil {
			remainingTime := int(fi.absexptime.Sub(time.Now()))
			if remainingTime < 0 {
				remainingTime = 0
			}
		}
		return &Msg{
			Kind:     'C',
			Filename: fi.filename,
			Contents: fi.contents,
			Numbytes: len(fi.contents),
			Exptime:  remainingTime,
			Version:  fi.version,
		}
	} else {
		return &Msg{Kind: 'F'} // file not found
	}
}

func  (fsys *FileSystem) internalWrite(msg *Msg) *Msg {
	fi := fsys.fs.dir[msg.Filename]
	if fi != nil {
		fi.cancelTimer()
	} else {
		fi = &FileInfo{}
	}

	fsys.gversion += 1
	fi.filename = msg.Filename
	fi.contents = msg.Contents
	fi.version = fsys.gversion

	var absexptime time.Time
	if msg.Exptime > 0 {
		dur := time.Duration(msg.Exptime) * time.Second
		absexptime = time.Now().Add(dur)
		timerFunc := func(name string, ver int) func() {
			return func() {
				fsys.processDelete(&Msg{Kind: 'D',
					Filename: name,
					Version:  ver})
			}
		}(msg.Filename, fsys.gversion)

		fi.timer = time.AfterFunc(dur, timerFunc)
	}
	fi.absexptime = absexptime
	fsys.fs.dir[msg.Filename] = fi

	return ok(fsys.gversion)
}

func (fsys *FileSystem) processWrite(msg *Msg) *Msg {
	fsys.fs.Lock()
	defer fsys.fs.Unlock()
	return fsys.internalWrite(msg)
}

func (fsys *FileSystem) processCas(msg *Msg) *Msg {
	fsys.fs.Lock()
	defer fsys.fs.Unlock()

	if fi := fsys.fs.dir[msg.Filename]; fi != nil {
		if msg.Version != fi.version {
			return &Msg{Kind: 'V', Version: fi.version}
		}
	}
	return fsys.internalWrite(msg)
}

func (fsys *FileSystem) processDelete(msg *Msg) *Msg {
	fsys.fs.Lock()
	defer fsys.fs.Unlock()
	fi := fsys.fs.dir[msg.Filename]
	if fi != nil {
		if msg.Version > 0 && fi.version != msg.Version {
			// non-zero msg.Version indicates a delete due to an expired timer
			return nil // nothing to do
		}
		fi.cancelTimer()
		delete(fsys.fs.dir, msg.Filename)
		return ok(0)
	} else {
		return &Msg{Kind: 'F'} // file not found
	}

}

func ok(version int) *Msg {
	return &Msg{Kind: 'O', Version: version}
}

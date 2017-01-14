package p2p

import (
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"os"
	"path"
)

// Basic user implementation

type User struct {
	app  App
	dht  *DHTRouter
	dir  string
	name string
	peer *Peer
}

func (u *User) Name() string { return u.name }

func NewUser(app App, dir, name string) *User {
	return &User{
		app:  app,
		dir:  dir,
		name: name,
	}
}

func (u *User) PeerAddr() string {
	if u.peer == nil {
		return ""
	}
	return u.peer.addr
}

func (u *User) RouterAddr() string {
	if u.dht == nil {
		return ""
	}
	return u.dht.addr
}

func (u *User) Boot() error {
	// Boot the router
	if err := u.dht.boot(); err != nil {
		return err
	}
	// Start the peer
	if err := u.peer.start(); err != nil {
		return err
	}
	return nil
}

func (u *User) Start(bootAddr string) error {
	// Start the router
	if err := u.dht.start(bootAddr); err != nil {
		return err
	}
	// Start peer
	if err := u.peer.start(); err != nil {
		return err
	}
	return nil
}

func (u *User) Stop() error {
	// Stop peer
	if err := u.peer.stop(); err != nil {
		return err
	}
	// Stop the router
	if err := u.dht.stop(); err != nil {
		return err
	}
	return nil
}

func (u *User) FindFile(key []byte) (int64, io.Reader, error) {
	filepath := u.Path(Hexstr(key))
	file, err := os.Open(filepath)
	if err != nil {
		return 0, nil, err
	}
	fileInfo, err := file.Stat()
	if err != nil {
		return 0, nil, err
	}
	filesize := fileInfo.Size()
	return filesize, file, nil
}

// This is called when the peer gets a file
// that was queried for.. The file data can
// be read from the io.Reader (e.g. TCP conn)
// This method is minimal since an application
// will make decisions about handling the file data
func (u *User) HandleFile(err error, filesize int64, key []byte, r io.Reader) {
	fileOutput := NewFileOutput(err, filesize, key, r)
	u.app.Handler(fileOutput)
}

func (u *User) StoreFile(filesize int64, key []byte, r io.Reader) error {
	if filesize <= 0 {
		return errors.New("File size must be greater than 0")
	}
	filepath := u.Path(Hexstr(key))
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	if _, err = io.CopyN(file, r, filesize); err != nil {
		return err
	}
	return nil
}

func (u *User) FindDir(key []byte) ([]string, []int64, io.Reader, error) {
	dirname := Hexstr(key)
	dirpath := u.Path(dirname)
	dir, err := ioutil.ReadDir(dirpath)
	if err != nil {
		return nil, nil, nil, err
	}
	filenames := make([]string, len(dir))
	filesizes := make([]int64, len(dir))
	files := make([]io.Reader, len(dir))
	for i, fileInfo := range dir {
		filepath := u.Path(dirname, fileInfo.Name())
		files[i], err = os.Open(filepath)
		if err != nil {
			return nil, nil, nil, err
		}
		filenames[i] = fileInfo.Name()
		filesizes[i] = fileInfo.Size()
	}
	r := io.MultiReader(files...)
	return filenames, filesizes, r, nil
}

func (u *User) HandleDir(err error, filenames []string, filesizes []int64, key []byte, r io.Reader) {
	dirOutput := NewDirOutput(err, filenames, filesizes, key, r)
	u.app.Handler(dirOutput)
}

func (u *User) StoreDir(filenames []string, filesizes []int64, key []byte, r io.Reader) error {
	if len(filenames) != len(filesizes) {
		return errors.New("Number of filenames does not equal number of filesizes")
	}
	for i, filename := range filenames {
		if filesizes[i] <= 0 {
			// return errors.New("File size must be greater than 0")
			continue
		}
		filepath := u.Path(Hexstr(key), filename)
		file, err := os.Create(filepath)
		if err != nil {
			return err
		}
		if _, err := io.CopyN(file, r, filesizes[i]); err != nil {
			// return err
			continue
		}
	}
	return nil
}

// Requests

func (u *User) FindFileRequest(filename, username string) error {
	ids, err := u.dht.getNameIds(username)
	if err != nil {
		return err
	}
	key := Hash([]byte(username + filename))
	for _, to := range ids {
		if err = u.peer.FindFileRequest(key, to); err != nil {
			//..
		}
	}
	return nil
}

func (u *User) StoreFileRequest(filename string) error {
	filepath := u.NamePath(filename)
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}
	filesize := fileInfo.Size()
	key := Hash([]byte(u.name + filename))
	// TODO: store file
	if err := u.peer.StoreFileRequest(filesize, key, file); err != nil {
		return err
	}
	return nil
}

func (u *User) FindDirRequest(dirname, username string) error {
	ids, err := u.dht.getNameIds(username)
	if err != nil {
		return err
	}
	key := Hash([]byte(username + dirname))
	for _, to := range ids {
		if err = u.peer.FindDirRequest(key, to); err != nil {
			//..
		}
	}
	return nil
}

func (u *User) StoreDirRequest(dirname string) error {
	dirpath := u.NamePath(dirname)
	dir, err := ioutil.ReadDir(dirpath)
	if err != nil {
		return err
	}
	key := Hash([]byte(u.name + dirname))
	filenames := make([]string, len(dir))
	filesizes := make([]int64, len(dir))
	files := make([]io.Reader, len(dir))
	for i, fileInfo := range dir {
		filepath := u.Path(dirname, fileInfo.Name())
		files[i], err = os.Open(filepath)
		if err != nil {
			return err
		}
		filenames[i] = fileInfo.Name()
		filesizes[i] = fileInfo.Size()
	}
	r := io.MultiReader(files...)
	// TODO: store dir
	if err = u.peer.StoreDirRequest(filenames, filesizes, key, r); err != nil {
		return err
	}
	return nil
}

// Path

func (u *User) Path(args ...string) string {
	args = append([]string{u.dir}, args...)
	return path.Join(args...)
}

func (u *User) NamePath(args ...string) string {
	args = append([]string{u.dir, u.name}, args...)
	return path.Join(args...)
}

// App

type App interface {
	Handler(Output) error
}

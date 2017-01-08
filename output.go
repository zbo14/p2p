package p2p

import (
	"encoding/json"
	"github.com/pkg/errors"
	"io"
)

type Output interface {
	Err() error
	IsErr() bool
	IsOutput()
	Key() []byte
	Write(string, io.Writer) error
}

type output struct {
	err error
	key []byte
	r   io.Reader
}

func NewOutput(err error, key []byte, r io.Reader) output {
	return output{err, key, r}
}

func (o output) Err() error {
	return o.err
}

func (o output) IsErr() bool {
	return o.err != nil
}

func (o output) Key() []byte {
	return o.key
}

type FileOutput struct {
	filesize int64
	output
}

type DirOutput struct {
	filenames []string
	filesizes []int64
	output
}

func NewFileOutput(err error, filesize int64, key []byte, r io.Reader) *FileOutput {
	return &FileOutput{
		filesize: filesize,
		output:   NewOutput(err, key, r),
	}
}

func NewDirOutput(err error, filenames []string, filesizes []int64, key []byte, r io.Reader) *DirOutput {
	return &DirOutput{
		filenames: filenames,
		filesizes: filesizes,
		output:    NewOutput(err, key, r),
	}
}

type FileHeader struct {
	Filename string `json:"file_name"`
	Filesize int64  `json:"file_size"`
}

type DirHeader struct {
	Dirname   string   `json:"dir_name"`
	Filenames []string `json:"file_names"`
	Filesizes []int64  `json:"file_sizes"`
}

func NewFileHeader(filename string, filesize int64) *FileHeader {
	return &FileHeader{
		Filename: filename,
		Filesize: filesize,
	}
}

func NewDirHeader(dirname string, filenames []string, filesizes []int64) *DirHeader {
	return &DirHeader{
		Dirname:   dirname,
		Filenames: filenames,
		Filesizes: filesizes,
	}
}

func AssertFileOutput(O Output) *FileOutput {
	return O.(*FileOutput)
}

func AssertDirOutput(O Output) *DirOutput {
	return O.(*DirOutput)
}

func (_ *FileOutput) IsOutput() {}
func (_ *DirOutput) IsOutput()  {}

func (f *FileOutput) Write(filename string, w io.Writer) error {
	header := NewFileHeader(filename, f.filesize)
	enc := json.NewEncoder(w)
	if err := enc.Encode(header); err != nil {
		return err
	}
	n, err := io.CopyN(w, f.r, f.filesize)
	if err != nil {
		return err
	} else if n != f.filesize {
		return errors.New("Could not write entire file")
	}
	return nil
}

func (d *DirOutput) Write(dirname string, w io.Writer) error {
	header := NewDirHeader(dirname, d.filenames, d.filesizes)
	enc := json.NewEncoder(w)
	if err := enc.Encode(header); err != nil {
		return err
	}
	for _, filesize := range d.filesizes {
		n, err := io.CopyN(w, d.r, filesize)
		if err != nil {
			return err
		} else if n != filesize {
			return errors.New("Could not write entire file")
		}
	}
	return nil
}

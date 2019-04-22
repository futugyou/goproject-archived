package main

type File struct {
	fd int
}

func OpenFile(name string) (f *File, err error)       { return nil, nil }
func CloseFile(f *File) error                         { return nil }
func ReadFile(f *File, offset int64, data []byte) int { return 0 }
func (f *File) Close() error                          { return nil }
func (f *File) Read(offset int64, data []byte) int    { return 0 }

func main() {
	var closeF = (*File).Close
	var readF = (*File).Read
	f, _ := OpenFile("")
	readF(f, 0, nil)
	closeF(f)

}

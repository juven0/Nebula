package files

type File struct {
	FileId      string
	FileName    string
	FileSize    int64
	FileContent []byte
	Timestamp   string
	Owner       string
}

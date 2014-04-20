/* Logfiler provides functionalities for writing and reading the recovery logs.
   The format of the log file is:
        int64: number of records
        an array of records
   The format of the record is:
        int64: number of bytes for the data field
        data field: bytes
*/

package logfile

import (
    "encoding/binary"
    "errors"
    "os"
)

const (
    filename string = "logs.log"
)

type Logfiler struct {
    file *os.File
    size int64          // number of records in the log
    tailOffset int64    // byte offset for the tail
    readOffset int64    // byte offset for the read pointer
}

// this opens a new empty logfile
func NewLogger() (*Logfiler, error) {
    l := new(Logfiler)
    l.tailOffset = 8
    l.readOffset = 8

    var f *os.File
    var err error
    if f, err = os.Create(filename); err == nil {
        l.file = f
        l.size = 0
        if err = l.writeInt64(l.size, 0); err != nil {
            return nil, err
        }
    } else {
        return nil, err
    }

    return l, nil
}

// this recovers from the existing logfile
func Recover() (*Logfiler, error) {
    l := new(Logfiler)
    l.tailOffset = 8
    l.readOffset = 8

    var f *os.File
    var err error
    if f, err = os.OpenFile(filename, os.O_RDWR, 0666); err == nil {
        var num int64
        l.file = f
        if num, err = l.readInt64(0); err == nil {
            l.size = num
        } else {
            return nil, err
        }
    } else {
        return nil, err
    }

    var size int64
    if size, err = l.readInt64(0); err != nil {
        return nil, err
    }
    l.size = size

    for ; size > 0; size-- {
        if size, err = l.readInt64(l.tailOffset); err != nil {
            return nil, err
        }
        l.tailOffset += (l.tailOffset + 8)
    }

    return l, nil
}

func (l *Logfiler) AppendRecord(b []byte) error {
    s := int64(len(b))
    if err := l.writeInt64(s, l.tailOffset); err != nil {
        return err
    }
    if _, err := l.file.WriteAt(b, l.tailOffset + 8); err != nil {
        return err
    }
    if err := l.writeInt64(l.size + 1, 0); err != nil {
        return err
    }
    l.tailOffset += (s + 8)
    l.size++
    return nil
}

func (l *Logfiler) NextRecord() ([]byte, error) {
    if l.readOffset == l.tailOffset {
        return nil, errors.New("reach the end of the log")
    } else {
        var size int64
        var err error
        if size, err = l.readInt64(l.readOffset); err != nil {
            return nil, err
        }
        buf := make([]byte, size)
        if _, err := l.file.ReadAt(buf, l.readOffset + 8); err != nil {
            return nil, err
        } else {
            l.readOffset += (size + 8)
            return buf, nil
        }
    }
}

func (l *Logfiler) Close() error {
    return l.file.Close()
}

// write num at offset
func (l *Logfiler) writeInt64(num, offset int64) error {
    b := make([]byte, 8)
    binary.PutVarint(b, num)
    _, err := l.file.WriteAt(b, offset)
    return err
}

// read an in64 from offset
func (l *Logfiler) readInt64(offset int64) (int64, error) {
    b := make([]byte, 8)
    if _, err := l.file.ReadAt(b, offset); err == nil {
        var num int64
        var n int
        num, n = binary.Varint(b)
        if n <= 0 {
            return 0, errors.New("Logfiler readInt64 error")
        } else {
            return num, nil
        }
    } else {
        return 0, err
    }
}

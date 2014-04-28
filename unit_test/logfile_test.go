package test

import (
    "os"
    "log"
    "testing"
    "time"
    "encoding/json"
    "math/rand"
    "github.com/gobby/src/logfile"
)

const (
    arrLen int = 20
    numArr int = 20
)

var LOGE = log.New(os.Stderr, "", log.Lshortfile|log.Lmicroseconds)

func TestNewLooger(t *testing.T) {
    LOGE.Println("TestNewLogger......")
    if l, err := logfile.NewLogger(); err == nil {
        l.Close()
    } else {
        LOGE.Println("TestNewLogger error")
    }
}

func TestAppend1(t *testing.T) {
    LOGE.Println("TestAppend1......")
    if l, err := logfile.NewLogger(); err == nil {
        var arrx, arry []int
        arrx = randIntSlice()
        b, _ := json.Marshal(arrx)
        l.AppendRecord(b)
        bres, _ := l.NextRecord()
        json.Unmarshal(bres, &arry)
        for i := 0; i < arrLen; i++ {
            if arrx[i] != arry[i] {
                LOGE.Println("TestAppend1 compare error!")
            }
        }
        l.Close()
    } else {
        LOGE.Println("TestAppend1 new error")
    }
}

func TestAppend2(t *testing.T) {
    LOGE.Println("TestAppend2......")
    if l, err := logfile.NewLogger(); err == nil {
        var arrx, arry [numArr][]int
        for i := 0; i < numArr; i++ {
            arrx[i] = randIntSlice()
            b, _ := json.Marshal(arrx[i])
            l.AppendRecord(b)
        }
        for i := 0; i < numArr; i++ {
            bres, _ := l.NextRecord()
            json.Unmarshal(bres, &arry[i])
            for j := 0; j < arrLen; j++ {
                if arrx[i][j] != arry[i][j] {
                    LOGE.Println("TestAppend1 compare error!")
                }
            }
        }
        l.Close()
    } else {
        LOGE.Println("TestAppend1 new error")
    }
}

func randIntSlice() ([]int) {
    rand.Seed(time.Now().Unix())
    a := make([]int, arrLen)
    for i := 0; i < arrLen; i++ {
        a[i] = rand.Int()
    }
    return a
}

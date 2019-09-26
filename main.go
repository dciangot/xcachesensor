// Copyright © 2019 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type DataIn struct {
	Code int8
}

type FileInfo struct {
	Bs uint64
	Fs uint64
}

type MsgMon struct {
	Time uint32
}

type Accesses struct {
	Acc uint32
}

type MMon struct {
	Attach      uint64
	Detach      uint64
	BytesOnDisk int64
	BytesOnRam  int64
	BytesMissed int64
}

func converEnvToInt(s string) int64 {
	integer, _ := strconv.Atoi(s)
	//if err != nil {
	//	CheckError(err)
	//}

	return int64(integer)
}

var (
	path     = flag.String("path", os.Getenv("CACHE_META_PATH"), "The path where cache metadata are stored")
	interval = flag.Int64("interval", converEnvToInt(os.Getenv("INTERVAL")), "Collect only new accesses within last INTERVAL seconds")

	logger = log.New(os.Stdout, "[Producer] ", log.LstdFlags)
)

// CheckError ...
func CheckError(err error) {
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(0)
	}
}

func read(filename string, f os.FileInfo, err error) error {

	if f.IsDir() {
		return nil
	}

	if filepath.Ext(filename) == ".cinfo" {
		buf, err := ioutil.ReadFile(filename)
		if err != nil && err != io.EOF {
			return err
		}

		if _, _, err := translate(buf, filename); err != nil {
			fmt.Println("failed to retrieve stats:", err)
			return err
		}
	}
	return nil
}

func main() {
	flag.Parse()

	// var buf []byte

	// buf, err := ioutil.ReadFile(*path)
	// if err != nil && err != io.EOF {
	// 	CheckError(err)
	// }

	err := filepath.Walk(*path, read)
	if err != nil && err != io.EOF {
		CheckError(err)
	}

	// if _, _, err := translate(buf, *path); err != nil {
	// 	fmt.Println("failed to retrieve stats:", err)
	// }

	// d, err := os.Open(*path)
	// if err != nil {
	// 	fmt.Println(err)
	// 	os.Exit(1)
	// }
	// defer d.Close()
	// fi, err := d.Readdir(-1)
	// if err != nil {
	// 	fmt.Println(err)
	// 	os.Exit(1)
	// }

	// files := make(chan string, 8)
	// var wg sync.WaitGroup

	// for _, fi := range fi {
	// 	if fi.Mode().IsRegular() && filepath.Ext(fi.Name()) == ".cinfo" {
	// 		fmt.Println(fi.Name(), fi.Size(), "bytes")

	// 		files <- fi.Name()
	// 		//if err = read(fi.Name()); err != nil {
	// 		//	fmt.Println("Function read failed:", err)
	// 		//}
	// 		wg.Add(1)
	// 		go read(<-files, &wg)
	// 	}
	// }

	// wg.Wait()
}

func translate(b []byte, filename string) (Accesses, []MMon, error) {

	r := bytes.NewReader(b[:3])
	var data DataIn

	if err := binary.Read(r, binary.LittleEndian, &data); err != nil {
		fmt.Println("binary.Read failed:", err)
	}

	fmt.Println("Reading ===>", filename)
	fmt.Println("version", data.Code)

	r = bytes.NewReader(b[4:])
	var fileInfo FileInfo

	if err := binary.Read(r, binary.LittleEndian, &fileInfo); err != nil {
		fmt.Println("binary.Read failed:", err)
	}

	fmt.Println("file size", fileInfo.Fs)
	fmt.Println("bucket size", fileInfo.Bs)

	var buckets int

	if fileInfo.Bs != 0 {
		buckets = (int(fileInfo.Fs)-1)/int(fileInfo.Bs) + 1
		fmt.Println("buckets", buckets)

		StateVectorLengthInBytes := (buckets-1)/8 + 1

		// // jump over  disk written state vector and checksum
		jump := 32 + StateVectorLengthInBytes + 4

		r = bytes.NewReader(b[jump:])

		var msg MsgMon

		if err := binary.Read(r, binary.LittleEndian, &msg); err != nil {
			CheckError(err)
		}

		t := time.Unix(int64(msg.Time), 0)

		fmt.Println("time", int64(msg.Time), "about", time.Now().Sub(t).Seconds(), "seconds ago")

		if int64(time.Now().Sub(t).Seconds()) > *interval {
			var acc Accesses
			var ms []MMon
			return acc, ms, errors.New("no recent entry available")
		}

		var acc Accesses

		r = bytes.NewReader(b[jump+8:])

		if err := binary.Read(r, binary.LittleEndian, &acc); err != nil {
			CheckError(err)
		}

		fmt.Println("# of accesses", acc.Acc)
		fmt.Println("++++++ Access details +++++")

		var msv []MMon
		var ms MMon
		jump = jump + 16
		for n := 1; n <= int(acc.Acc); n++ {

			r = bytes.NewReader(b[jump : jump+40])
			if err := binary.Read(r, binary.LittleEndian, &ms); err != nil {
				CheckError(err)
			}

			fmt.Println("access #", n)
			fmt.Println("attach", ms.Attach)
			fmt.Println("detach", ms.Detach)
			fmt.Println("BytesOnDisk", ms.BytesOnDisk)
			fmt.Println("BytesOnRam", ms.BytesOnRam)
			fmt.Println("BytesMissed", ms.BytesMissed)
			fmt.Println("------------")

			msv = append(msv, ms)
			jump = jump + 40
		}

		return acc, msv, nil

	} else {
		fmt.Println("no buckets available")
		var acc Accesses
		var ms []MMon

		return acc, ms, errors.New("no buckets available")
	}
}

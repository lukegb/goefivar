// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"unicode/utf16"

	"github.com/lukegb/goefivar/efiboot"
	"github.com/lukegb/goefivar/efivar"
)

var (
	unicodeArgs = flag.Bool("unicode_data", true, "Treat optional data as UCS-2/UTF-16")
)

func main() {
	flag.Parse()

	if !efivar.Supported() {
		fmt.Fprintf(os.Stderr, "EFI variables are not supported on this system.\n")
		os.Exit(1)
	}

	if flag.NArg() != 1 {
		bos, err := efiboot.BootOptions()
		if err != nil {
			log.Fatalf("BootOptions: %v", err)
		}

		fmt.Fprintf(os.Stderr, "%s [BootXXXX]\n\nAvailable boot options:\n", os.Args[0])
		for _, bo := range bos {
			fmt.Fprintf(os.Stderr, "  - %s (%s)\n", bo.Variable.Name, bo.LoadOpt.Description)
		}

		os.Exit(1)
	}

	v, err := efivar.Get(efivar.GlobalUUID, flag.Arg(0))
	switch {
	case os.IsNotExist(err):
		log.Fatalf("No such boot option %v", flag.Arg(0))
	case err != nil:
		log.Fatalf("Get(%v, %q): %v", efivar.GlobalUUID, flag.Arg(0), err)
	}

	lo, err := efiboot.FromVariable(v)
	if err != nil {
		log.Fatalf("FromVariable: %v", err)
	}

	f, err := ioutil.TempFile("", "efibootedit")
	if err != nil {
		log.Fatalf("TempFile: %v", err)
	}
	fpath := f.Name()
	defer os.Remove(fpath)

	data := lo.OptionalData.InterpretAsUTF8()
	if *unicodeArgs {
		data = lo.OptionalData.InterpretAsUCS2()
	}

	if _, err := f.Write(append([]byte(data), '\n')); err != nil {
		log.Fatalf("Write: %v", err)
	}
	if err := f.Close(); err != nil {
		log.Fatalf("Close: %v", err)
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim" // sorry.
	}
	cmd := exec.Command(editor, fpath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("Running editor %v: %v", cmd.Args, err)
	}

	newData, err := ioutil.ReadFile(fpath)
	if err != nil {
		log.Fatalf("ReadFile: %v", err)
	}
	newData = newData[:len(newData)-1] // strip trailing newline

	if *unicodeArgs {
		d16 := utf16.Encode([]rune(string(newData)))
		dataBytes := make([]byte, len(d16)*2)
		for n, b16 := range d16 {
			dataBytes[n*2] = byte(b16 & 0xff)
			dataBytes[n*2+1] = byte((b16 >> 8) & 0xff)
		}
		lo.OptionalData = efiboot.OptionalData(dataBytes)
	} else {
		lo.OptionalData = efiboot.OptionalData(newData)
	}

	b, err := lo.Bytes()
	if err != nil {
		log.Fatalf("lo.Bytes: %v", err)
	}

	v.Data = b
	if err := v.Set(0644); err != nil {
		log.Fatalf("Set: %v", err)
	}
}

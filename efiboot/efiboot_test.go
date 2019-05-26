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

package efiboot

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/lukegb/goefivar/efivar"
)

var (
	// Generated with:
	// xxd -seek 0x04 -ps /sys/firmware/efi/efivars/Boot000C-8be4df61-93ca-11d2-aa0d-00e098032b8c
	archBootOptBytes = mustDecodeString(`
010000005000410072006300680020004c0069006e007500780000000401
2a0001000000000001000000000000983a0000000000b647c141bfe9274c
81c6174026e79fd00202040422005c0076006d006c0069006e0075007a00
2d006c0069006e007500780000007fff040072006f006f0074003d004c00
4100420045004c003d004c0049004e0052004f004f005400200072007700
200069006e0069007400720064003d005c0069006e00740065006c002d00
750063006f00640065002e0069006d006700200069006e00690074007200
64003d005c0069006e0069007400720061006d00660073002d006c006900
6e00750078002e0069006d00670020006e00760069006400690061002d00
640072006d002e006d006f00640065007300650074003d003100
`)
)

func mustDecodeString(s string) []byte {
	bs, err := hex.DecodeString(strings.ReplaceAll(s, "\n", ""))
	if err != nil {
		panic(err)
	}
	return bs
}

func TestBootOptions(t *testing.T) {
	if !efivar.Supported() {
		t.Skip("efivar is not supported")
	}
	los, err := BootOptions()
	if err != nil {
		t.Fatalf("BootOptions: %v", err)
	}
	if len(los) == 0 {
		t.Fatal("BootOptions returned empty slice; want at least one element")
	}
}

func TestFromVariable(t *testing.T) {
	if !efivar.Supported() {
		t.Skip("efivar is not supported")
	}
	bc, err := BootCurrent()
	if err != nil {
		t.Fatalf("BootCurrent: %v", err)
	}
	v, err := bc.Get()
	if err != nil {
		t.Fatalf("BootCurrent.Get: %v", err)
	}
	lo, err := FromVariable(v)
	if err != nil {
		t.Fatalf("FromVariable: %v", err)
	}
	if len(lo.Description) == 0 {
		t.Errorf("lo.Description is empty; want non-empty")
	}
}

func TestRoundtrip(t *testing.T) {
	lo, err := FromBytes(archBootOptBytes)
	if err != nil {
		t.Fatalf("FromBytes: %v", err)
	}

	bs, err := lo.Bytes()
	if err != nil {
		t.Fatalf("lo.Bytes: %v", err)
	}

	if len(bs) != len(archBootOptBytes) {
		t.Fatalf("len(bs) != len(archBootOptBytes): got %d; want %d", len(bs), len(archBootOptBytes))
	}

	for n := 0; n < len(bs); n++ {
		if bs[n] != archBootOptBytes[n] {
			t.Fatalf("bs != archBootOptBytes")
		}
	}
}

func TestTweakEntry(t *testing.T) {
	wantBytes := mustDecodeString(`
0100000050004d0041007200630068002000570069006e00750078000000
04012a0001000000000001000000000000983a0000000000b647c141bfe9
274c81c6174026e79fd00202040422005c0076006d006c0069006e007500
7a002d006c0069006e007500780000007fff040068656c6c6f20776f726c
64
`)

	lo, err := FromBytes(archBootOptBytes)
	if err != nil {
		t.Fatalf("FromBytes: %v", err)
	}

	lo.Description = "MArch Winux"
	lo.OptionalData = OptionalData("hello world")

	bs, err := lo.Bytes()
	if err != nil {
		t.Fatalf("lo.Bytes: %v", err)
	}

	if len(bs) != len(wantBytes) {
		t.Fatalf("len(bs) != len(wantBytes): got %d; want %d", len(bs), len(wantBytes))
	}

	for n := 0; n < len(bs); n++ {
		if bs[n] != wantBytes[n] {
			t.Fatalf("bs != wantBytes: got %v", hex.EncodeToString(bs))
		}
	}
}

func TestTweakEntryByFilePathNotImplemented(t *testing.T) {
	lo, err := FromBytes(archBootOptBytes)
	if err != nil {
		t.Fatalf("FromBytes: %v", err)
	}

	lo.FilePath = "foo"

	if _, err := lo.Bytes(); err == nil {
		t.Fatalf("lo.Bytes returned no error; was expecting 'not implemented'")
	}
}

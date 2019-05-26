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

// #cgo pkg-config: efiboot
// #include <efiboot.h>
// #include <stdlib.h>
import "C"

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"github.com/lukegb/goefivar/efivar"
)

var (
	ErrVariableCorrupted = errors.New("efiboot: variable content is not valid")

	BootCurrentName = efivar.VariableName{efivar.GlobalUUID, "BootCurrent"}
	BootNextName    = efivar.VariableName{efivar.GlobalUUID, "BootNext"}
	BootOrderName   = efivar.VariableName{efivar.GlobalUUID, "BootOrder"}
)

type Attributes uint32
type OptionalData []byte

func (d OptionalData) InterpretAsUTF8() string {
	return string(d)
}

func (d OptionalData) InterpretAsUCS2() string {
	if len(d)%2 != 0 {
		return ""
	}
	d16 := make([]uint16, len(d)/2)
	for n := 0; n < len(d); n += 2 {
		d16[n/2] = uint16(d[n]) | uint16(d[n+1]<<8)
	}
	return string(utf16.Decode(d16))
}

func (d OptionalData) String() string {
	out := make([]byte, len(d))
	for n, b := range d {
		out[n] = '.'
		if b < utf8.RuneSelf && strconv.IsPrint(rune(b)) {
			out[n] = b
		}
	}
	return string(out)
}

type LoadOpt struct {
	Attributes   Attributes
	Description  string
	FilePath     string
	rawFilePath  []byte
	OptionalData OptionalData
}

func (lo *LoadOpt) Bytes() ([]byte, error) {
	dpBytes := C.CBytes(lo.rawFilePath)
	defer C.free(dpBytes)

	dpStr, err := efivar.DevicePathToString(unsafe.Pointer(dpBytes), len(lo.rawFilePath))
	if err != nil {
		return nil, fmt.Errorf("DevicePathToString: %v", err)
	}
	if dpStr != lo.FilePath {
		return nil, fmt.Errorf("changing device path is unimplemented")
	}

	descriptionBytes := C.CString(lo.Description)
	defer C.free(unsafe.Pointer(descriptionBytes))

	optionalDataBytes := C.CBytes([]byte(lo.OptionalData))
	defer C.free(optionalDataBytes)

	sz := C.efi_loadopt_create(nil, 0, C.uint32_t(lo.Attributes), C.efidp(dpBytes), C.ssize_t(len(lo.rawFilePath)), (*C.uint8_t)(unsafe.Pointer(descriptionBytes)), (*C.uint8_t)(optionalDataBytes), C.size_t(len(lo.OptionalData)))
	if sz < 0 {
		return nil, fmt.Errorf("finding size of output buffer: efi_loadopt_create errored (rc = %d)", sz)
	}

	buf := C.malloc(C.size_t(sz))
	defer C.free(buf)

	rc := C.efi_loadopt_create((*C.uint8_t)(buf), C.ssize_t(sz), C.uint32_t(lo.Attributes), C.efidp(dpBytes), C.ssize_t(len(lo.rawFilePath)), (*C.uint8_t)(unsafe.Pointer(descriptionBytes)), (*C.uint8_t)(optionalDataBytes), C.size_t(len(lo.OptionalData)))
	if rc < 0 {
		return nil, fmt.Errorf("formatting output buffer: efi_loadopt_create errored (rc = %d)", rc)
	}

	return C.GoBytes(buf, C.int(sz)), nil
}

func FromBytes(bs []byte) (*LoadOpt, error) {
	dataPtr := C.CBytes(bs)
	defer C.free(dataPtr)

	loadOpt := (*C.efi_load_option)(dataPtr)
	loadOptSz := C.size_t(len(bs))
	ok := C.efi_loadopt_is_valid(loadOpt, loadOptSz)
	if ok != 1 {
		return nil, ErrVariableCorrupted
	}

	var optionalData *C.uint8_t
	var optionalDataSz C.size_t
	if C.efi_loadopt_optional_data(loadOpt, loadOptSz, &optionalData, &optionalDataSz) < 0 {
		return nil, ErrVariableCorrupted
	}

	dp := C.efi_loadopt_path(loadOpt, C.ssize_t(loadOptSz))
	if dp == nil {
		return nil, ErrVariableCorrupted
	}
	dpSz := C.efi_loadopt_pathlen(loadOpt, C.ssize_t(loadOptSz))

	dpStr, err := efivar.DevicePathToString(unsafe.Pointer(dp), int(dpSz))
	if err != nil {
		return nil, fmt.Errorf(" DevicePathToString: %v", err)
	}

	descPtr := C.efi_loadopt_desc(loadOpt, C.ssize_t(loadOptSz))
	if descPtr == nil {
		return nil, ErrVariableCorrupted
	}

	out := &LoadOpt{
		Attributes:   Attributes(C.efi_loadopt_attrs(loadOpt)),
		Description:  C.GoString((*C.char)(unsafe.Pointer(descPtr))),
		FilePath:     dpStr,
		rawFilePath:  C.GoBytes(unsafe.Pointer(dp), C.int(dpSz)),
		OptionalData: OptionalData(C.GoBytes(unsafe.Pointer(optionalData), C.int(optionalDataSz))),
	}
	return out, nil
}

type BootOption struct {
	Variable *efivar.Variable
	LoadOpt  *LoadOpt
}

func BootOptions() ([]*BootOption, error) {
	var bos []*BootOption
	vns, err := efivar.Variables()
	if err != nil {
		return nil, fmt.Errorf("efiboot: listing variables: %v", err)
	}
	for _, vn := range vns {
		if vn.GUID != efivar.GlobalUUID {
			continue
		}
		if !strings.HasPrefix(vn.Name, "Boot") || len(vn.Name) != len("Boot0000") || vn.Name == "BootNext" {
			continue
		}
		v, err := vn.Get()
		if err != nil {
			return nil, fmt.Errorf("efiboot: getting variable %q: %v", v.Name, err)
		}
		lo, err := FromVariable(v)
		if err != nil {
			return nil, fmt.Errorf("efiboot: parsing variable %q: %v", v.Name, err)
		}
		bos = append(bos, &BootOption{v, lo})
	}
	sort.Slice(bos, func(i, j int) bool { return bos[i].Variable.Name < bos[j].Variable.Name })
	return bos, nil
}

func FromVariable(v *efivar.Variable) (*LoadOpt, error) {
	return FromBytes(v.Data)
}

func varsInOtherVar(vn efivar.VariableName) ([]efivar.VariableName, error) {
	v, err := vn.Get()
	if err != nil {
		return nil, err
	}

	if len(v.Data)%2 == 1 {
		return nil, ErrVariableCorrupted
	}

	out := make([]efivar.VariableName, len(v.Data)/2)
	for n := range out {
		out[n] = efivar.VariableName{
			GUID: efivar.GlobalUUID,
			Name: fmt.Sprintf("Boot%02X%02X", v.Data[(n*2)+1], v.Data[(n*2)]),
		}
	}
	return out, nil
}

func varInOtherVar(vn efivar.VariableName) (efivar.VariableName, error) {
	outs, err := varsInOtherVar(vn)
	if err != nil {
		return efivar.VariableName{}, err
	}

	if len(outs) != 1 {
		return efivar.VariableName{}, ErrVariableCorrupted
	}

	return outs[0], nil
}

func BootCurrent() (efivar.VariableName, error) { return varInOtherVar(BootCurrentName) }
func BootNext() (efivar.VariableName, error)    { return varInOtherVar(BootNextName) }
func BootOrder() ([]efivar.VariableName, error) { return varsInOtherVar(BootOrderName) }

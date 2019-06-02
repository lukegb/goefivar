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

package efivar

// #cgo pkg-config: efivar
// #include "efivar.h"
// #include <stdlib.h>
import "C"

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/google/uuid"
)

var (
	ErrSomethingWentWrong = errors.New("efivar: something went wrong")

	uuidByteOrder = binary.BigEndian
	byteOrder     = endianness()

	globalUUID = C.EFI_GLOBAL_GUID
	GlobalUUID = efiToUUID(globalUUID)
)

type Attributes uint32

const (
	NonVolatile                       Attributes = C.EFI_VARIABLE_NON_VOLATILE
	BootserviceAccess                            = C.EFI_VARIABLE_BOOTSERVICE_ACCESS
	RuntimeAccess                                = C.EFI_VARIABLE_RUNTIME_ACCESS
	HardwareErrorRecord                          = C.EFI_VARIABLE_HARDWARE_ERROR_RECORD
	AuthenticatedWriteAccess                     = C.EFI_VARIABLE_AUTHENTICATED_WRITE_ACCESS
	TimeBasedAuthenticatedWriteAccess            = C.EFI_VARIABLE_TIME_BASED_AUTHENTICATED_WRITE_ACCESS
	AppendWrite                                  = C.EFI_VARIABLE_APPEND_WRITE

	//EFIVariableHasAuthHeader Attributes = C.EFI_VARIABLE_HAS_AUTH_HEADER
	//EFIVariableHasSignature             = C.EFI_VARIABLE_HAS_SIGNATURE
)

func endianness() binary.ByteOrder {
	switch C.BYTE_ORDER {
	case C.LITTLE_ENDIAN:
		return binary.LittleEndian
	case C.BIG_ENDIAN:
		return binary.BigEndian
	}
	panic(fmt.Sprintf("efivar: unknown BYTE_ORDER: %v", C.BYTE_ORDER))
}

func uuidToEFI(u uuid.UUID) C.efi_guid_t {
	ret := C.efi_guid_t{
		a: C.uint32_t(uuidByteOrder.Uint32(u[0:4])),
		b: C.uint16_t(uuidByteOrder.Uint16(u[4:6])),
		c: C.uint16_t(uuidByteOrder.Uint16(u[6:8])),
		d: C.uint16_t(byteOrder.Uint16(u[8:10])),
	}
	for n, b := range u[10:16] {
		ret.e[n] = C.uint8_t(b)
	}
	return ret
}

func efiGuidToStr(g C.efi_guid_t) (string, error) {
	var p *C.char
	ok := C.efi_guid_to_str(&g, &p)
	if ok < 0 {
		return "", ErrSomethingWentWrong
	}
	defer C.free(unsafe.Pointer(p))
	return C.GoString(p), nil
}

func efiToUUID(g C.efi_guid_t) uuid.UUID {
	var ret uuid.UUID
	uuidByteOrder.PutUint32(ret[0:4], uint32(g.a))
	uuidByteOrder.PutUint16(ret[4:6], uint16(g.b))
	uuidByteOrder.PutUint16(ret[6:8], uint16(g.c))
	byteOrder.PutUint16(ret[8:10], uint16(g.d))
	for n, b := range g.e {
		ret[10+n] = byte(b)
	}
	return ret
}

func Supported() bool {
	return C.efi_variables_supported() == 1
}

type VariableName struct {
	// GUID is the UUID of the vendor providing this variable.
	// The value stored in GlobalUUID is used for variables defined in the UEFI specification.
	GUID uuid.UUID

	// Name is the string name of this variable.
	// It is namespaced by GUID.
	Name string
}

func (vn VariableName) nameAndGuid() (*C.char, C.efi_guid_t, func()) {
	name := C.CString(vn.Name)
	guid := uuidToEFI(vn.GUID)
	return name, guid, func() {
		C.free(unsafe.Pointer(name))
	}
}

func (vn VariableName) Exists() (bool, error) {
	name, guid, cleanup := vn.nameAndGuid()
	defer cleanup()
	rc, err := C.efi_get_variable_exists(guid, name)
	switch {
	case rc == 0:
		return true, nil
	case os.IsNotExist(err):
		return false, nil
	}
	return false, err
}

func (vn VariableName) Get() (*Variable, error) {
	v := &Variable{
		VariableName: vn,
	}
	name, guid, cleanup := vn.nameAndGuid()
	defer cleanup()
	var data *C.uint8_t
	var dataSize C.size_t
	var attributes C.uint32_t
	rc, err := C.efi_get_variable(guid, name, &data, &dataSize, &attributes)
	if rc < 0 {
		return nil, err
	}
	defer C.free(unsafe.Pointer(data))
	v.Data = C.GoBytes(unsafe.Pointer(data), C.int(dataSize))
	v.Attributes = Attributes(attributes)
	return v, nil
}

func (vn VariableName) Delete() error {
	name, guid, cleanup := vn.nameAndGuid()
	defer cleanup()
	rc, err := C.efi_del_variable(guid, name)
	if rc < 0 {
		return err
	}
	return nil
}

type Variable struct {
	VariableName

	Data       []byte
	Attributes Attributes
}

func (v *Variable) Set(mode os.FileMode) error {
	name, guid, cleanup := v.nameAndGuid()
	defer cleanup()
	data := C.CBytes(v.Data)
	defer C.free(data)
	dataSize := C.size_t(len(v.Data))
	rc, err := C.efi_set_variable(guid, name, (*C.uint8_t)(data), dataSize, C.uint32_t(v.Attributes), C.mode_t(mode))
	if rc < 0 {
		return err
	}
	return nil
}

func Variables() ([]VariableName, error) {
	var guid *C.efi_guid_t
	var name *C.char
	var errno C.int
	var out []VariableName
	rc := C.efi_get_next_variable_name(&guid, &name, &errno)
	for rc > 0 {
		var v VariableName
		v.GUID = efiToUUID(*guid)
		v.Name = C.GoString(name)
		out = append(out, v)
		rc = C.efi_get_next_variable_name(&guid, &name, &errno)
	}
	if rc < 0 {
		return nil, syscall.Errno(errno)
	}
	return out, nil
}

func DevicePathToString(dp unsafe.Pointer, dpSz int) (string, error) {
	sz := C.efidp_format_device_path(nil, 0, (C.const_efidp)(dp), C.ssize_t(dpSz))
	if sz < 0 {
		return "", fmt.Errorf("efivar: getting device path string length failed")
	}

	buf := C.malloc(C.size_t(sz))
	defer C.free(buf)
	bufStr := (*C.char)(buf)

	if rc := C.efidp_format_device_path(bufStr, C.size_t(sz), (C.const_efidp)(dp), C.ssize_t(dpSz)); rc < 0 {
		return "", fmt.Errorf("efivar: formatting device path as string failed")
	}

	return C.GoStringN(bufStr, C.int(sz-1)), nil
}

func Get(guid uuid.UUID, name string) (*Variable, error) {
	return VariableName{guid, name}.Get()
}
func Exists(guid uuid.UUID, name string) (bool, error) {
	return VariableName{guid, name}.Exists()
}

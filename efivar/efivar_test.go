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

import (
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
)

var (
	bootCurrent = VariableName{
		GUID: GlobalUUID,
		Name: "BootCurrent",
	}

	testVariable = VariableName{
		GUID: uuid.MustParse("74552304-ce9f-4e52-89a0-f6c6fa47deac"),
		Name: "LukegbEFIvarTest",
	}
)

func TestUUIDToEFIRoundtrip(t *testing.T) {
	u := uuid.MustParse("84be9c3e-8a32-42c0-891c-4cd3b072becc")
	got := efiToUUID(uuidToEFI(u))
	if got != u {
		t.Errorf("efiToUUID(uuidToEFI(%q)) = %q; want %q", u, got, u)
	}

}

func TestUUIDToEFIString(t *testing.T) {
	u := uuid.MustParse("84be9c3e-8a32-42c0-891c-4cd3b072becc")
	got, err := efiGuidToStr(uuidToEFI(u))
	if err != nil {
		t.Fatalf("efiGuidToStr: %v", err)
	}
	if want := u.String(); got != want {
		t.Errorf("efiGuidToStr(uuidToEFI(%q)) = %v; want %v", u, got, want)
	}
}

func TestEFIToUUID(t *testing.T) {
	got := efiToUUID(globalUUID)
	want, err := efiGuidToStr(globalUUID)
	if err != nil {
		t.Fatalf("efiGuidToStr: %v", err)
	}
	if got.String() != want {
		t.Errorf("efiToUUID(globalUUID) = %v; want %v", got, want)
	}
}

func TestVariables(t *testing.T) {
	if !Supported() {
		t.Skip("efivar is not supported")
	}
	vs, err := Variables()
	if err != nil {
		t.Errorf("Variables: %v", err)
	}
	// Expect at least a BootCurrent variable.
	var bootCurrent *VariableName
	varNames := make([]string, len(vs))
	for n, v := range vs {
		v := v
		varNames[n] = v.Name
		if v.Name == "BootCurrent" {
			bootCurrent = &v
		}
	}
	sort.Strings(varNames)
	if bootCurrent == nil {
		t.Fatalf("Expected 'BootCurrent' variable, got: %v", varNames)
	}
	if want := GlobalUUID; bootCurrent.GUID != want {
		t.Errorf("BootCurrent should have GUID %q; got %q", want, bootCurrent.GUID)
	}
}

func TestVariableExists(t *testing.T) {
	if !Supported() {
		t.Skip("efivar is not supported")
	}
	ok, err := bootCurrent.Exists()
	if err != nil {
		t.Errorf("bootCurrent.Exists: %v", err)
	}
	if !ok {
		t.Errorf("bootCurrent.Exists() = %v; want true", ok)
	}
}

func TestVariableGet(t *testing.T) {
	if !Supported() {
		t.Skip("efivar is not supported")
	}
	v, err := bootCurrent.Get()
	if err != nil {
		t.Errorf("bootCurrent.Get: %v", err)
	}
	if len(v.Data) != 2 {
		t.Errorf("bootCurrent Data length is %v; want 2", len(v.Data))
	}
}

func TestVariableSetAndDelete(t *testing.T) {
	if !Supported() {
		t.Skip("efivar is not supported")
	}
	if os.Geteuid() != 0 {
		t.Skip("this test only works *sigh* as root")
	}
	testString := fmt.Sprintf("hello world %v", time.Now())
	v := &Variable{
		VariableName: testVariable,
		Data:         []byte(testString),
		Attributes:   NonVolatile | BootserviceAccess | RuntimeAccess,
	}
	if err := v.Set(0644); err != nil {
		t.Fatalf("testVariable Set: %v", err)
	}

	v2, err := testVariable.Get()
	if err != nil {
		t.Fatalf("testVariable Get: %v", err)
	}

	if string(v2.Data) != testString {
		t.Fatalf("v2.Data = %q; want %q", string(v2.Data), testString)
	}

	if err := v2.Delete(); err != nil {
		t.Fatalf("v2.Delete: %v", err)
	}

	ok, err := v.Exists()
	if err != nil {
		t.Fatalf("v.Exists: %v", err)
	}
	if ok {
		t.Fatalf("v.Exists() = %v; want false", ok)
	}
}

// Package plist serializes service settings with Apple's native plist writer.
package plist

/*
#cgo darwin LDFLAGS: -framework Foundation
#include <stdlib.h>
char *ar_plist_xml(const void *json, size_t length, char **error);
*/
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
	"unsafe"
)

// XML transports JSON-compatible Go values to Foundation for plist encoding.
func XML(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode plist values: %w", err)
	}
	return xmlFromJSON(data)
}

func xmlFromJSON(data []byte) ([]byte, error) {
	input := C.CBytes(data)
	defer C.free(input)
	var message *C.char
	out := C.ar_plist_xml(input, C.size_t(len(data)), &message)
	if message != nil {
		defer C.free(unsafe.Pointer(message))
		return nil, fmt.Errorf("serialize native plist: %s", C.GoString(message))
	}
	if out == nil {
		return nil, errors.New("native plist allocation failed")
	}
	defer C.free(unsafe.Pointer(out))
	return []byte(C.GoString(out)), nil
}

package ceph

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gofrs/uuid"
	log "github.com/sirupsen/logrus"
)

var diskLetters = []rune("abcdefghijklmnopqrstuvwxyz")

// CephConIsNil is a global string error msg
const CephConIsNil string = "the ceph connection was nil"

// diskLetterForIndex return diskLetters for index
func diskLetterForIndex(i int) string {

	q := i / len(diskLetters)
	r := i % len(diskLetters)
	letter := diskLetters[r]

	if q == 0 {
		return fmt.Sprintf("%c", letter)
	}

	return fmt.Sprintf("%s%c", diskLetterForIndex(q-1), letter)
}

// WaitSleepInterval time
var WaitSleepInterval = 1 * time.Second

// WaitTimeout time
var WaitTimeout = 5 * time.Minute

// waitForSuccess wait for success and timeout after 5 minutes.
func waitForSuccess(errorMessage string, f func() error) error {
	start := time.Now()
	for {
		err := f()
		if err == nil {
			return nil
		}
		log.Debugf("%s. Re-trying.\n", err)

		time.Sleep(WaitSleepInterval)
		if time.Since(start) > WaitTimeout {
			return fmt.Errorf("%s: %s", errorMessage, err)
		}
	}
}

// return an indented XML
func xmlMarshallIndented(b interface{}) (string, error) {
	buf := new(bytes.Buffer)
	enc := xml.NewEncoder(buf)
	enc.Indent("  ", "    ")
	if err := enc.Encode(b); err != nil {
		return "", fmt.Errorf("could not marshall this:\n%s", spew.Sdump(b))
	}
	return buf.String(), nil
}

// formatBoolYesNo is similar to strconv.FormatBool with yes/no instead of true/false
func formatBoolYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// formatYesNoBool
func formatYesNoBool(x string) bool {
	return strings.ToLower(strings.TrimSpace(x)) == "yes"
}

// getCPUNum
func getCPUNum() int {
	return runtime.NumCPU()
}

// getUUID
func getUUID() string {
	return uuid.Must(uuid.NewV4()).String()
}

// InSlice check x exist in y(slice)
func InSlice(x interface{}, y interface{}) bool {
	if x == nil || y == nil {
		return false
	}

	switch reflect.TypeOf(y).Kind() {
	case reflect.Slice:
		tmp := reflect.ValueOf(y)
		for i := 0; i < tmp.Len(); i++ {
			if reflect.DeepEqual(x, tmp.Index(i).Interface()) {
				return true
			}
		}
	}
	return false
}

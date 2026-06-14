package api

import (
	"errors"
	"os/exec"
	"testing"
)

func TestDirectoryPickerCanceled(t *testing.T) {
	t.Run("empty stderr exit is treated as canceled", func(t *testing.T) {
		err := exec.Command("sh", "-c", "exit 1").Run()
		if !directoryPickerCanceled(err) {
			t.Fatal("directoryPickerCanceled() = false, want true")
		}
	})

	t.Run("apple script cancel code is treated as canceled", func(t *testing.T) {
		err := exec.Command("sh", "-c", "printf 'execution error: User canceled. (-128)\n' >&2; exit 1").Run()
		if !directoryPickerCanceled(err) {
			t.Fatal("directoryPickerCanceled() = false, want true for AppleScript cancel")
		}
	})

	t.Run("other stderr is not treated as canceled", func(t *testing.T) {
		err := exec.Command("sh", "-c", "printf 'permission denied\n' >&2; exit 1").Run()
		if directoryPickerCanceled(err) {
			t.Fatal("directoryPickerCanceled() = true, want false")
		}
	})

	t.Run("non exit errors are not treated as canceled", func(t *testing.T) {
		if directoryPickerCanceled(errors.New("boom")) {
			t.Fatal("directoryPickerCanceled() = true, want false")
		}
	})
}

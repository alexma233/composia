package controller

import "testing"

func TestTaskParamsReturnsJSONDecodeError(t *testing.T) {
	t.Parallel()

	if _, err := taskParams("{"); err == nil {
		t.Fatal("expected invalid task params JSON to fail")
	}
}

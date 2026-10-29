package proc

import (
	"context"
	"errors"
	"github.com/tyzrex/kport/types"
	"reflect"
	"testing"
)

func TestDiscoveryCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := FindAllProcesses(types.FindOptions{Context: ctx}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestDescendantsLeafFirstAndCycleSafe(t *testing.T) {
	got := descendants(10, map[int][]int{10: {11, 12}, 11: {13}, 13: {10}})
	if !reflect.DeepEqual(got, []int{13, 11, 12}) {
		t.Fatal(got)
	}
}

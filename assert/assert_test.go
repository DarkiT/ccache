package assert

import (
	"testing"
	"time"
)

func TestEqual(t *testing.T) {
	Equal(t, 1, 1)
	Equal(t, "hello", "hello")
}

func TestList(t *testing.T) {
	List(t, []int{1, 2, 3}, []int{1, 2, 3})
	List(t, []string{"a", "b"}, []string{"a", "b"})
}

func TestDoesNotContain(t *testing.T) {
	DoesNotContain(t, []int{1, 2, 3}, 4)
	DoesNotContain(t, []string{"a", "b"}, "c")
}

func TestNil(t *testing.T) {
	var ptr *int
	Nil(t, ptr)
	Nil(t, nil)
}

func TestNotNil(t *testing.T) {
	var val int = 10
	NotNil(t, &val)
	NotNil(t, "not nil")
}

func TestTrue(t *testing.T) {
	True(t, true)
}

func TestFalse(t *testing.T) {
	False(t, false)
}

func TestContains(t *testing.T) {
	Contains(t, "hello world", "world")
	Contains(t, "golang", "go")
}

func TestError(t *testing.T) {
	Error(t, nil, nil)
	Error(t, error(nil), error(nil))
}

func TestNowish(t *testing.T) {
	Nowish(t, time.Now().UTC())
}

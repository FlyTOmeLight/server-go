package core

import (
	"fmt"
	"testing"
)

func TestGenerateObjectID(t *testing.T) {
	result := NewID("prefix")
	fmt.Println(result)
}

func TestRandomString(t *testing.T) {
	for i := 0; i < 100; i++ {
		fmt.Println(RandomString(10))
	}
}

func BenchmarkRandomString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = RandomString(10)
	}
}
